package task

import "errors"

var (
	ErrTaskNotFound       = errors.New("task not found")
	ErrMissingImage       = errors.New("task is missing image")
	ErrMountMissingTarget = errors.New("mount missing target")
	ErrAttemptsSpecified  = errors.New("may not specify retry attempts")
	ErrExcessiveLimit     = errors.New("may not specify retry limit > 10")
	ErrInvalidTimeout     = errors.New("invalid timeout")
	ErrTaskNotCancellable = errors.New("task is not cancellable")
	ErrTaskNotRestartable = errors.New("task not restartable")
)
