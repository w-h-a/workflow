package notifier

type AlertType string

const (
	Success AlertType = "SUCCESS"
	Failure AlertType = "FAILURE"
)

type Event struct {
	Source  string
	Type    AlertType
	Title   string
	Payload map[string]any
}
