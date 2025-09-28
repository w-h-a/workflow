package notifier

import "context"

type NotifierType string

const (
	Local NotifierType = "local"
)

var (
	NotifierTypes = map[string]NotifierType{
		"local": Local,
	}
)

type Notifier interface {
	Notify(ctx context.Context, event Event, opts ...NotifyOption) error
}
