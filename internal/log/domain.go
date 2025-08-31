package log

type Entry struct {
	TaskID string `json:"taskId"`
	Log    string `json:"log"`
}
