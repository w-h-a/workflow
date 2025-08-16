package broker

import "errors"

var (
	ErrCreatingChannel = errors.New("failed to create channel")
	ErrCreatingQueue   = errors.New("failed to create queue")
	ErrSubscribing     = errors.New("failed to subscribe")
	ErrPublishing      = errors.New("failed to publish")
)
