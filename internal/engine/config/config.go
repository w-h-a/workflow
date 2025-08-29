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
	mode               string
	coordinatorHttp    string
	workerHttp         string
	workerQueues       map[string]int
	broker             string
	brokerLocation     string
	runner             string
	runnerHost         string
	runnerRegistryUser string
	runnerRegistryPass string
	readwriter         string
	readwriterLocation string
}

func New() {
	once.Do(func() {
		instance = &config{
			env:                "dev",
			name:               "workflow",
			version:            "0.1.0-alpha.0",
			mode:               "standalone",
			coordinatorHttp:    ":4000",
			workerHttp:         ":4001",
			workerQueues:       map[string]int{broker.SCHEDULED: 1, broker.CANCELLED: 1},
			broker:             "memory",
			brokerLocation:     "",
			runner:             "docker",
			runnerHost:         "unix:///var/run/docker.sock",
			runnerRegistryUser: "",
			runnerRegistryPass: "",
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

		mode := os.Getenv("MODE")
		if len(mode) > 0 {
			instance.mode = mode
		}

		httpAddress := os.Getenv("HTTP_ADDRESS")
		if len(httpAddress) > 0 {
			if instance.mode == "coordinator" {
				instance.coordinatorHttp = httpAddress
			}
			if instance.mode == "worker" {
				instance.workerHttp = httpAddress
			}
		}

		qs := os.Getenv("QUEUES")
		if len(qs) > 0 && instance.mode == "worker" {
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
				instance.workerQueues[name] = concurrency
			}
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

		runnerRegistryUser := os.Getenv("RUNNER_REGISTRY_USER")
		if len(runnerRegistryUser) > 0 {
			instance.runnerRegistryUser = runnerRegistryUser
		}

		runnerRegistryPass := os.Getenv("RUNNER_REGISTRY_PASS")
		if len(runnerRegistryPass) > 0 {
			instance.runnerRegistryPass = runnerRegistryPass
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

func Mode() string {
	if instance == nil {
		panic("cfg is nil")
	}

	return instance.mode
}

func CoordinatorHttp() string {
	if instance == nil {
		panic("cfg is nil")
	}

	return instance.coordinatorHttp
}

func WorkerHttp() string {
	if instance == nil {
		panic("cfg is nil")
	}

	return instance.workerHttp
}

func WorkerQueues() map[string]int {
	if instance == nil {
		panic("cfg is nil")
	}

	queues := make(map[string]int, len(instance.workerQueues))

	maps.Copy(queues, instance.workerQueues)

	return queues
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

func RunnerRegistryUser() string {
	if instance == nil {
		panic("cfg is nil")
	}

	return instance.runnerRegistryUser
}

func RunnerRegistryPass() string {
	if instance == nil {
		panic("cfg is nil")
	}

	return instance.runnerRegistryPass
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
