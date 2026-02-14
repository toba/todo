package clickup

import (
	"sync"
	"time"

	"github.com/toba/todo/internal/core"
	"github.com/toba/todo/internal/issue"
)

// SyncStateProvider abstracts sync state storage for the syncer.
type SyncStateProvider interface {
	GetTaskID(issueID string) *string
	GetSyncedAt(issueID string) *time.Time
	SetTaskID(issueID, taskID string)
	SetSyncedAt(issueID string, t time.Time)
	Clear(issueID string)
	Flush() error
}

// extensionCache holds cached sync state for a single issue.
type extensionCache struct {
	taskID   string
	syncedAt *time.Time
}

// pendingOp represents a pending write operation.
type pendingOp struct {
	issueID string
	isSet   bool // true = set, false = remove
}

// ExtensionSyncProvider implements SyncStateProvider using issue extension metadata.
// Instead of batch GraphQL mutations via subprocess, it writes directly via core.SaveExtensionOnly().
type ExtensionSyncProvider struct {
	store *core.Core
	mu    sync.RWMutex
	cache map[string]*extensionCache
	ops   []pendingOp
}

// NewExtensionSyncProvider creates a provider pre-populated from an issue list.
func NewExtensionSyncProvider(store *core.Core, issues []*issue.Issue) *ExtensionSyncProvider {
	p := &ExtensionSyncProvider{
		store: store,
		cache: make(map[string]*extensionCache, len(issues)),
	}

	for _, b := range issues {
		taskID := GetExtensionString(b, ExtKeyTaskID)
		syncedAt := GetExtensionTime(b, ExtKeySyncedAt)

		if taskID != "" || syncedAt != nil {
			p.cache[b.ID] = &extensionCache{
				taskID:   taskID,
				syncedAt: syncedAt,
			}
		}
	}

	return p
}

func (p *ExtensionSyncProvider) GetTaskID(issueID string) *string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	c, ok := p.cache[issueID]
	if !ok || c.taskID == "" {
		return nil
	}
	return &c.taskID
}

func (p *ExtensionSyncProvider) GetSyncedAt(issueID string) *time.Time {
	p.mu.RLock()
	defer p.mu.RUnlock()

	c, ok := p.cache[issueID]
	if !ok {
		return nil
	}
	return c.syncedAt
}

func (p *ExtensionSyncProvider) SetTaskID(issueID, taskID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cache[issueID] == nil {
		p.cache[issueID] = &extensionCache{}
	}
	p.cache[issueID].taskID = taskID
	p.ops = append(p.ops, pendingOp{issueID: issueID, isSet: true})
}

func (p *ExtensionSyncProvider) SetSyncedAt(issueID string, t time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()

	utc := t.UTC()
	if p.cache[issueID] == nil {
		p.cache[issueID] = &extensionCache{}
	}
	p.cache[issueID].syncedAt = &utc
	p.ops = append(p.ops, pendingOp{issueID: issueID, isSet: true})
}

func (p *ExtensionSyncProvider) Clear(issueID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.cache, issueID)
	p.ops = append(p.ops, pendingOp{issueID: issueID, isSet: false})
}

// Flush writes all pending operations directly via core.SaveExtensionOnly().
func (p *ExtensionSyncProvider) Flush() error {
	p.mu.Lock()
	ops := p.ops
	p.ops = nil
	p.mu.Unlock()

	if len(ops) == 0 {
		return nil
	}

	// Deduplicate: keep only the last operation per issue ID
	seen := make(map[string]int, len(ops))
	for i, op := range ops {
		seen[op.issueID] = i
	}

	for issueID, idx := range seen {
		op := ops[idx]

		b, err := p.store.Get(issueID)
		if err != nil {
			continue // Issue may have been deleted
		}

		if op.isSet {
			// Build extension data from cache
			p.mu.RLock()
			c := p.cache[issueID]
			p.mu.RUnlock()

			if c == nil {
				continue
			}

			data := map[string]any{
				ExtKeyTaskID: c.taskID,
			}
			if c.syncedAt != nil {
				data[ExtKeySyncedAt] = c.syncedAt.Format(time.RFC3339)
			}

			b.SetExtension(ExtensionName, data)
		} else {
			b.RemoveExtension(ExtensionName)
		}

		if err := p.store.SaveExtensionOnly(b, nil); err != nil {
			return err
		}
	}

	return nil
}

// GetExtensionString returns a string value from an issue's clickup extension data.
func GetExtensionString(b *issue.Issue, key string) string {
	if b.Extensions == nil {
		return ""
	}
	extData, ok := b.Extensions[ExtensionName]
	if !ok {
		return ""
	}
	val, ok := extData[key]
	if !ok {
		return ""
	}
	s, _ := val.(string)
	return s
}

// GetExtensionTime returns a time value from an issue's clickup extension data.
// Expects the value to be an RFC3339 string. Returns nil if not found or unparseable.
func GetExtensionTime(b *issue.Issue, key string) *time.Time {
	s := GetExtensionString(b, key)
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}
