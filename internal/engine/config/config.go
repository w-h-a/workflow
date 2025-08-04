package config

import "sync"

var (
	instance *config
	once     sync.Once
)

type config struct {
	env         string
	name        string
	version     string
	httpAddress string
}

func New() {
	once.Do(func() {
		instance = &config{
			env:         "dev",
			name:        "workflow",
			version:     "0.1.0-alpha.0",
			httpAddress: ":4000",
		}
	})
}

func Env() string {
	if instance == nil {
		panic("cfg is nil")
	}

	return instance.env
}

func Name() string {
	if instance == nil {
		panic("cfg is nil")
	}

	return instance.name
}

func Version() string {
	if instance == nil {
		panic("cfg is nil")
	}

	return instance.version
}

func HttpAddress() string {
	if instance == nil {
		panic("cfg is nil")
	}

	return instance.httpAddress
}
