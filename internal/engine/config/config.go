package config

import (
	"maps"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/w-h-a/workflow/internal/engine/clients/broker"
	"github.com/w-h-a/workflow/internal/engine/clients/readwriter"
	"github.com/w-h-a/workflow/internal/engine/clients/runner"
)

var (
	instance *config
	once     sync.Once
)

type config struct {
	env                string
	name               string
	version            string
	queues             map[string]int
	httpAddress        string
	mode               string
	broker             string
	brokerLocation     string
	runner             string
	runnerHost         string
	readwriter         string
	readwriterLocation string
}

func New() {
	once.Do(func() {
		instance = &config{
			env:                "dev",
			name:               "workflow",
			version:            "0.1.0-alpha.0",
			queues:             map[string]int{broker.SCHEDULED: 1, broker.CANCELLED: 1},
			httpAddress:        ":4000",
			mode:               "standalone",
			broker:             "memory",
			brokerLocation:     "",
			runner:             "docker",
			runnerHost:         "unix:///var/run/docker.sock",
			readwriter:         "memory",
			readwriterLocation: "",
		}

		env := os.Getenv("ENV")
		if len(env) > 0 {
			instance.env = env
		}

		name := os.Getenv("NAME")
		if len(name) > 0 {
			instance.name = name
		}

		version := os.Getenv("VERSION")
		if len(version) > 0 {
			instance.version = version
		}

		qs := os.Getenv("QUEUES")
		if len(qs) > 0 {
			for _, q := range strings.Split(qs, ",") {
				q = strings.TrimSpace(q)
				if len(q) == 0 {
					continue
				}
				def := strings.Split(q, ":")
				if len(def) != 2 {
					panic("invalid queue definition")
				}
				name := strings.TrimSpace(def[0])
				if len(name) == 0 {
					panic("queue name cannot be empty")
				}
				concurrency, err := strconv.Atoi(strings.TrimSpace(def[1]))
				if err != nil {
					panic("queue concurrency is not an integer")
				}
				instance.queues[name] = concurrency
			}
		}

		httpAddress := os.Getenv("HTTP_ADDRESS")
		if len(httpAddress) > 0 {
			instance.httpAddress = httpAddress
		}

		mode := os.Getenv("MODE")
		if len(mode) > 0 {
			instance.mode = mode
		}

		b := os.Getenv("BROKER")
		if len(b) > 0 {
			if _, ok := broker.BrokerTypes[b]; ok {
				instance.broker = b
			} else {
				panic("unsupported broker")
			}
		}

		brokerLocation := os.Getenv("BROKER_LOCATION")
		if len(brokerLocation) > 0 {
			instance.brokerLocation = brokerLocation
		}

		r := os.Getenv("RUNNER")
		if len(r) > 0 {
			if _, ok := runner.RuntimeTypes[r]; ok {
				instance.runner = r
			} else {
				panic("unsupported runner")
			}
		}

		runnerHost := os.Getenv("RUNNER_HOST")
		if len(runnerHost) > 0 {
			instance.runnerHost = runnerHost
		}

		rw := os.Getenv("READ_WRITER")
		if len(rw) > 0 {
			if _, ok := readwriter.ReadWriterTypes[rw]; ok {
				instance.readwriter = rw
			} else {
				panic("unsupported readwriter")
			}
		}

		readwriterLocation := os.Getenv("READ_WRITER_LOCATION")
		if len(readwriterLocation) > 0 {
			instance.readwriterLocation = readwriterLocation
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

func Queues() map[string]int {
	if instance == nil {
		panic("cfg is nil")
	}

	queues := make(map[string]int, len(instance.queues))

	maps.Copy(queues, instance.queues)

	return queues
}

func HttpAddress() string {
	if instance == nil {
		panic("cfg is nil")
	}

	return instance.httpAddress
}

func Mode() string {
	if instance == nil {
		panic("cfg is nil")
	}

	return instance.mode
}

func Broker() string {
	if instance == nil {
		panic("cfg is nil")
	}

	return instance.broker
}

func BrokerLocation() string {
	if instance == nil {
		panic("cfg is nil")
	}

	return instance.brokerLocation
}

func Runner() string {
	if instance == nil {
		panic("cfg is nil")
	}

	return instance.runner
}

func RunnerHost() string {
	if instance == nil {
		panic("cfg is nil")
	}

	return instance.runnerHost
}

func ReadWriter() string {
	if instance == nil {
		panic("cfg is nil")
	}

	return instance.readwriter
}

func ReadWriterLocation() string {
	if instance == nil {
		panic("cfg is nil")
	}

	return instance.readwriterLocation
}
