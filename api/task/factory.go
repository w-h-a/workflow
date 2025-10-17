package task

import (
	"encoding/json"
	"strings"
	"time"
)

func Factory(bs []byte) (*Task, error) {
	var t *Task

	if err := json.Unmarshal(bs, &t); err != nil {
		return nil, err
	}

	if len(strings.TrimSpace(t.Image)) == 0 {
		return nil, ErrMissingImage
	}

	// TODO: validate queue field

	for _, m := range t.Mounts {
		if m == nil || len(strings.TrimSpace(m.Target)) == 0 {
			return nil, ErrMountMissingTarget
		}
	}

	if t.Retry != nil {
		if t.Retry.Attempts != 0 {
			return nil, ErrAttemptsSpecified
		}

		if t.Retry.Limit > 10 {
			return nil, ErrExcessiveLimit
		}

		if t.Retry.Limit < 0 {
			t.Retry.Limit = 0
		}
	}

	if len(t.Timeout) > 0 {
		timeout, err := time.ParseDuration(t.Timeout)
		if err != nil {
			return nil, ErrInvalidTimeout
		}

		if timeout <= 0 {
			return nil, ErrInvalidTimeout
		}
	}

	return t, nil
}
