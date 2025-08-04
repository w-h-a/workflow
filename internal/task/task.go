package task

import "encoding/json"

func Factory(bs []byte) (*Task, error) {
	var t *Task

	if err := json.Unmarshal(bs, &t); err != nil {
		return nil, err
	}

	// TODO: validate that we have what is required

	return t, nil
}
