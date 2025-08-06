package runner

import "errors"

var (
	ErrVolumeNotFound    = errors.New("volume not found")
	ErrInvalidVolumeName = errors.New("invalid volume name")
	ErrBadExitCode       = errors.New("bad exit code")
)
