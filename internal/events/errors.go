// Package events provides event and template management.
package events

import "errors"

// Event errors.
var (
	ErrEventNotFound     = errors.New("event not found")
	ErrTemplateNotFound  = errors.New("template not found")
	ErrInvalidStatus     = errors.New("invalid status for event type")
	ErrInvalidSeverity   = errors.New("severity is required for incidents")
	ErrServiceNotInEvent = errors.New("service is not associated with this event")
)
