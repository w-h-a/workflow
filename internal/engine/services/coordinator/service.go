package coordinator

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/w-h-a/workflow/internal/engine/clients/broker"
)

type Service struct {
	name   string
	broker broker.Broker
}

func (s *Service) Name() string {
	return s.name
}

func New(b broker.Broker) *Service {
	name := fmt.Sprintf("worker-%s", strings.ReplaceAll(uuid.NewString(), "-", ""))

	return &Service{
		name:   name,
		broker: b,
	}
}
