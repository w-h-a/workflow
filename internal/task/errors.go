package task

import "errors"

var (
	ErrTaskNotFound                  = errors.New("task not found")
	ErrAttemptsSpecified             = errors.New("may not specify retry attempts")
	ErrExcessiveLimit                = errors.New("may not specify retry limit > 10")
	ErrInvalidInitialDelayDuration   = errors.New("invalid initial delay duration")
	ErrExcessiveInitialDelayDuration = errors.New("may not specify retry initial delay duration > 5 mins")
)
