package log

import "encoding/json"

func Factory(bs []byte) (*Entry, error) {
	var e *Entry

	if err := json.Unmarshal(bs, &e); err != nil {
		return nil, err
	}

	// TODO: more validation

	return e, nil
}
