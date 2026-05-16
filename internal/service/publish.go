package service

import (
	"context"

	"mqtter/internal/domain"
	"mqtter/internal/ports"
)

type PublishService struct {
	commands     ports.PublishCommandRepository
	topics       ports.TopicRepository
	publisher    ports.MQTTPublisher
	clock        ports.Clock
	ids          ports.IDGenerator
	maxPayload   int
	inlineClient string
}

func NewPublishService(commands ports.PublishCommandRepository, topics ports.TopicRepository, publisher ports.MQTTPublisher, clock ports.Clock, ids ports.IDGenerator, maxPayload int) *PublishService {
	if clock == nil {
		clock = SystemClock{}
	}
	if ids == nil {
		ids = RandomIDGenerator{}
	}
	return &PublishService{
		commands:     commands,
		topics:       topics,
		publisher:    publisher,
		clock:        clock,
		ids:          ids,
		maxPayload:   maxPayload,
		inlineClient: "inline",
	}
}

func (s *PublishService) Publish(ctx context.Context, cmd domain.PublishCommand) (domain.PublishResult, error) {
	if err := domain.ValidatePublishTopic(cmd.Topic); err != nil {
		return domain.PublishResult{}, err
	}
	if err := domain.ValidateQoS(cmd.QoS); err != nil {
		return domain.PublishResult{}, err
	}
	if _, err := domain.ValidateTextPayload(cmd.PayloadText, s.maxPayload); err != nil {
		return domain.PublishResult{}, err
	}

	id := s.ids.NewID()
	created, err := s.commands.CreatePublishCommand(ctx, id, cmd, s.clock.Now())
	if err != nil {
		return domain.PublishResult{}, err
	}

	err = s.publisher.Publish(ctx, cmd.Topic, cmd.PayloadText, domain.PublishOptions{QoS: cmd.QoS, Retain: cmd.Retain})
	if err != nil {
		_ = s.commands.MarkPublishCommand(ctx, created.ID, domain.PublishStatusFailed, err.Error(), nil)
		return domain.PublishResult{}, err
	}

	publishedAt := s.clock.Now()
	if s.topics != nil {
		_ = s.topics.RecordAdminPublishTopic(ctx, s.inlineClient, cmd.Topic, cmd.QoS, publishedAt)
	}
	if err := s.commands.MarkPublishCommand(ctx, created.ID, domain.PublishStatusPublished, "", &publishedAt); err != nil {
		return domain.PublishResult{}, err
	}

	return domain.PublishResult{
		CommandID:   created.ID,
		Status:      domain.PublishStatusPublished,
		PublishedAt: publishedAt,
	}, nil
}

func (s *PublishService) ListPublishCommands(ctx context.Context, f domain.CommandFilter) (domain.Page[domain.PublishCommandDTO], error) {
	f.Page, f.PageSize = domain.NormalizePage(f.Page, f.PageSize)
	return s.commands.ListPublishCommands(ctx, f)
}
