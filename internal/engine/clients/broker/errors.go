package broker

import "errors"

var (
	ErrUnknownTopic = errors.New("unknown topic")
	ErrUnknownGroup = errors.New("unknown group")
)
