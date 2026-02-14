package graph

import (
	"fmt"

	"github.com/toba/todo/internal/issue"
	"github.com/toba/todo/internal/core"
)

//go:generate go tool gqlgen generate

// Resolver is the root resolver for the GraphQL schema.
// It holds a reference to core.Core for data access.
type Resolver struct {
	Core *core.Core
}

// validateETag checks if the provided ifMatch etag matches the issue's current etag.
// Returns an error if validation fails or if require_if_match is enabled and no etag provided.
func (r *Resolver) validateETag(b *issue.Issue, ifMatch *string) error {
	cfg := r.Core.Config()
	requireIfMatch := cfg != nil && cfg.Issues.RequireIfMatch

	// If require_if_match is enabled and no etag provided, reject
	if requireIfMatch && (ifMatch == nil || *ifMatch == "") {
		return &core.ETagRequiredError{}
	}

	// If ifMatch provided, validate it
	if ifMatch != nil && *ifMatch != "" {
		currentETag := b.ETag()
		if currentETag != *ifMatch {
			return &core.ETagMismatchError{Provided: *ifMatch, Current: currentETag}
		}
	}

	return nil
}

// validateAndSetParent validates and sets the parent relationship.
func (r *Resolver) validateAndSetParent(b *issue.Issue, parentID string) error {
	if parentID == "" {
		b.Parent = ""
		return nil
	}

	// Normalise short ID to full ID
	normalizedParent, _ := r.Core.NormalizeID(parentID)

	// Validate parent type hierarchy
	if err := r.Core.ValidateParent(b, normalizedParent); err != nil {
		return err
	}

	// Check for cycles
	if cycle := r.Core.DetectCycle(b.ID, issue.LinkTypeParent, normalizedParent); cycle != nil {
		return fmt.Errorf("setting parent would create cycle: %v", cycle)
	}

	b.Parent = normalizedParent
	return nil
}

// validateAndAddBlocking validates and adds blocking relationships.
func (r *Resolver) validateAndAddBlocking(b *issue.Issue, targetIDs []string) error {
	for _, targetID := range targetIDs {
		// Normalise short ID to full ID
		normalizedTargetID, _ := r.Core.NormalizeID(targetID)

		// Validate: cannot block itself
		if normalizedTargetID == b.ID {
			return fmt.Errorf("issue cannot block itself")
		}

		// Validate: target must exist
		if _, err := r.Core.Get(normalizedTargetID); err != nil {
			return fmt.Errorf("blocking target issue not found: %s", targetID)
		}

		// Check for cycles in both directions
		if cycle := r.Core.DetectCycle(b.ID, issue.LinkTypeBlocking, normalizedTargetID); cycle != nil {
			return fmt.Errorf("adding blocking relationship would create cycle: %v", cycle)
		}
		if cycle := r.Core.DetectCycle(normalizedTargetID, issue.LinkTypeBlockedBy, b.ID); cycle != nil {
			return fmt.Errorf("adding blocking relationship would create cycle: %v", cycle)
		}

		b.AddBlocking(normalizedTargetID)
	}
	return nil
}

// removeBlockingRelationships removes blocking relationships.
func (r *Resolver) removeBlockingRelationships(b *issue.Issue, targetIDs []string) {
	for _, targetID := range targetIDs {
		normalizedTargetID, _ := r.Core.NormalizeID(targetID)
		b.RemoveBlocking(normalizedTargetID)
	}
}

// validateAndAddBlockedBy validates and adds blocked-by relationships.
func (r *Resolver) validateAndAddBlockedBy(b *issue.Issue, targetIDs []string) error {
	for _, targetID := range targetIDs {
		// Normalise short ID to full ID
		normalizedTargetID, _ := r.Core.NormalizeID(targetID)

		// Validate: cannot be blocked by itself
		if normalizedTargetID == b.ID {
			return fmt.Errorf("issue cannot be blocked by itself")
		}

		// Validate: blocker must exist
		if _, err := r.Core.Get(normalizedTargetID); err != nil {
			return fmt.Errorf("blocker issue not found: %s", targetID)
		}

		// Check for cycles in both directions
		if cycle := r.Core.DetectCycle(normalizedTargetID, issue.LinkTypeBlocking, b.ID); cycle != nil {
			return fmt.Errorf("adding blocked-by relationship would create cycle: %v", cycle)
		}
		if cycle := r.Core.DetectCycle(b.ID, issue.LinkTypeBlockedBy, normalizedTargetID); cycle != nil {
			return fmt.Errorf("adding blocked-by relationship would create cycle: %v", cycle)
		}

		b.AddBlockedBy(normalizedTargetID)
	}
	return nil
}

// removeBlockedByRelationships removes blocked-by relationships.
func (r *Resolver) removeBlockedByRelationships(b *issue.Issue, targetIDs []string) {
	for _, targetID := range targetIDs {
		normalizedTargetID, _ := r.Core.NormalizeID(targetID)
		b.RemoveBlockedBy(normalizedTargetID)
	}
}
