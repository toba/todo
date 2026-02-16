package issue

import (
	"sort"
	"strings"
	"time"

	"github.com/toba/todo/internal/config"
)

// CompareByStatusPriorityAndType returns true if a should sort before b,
// using status order, then priority, then type, then title.
// Unrecognized statuses, priorities, and types are sorted last within their category.
// Issues without priority are treated as "normal" priority for sorting purposes.
func CompareByStatusPriorityAndType(a, b *Issue, statusNames, priorityNames, typeNames []string) bool {
	statusOrder := make(map[string]int)
	for i, s := range statusNames {
		statusOrder[s] = i
	}
	priorityOrder := make(map[string]int)
	for i, p := range priorityNames {
		priorityOrder[p] = i
	}
	typeOrder := make(map[string]int)
	for i, t := range typeNames {
		typeOrder[t] = i
	}

	// Find the index of "normal" priority for issues without priority set
	normalPriorityOrder := len(priorityNames) // default to last if "normal" not found
	for i, p := range priorityNames {
		if p == config.PriorityNormal {
			normalPriorityOrder = i
			break
		}
	}

	// Helper to get order with unrecognized values sorted last
	getStatusOrder := func(status string) int {
		if order, ok := statusOrder[status]; ok {
			return order
		}
		return len(statusNames)
	}
	getPriorityOrder := func(priority string) int {
		if priority == "" {
			return normalPriorityOrder
		}
		if order, ok := priorityOrder[priority]; ok {
			return order
		}
		return len(priorityNames)
	}
	getTypeOrder := func(typ string) int {
		if order, ok := typeOrder[typ]; ok {
			return order
		}
		return len(typeNames)
	}

	// Primary: status order
	oi, oj := getStatusOrder(a.Status), getStatusOrder(b.Status)
	if oi != oj {
		return oi < oj
	}
	// Secondary: priority order
	pi, pj := getPriorityOrder(a.Priority), getPriorityOrder(b.Priority)
	if pi != pj {
		return pi < pj
	}
	// Tertiary: type order
	ti, tj := getTypeOrder(a.Type), getTypeOrder(b.Type)
	if ti != tj {
		return ti < tj
	}
	// Quaternary: title (case-insensitive) for stable, user-friendly ordering
	return strings.ToLower(a.Title) < strings.ToLower(b.Title)
}

// SortByStatusPriorityAndType sorts issues by status order, then priority, then type, then title.
// This is the default sorting used by both CLI and TUI.
func SortByStatusPriorityAndType(issues []*Issue, statusNames, priorityNames, typeNames []string) {
	sort.Slice(issues, func(i, j int) bool {
		return CompareByStatusPriorityAndType(issues[i], issues[j], statusNames, priorityNames, typeNames)
	})
}

// ComputeEffectiveDates builds a map of issue ID to effective date for sorting.
// The effective date for an issue is the maximum of its own date and all descendants' dates.
// field must be "created_at" or "updated_at".
func ComputeEffectiveDates(allIssues []*Issue, field string) map[string]time.Time {
	// Build parent→children index
	children := map[string][]string{}
	issueByID := map[string]*Issue{}
	for _, b := range allIssues {
		issueByID[b.ID] = b
		if b.Parent != "" {
			children[b.Parent] = append(children[b.Parent], b.ID)
		}
	}

	// Recursive: effective date = max(own date, max of children's effective dates)
	cache := map[string]time.Time{}
	var compute func(id string) time.Time
	compute = func(id string) time.Time {
		if t, ok := cache[id]; ok {
			return t
		}
		b := issueByID[id]
		var best time.Time
		if b != nil {
			switch field {
			case FieldCreatedAt:
				if b.CreatedAt != nil {
					best = *b.CreatedAt
				}
			case FieldUpdatedAt:
				if b.UpdatedAt != nil {
					best = *b.UpdatedAt
				}
			}
		}
		for _, childID := range children[id] {
			if ct := compute(childID); ct.After(best) {
				best = ct
			}
		}
		cache[id] = best
		return best
	}

	for _, b := range allIssues {
		compute(b.ID)
	}
	return cache
}

// SortByEffectiveDate sorts issues by effective date, newest first.
// Issues without dates sort last. Ties are broken by title for stability.
func SortByEffectiveDate(issues []*Issue, effectiveDates map[string]time.Time) {
	sort.Slice(issues, func(i, j int) bool {
		di := effectiveDates[issues[i].ID]
		dj := effectiveDates[issues[j].ID]
		if di.IsZero() && dj.IsZero() {
			return strings.ToLower(issues[i].Title) < strings.ToLower(issues[j].Title)
		}
		if di.IsZero() {
			return false // no date sorts last
		}
		if dj.IsZero() {
			return true
		}
		if !di.Equal(dj) {
			return di.After(dj) // newest first
		}
		return strings.ToLower(issues[i].Title) < strings.ToLower(issues[j].Title)
	})
}

// SortByDueDate sorts issues by due date, soonest first.
// Issues without a due date sort last. Ties are broken by title for stability.
func SortByDueDate(issues []*Issue) {
	sort.Slice(issues, func(i, j int) bool {
		di := issues[i].Due
		dj := issues[j].Due
		if di == nil && dj == nil {
			return strings.ToLower(issues[i].Title) < strings.ToLower(issues[j].Title)
		}
		if di == nil {
			return false // no due date sorts last
		}
		if dj == nil {
			return true
		}
		if !di.Time.Equal(dj.Time) {
			return di.Time.Before(dj.Time) // soonest first
		}
		return strings.ToLower(issues[i].Title) < strings.ToLower(issues[j].Title)
	})
}
