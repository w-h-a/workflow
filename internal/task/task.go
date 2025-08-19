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

		if len(t.Retry.InitialDelay) == 0 {
			t.Retry.InitialDelay = DEFAULT_RETRY_INITIAL_DELAY
		}

		delay, err := time.ParseDuration(t.Retry.InitialDelay)
		if err != nil {
			return nil, ErrInvalidInitialDelayDuration
		}

		if delay > (time.Minute * 5) {
			return nil, ErrExcessiveInitialDelayDuration
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
