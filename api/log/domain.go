package log

const (
	Queue string = "LOGS"
)

type Entry struct {
	TaskID string `json:"taskId"`
	Log    string `json:"log"`
}
