package runner

import "errors"

var (
	ErrPullingImage      = errors.New("failed to pull image")
	ErrVolumeNotFound    = errors.New("volume not found")
	ErrInvalidVolumeName = errors.New("invalid volume name")
	ErrBadExitCode       = errors.New("bad exit code")
	ErrRunnerClosing     = errors.New("runner is closing")
)
