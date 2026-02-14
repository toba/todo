package issue

import (
	"sort"
	"strings"
	"time"
)

// CompareByStatusPriorityAndType returns true if a should sort before b,
// using status order, then priority, then type, then title.
// Unrecognized statuses, priorities, and types are sorted last within their category.
// Beans without priority are treated as "normal" priority for sorting purposes.
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

	// Find the index of "normal" priority for beans without priority set
	normalPriorityOrder := len(priorityNames) // default to last if "normal" not found
	for i, p := range priorityNames {
		if p == "normal" {
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

// SortByStatusPriorityAndType sorts beans by status order, then priority, then type, then title.
// This is the default sorting used by both CLI and TUI.
func SortByStatusPriorityAndType(beans []*Issue, statusNames, priorityNames, typeNames []string) {
	sort.Slice(beans, func(i, j int) bool {
		return CompareByStatusPriorityAndType(beans[i], beans[j], statusNames, priorityNames, typeNames)
	})
}

// ComputeEffectiveDates builds a map of bean ID to effective date for sorting.
// The effective date for a bean is the maximum of its own date and all descendants' dates.
// field must be "created_at" or "updated_at".
func ComputeEffectiveDates(allBeans []*Issue, field string) map[string]time.Time {
	// Build parent→children index
	children := map[string][]string{}
	beanByID := map[string]*Issue{}
	for _, b := range allBeans {
		beanByID[b.ID] = b
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
		b := beanByID[id]
		var best time.Time
		if b != nil {
			switch field {
			case "created_at":
				if b.CreatedAt != nil {
					best = *b.CreatedAt
				}
			case "updated_at":
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

	for _, b := range allBeans {
		compute(b.ID)
	}
	return cache
}

// SortByEffectiveDate sorts beans by effective date, newest first.
// Beans without dates sort last. Ties are broken by title for stability.
func SortByEffectiveDate(beans []*Issue, effectiveDates map[string]time.Time) {
	sort.Slice(beans, func(i, j int) bool {
		di := effectiveDates[beans[i].ID]
		dj := effectiveDates[beans[j].ID]
		if di.IsZero() && dj.IsZero() {
			return strings.ToLower(beans[i].Title) < strings.ToLower(beans[j].Title)
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
		return strings.ToLower(beans[i].Title) < strings.ToLower(beans[j].Title)
	})
}

// SortByCreatedAt sorts beans by effective created_at date, newest first.
func SortByCreatedAt(beans []*Issue, effectiveDates map[string]time.Time) {
	SortByEffectiveDate(beans, effectiveDates)
}

// SortByUpdatedAt sorts beans by effective updated_at date, newest first.
func SortByUpdatedAt(beans []*Issue, effectiveDates map[string]time.Time) {
	SortByEffectiveDate(beans, effectiveDates)
}

// SortByDueDate sorts beans by due date, soonest first.
// Beans without a due date sort last. Ties are broken by title for stability.
func SortByDueDate(beans []*Issue) {
	sort.Slice(beans, func(i, j int) bool {
		di := beans[i].Due
		dj := beans[j].Due
		if di == nil && dj == nil {
			return strings.ToLower(beans[i].Title) < strings.ToLower(beans[j].Title)
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
		return strings.ToLower(beans[i].Title) < strings.ToLower(beans[j].Title)
	})
}
