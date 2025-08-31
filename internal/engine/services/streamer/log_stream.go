package streamer

type LogStream struct {
	Logs chan string
	Stop chan struct{}
}
