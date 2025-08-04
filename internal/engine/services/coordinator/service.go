package coordinator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/w-h-a/workflow/internal/engine/clients/broker"
	"github.com/w-h-a/workflow/internal/task"
)

type Service struct {
	name   string
	topics []string
	broker broker.Broker
}

func (s *Service) Name() string {
	return s.name
}

func (s *Service) ScheduleTask(ctx context.Context, t *task.Task) (*task.Task, error) {
	// just a naive implementation
	now := time.Now()

	if len(t.ID) == 0 {
		t.ID = strings.ReplaceAll(uuid.NewString(), "-", "")
		t.State = task.Scheduled
		t.ScheduledAt = &now
	}

	bs, _ := json.Marshal(t)

	var topic string

	if len(s.topics) > 0 {
		topic = s.topics[0]
	}

	if len(topic) == 0 {
		return nil, fmt.Errorf("no topic found")
	}

	opts := []broker.PublishOption{
		broker.PublishWithTopic(topic),
	}

	if err := s.broker.Publish(ctx, bs, opts...); err != nil {
		return nil, err
	}

	return t, nil
}

func New(topics []string, b broker.Broker) *Service {
	name := fmt.Sprintf("coordinator-%s", strings.ReplaceAll(uuid.NewString(), "-", ""))

	return &Service{
		name:   name,
		topics: topics,
		broker: b,
	}
}
