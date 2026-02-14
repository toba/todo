package syncutil

import (
	"time"

	"github.com/toba/todo/internal/issue"
)

// GetSyncString retrieves a string value from an issue's sync metadata.
func GetSyncString(b *issue.Issue, syncName, key string) string {
	if b.Sync == nil {
		return ""
	}
	extData, ok := b.Sync[syncName]
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

// GetSyncTime retrieves a time value from an issue's sync metadata.
func GetSyncTime(b *issue.Issue, syncName, key string) *time.Time {
	s := GetSyncString(b, syncName, key)
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}
