package service

import (
	"context"

	"mqtter/internal/domain"
	"mqtter/internal/ports"
)

type MessageService struct {
	messages ports.MessageRepository
	clock    ports.Clock
}

func NewMessageService(messages ports.MessageRepository, clock ports.Clock) *MessageService {
	if clock == nil {
		clock = SystemClock{}
	}
	return &MessageService{messages: messages, clock: clock}
}

func (s *MessageService) QueryMessages(ctx context.Context, f domain.MessageFilter) (domain.Page[domain.MessageDTO], error) {
	f = domain.ApplyDefaultMessageRange(s.clock.Now(), f)
	return s.messages.QueryMessages(ctx, f)
}
