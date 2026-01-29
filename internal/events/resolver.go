package events

import "context"

// GroupServiceResolver resolves group IDs to service IDs.
type GroupServiceResolver interface {
	GetGroupServices(ctx context.Context, groupID string) ([]string, error)
}
