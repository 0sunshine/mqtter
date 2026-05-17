package service

import (
	"context"
	"strings"

	"mqtter/internal/domain"
	"mqtter/internal/ports"
)

type quickActionDeviceReader interface {
	GetDevice(ctx context.Context, id string) (domain.DeviceDTO, error)
}

type quickActionPublisher interface {
	Publish(ctx context.Context, cmd domain.PublishCommand) (domain.PublishResult, error)
}

type QuickActionService struct {
	repo       ports.QuickActionRepository
	devices    quickActionDeviceReader
	publisher  quickActionPublisher
	clock      ports.Clock
	ids        ports.IDGenerator
	maxPayload int
}

func NewQuickActionService(repo ports.QuickActionRepository, devices quickActionDeviceReader, publisher quickActionPublisher, clock ports.Clock, ids ports.IDGenerator, maxPayload int) *QuickActionService {
	if clock == nil {
		clock = SystemClock{}
	}
	if ids == nil {
		ids = RandomIDGenerator{}
	}
	return &QuickActionService{repo: repo, devices: devices, publisher: publisher, clock: clock, ids: ids, maxPayload: maxPayload}
}

func (s *QuickActionService) CreateQuickAction(ctx context.Context, cmd domain.CreateQuickActionCommand) (domain.QuickActionDTO, error) {
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Topic = strings.TrimSpace(cmd.Topic)
	if err := domain.ValidateQuickActionCommand(cmd, s.maxPayload); err != nil {
		return domain.QuickActionDTO{}, err
	}
	device, err := s.devices.GetDevice(ctx, cmd.DeviceID)
	if err != nil {
		return domain.QuickActionDTO{}, err
	}
	if device.Type != domain.InfraredControllerType {
		return domain.QuickActionDTO{}, domain.InvalidInput("unsupported_device_type", "quick actions are only available for smart infrared controllers")
	}
	now := s.clock.Now()
	return s.repo.CreateQuickAction(ctx, domain.QuickActionDTO{
		ID:          s.ids.NewID(),
		DeviceID:    cmd.DeviceID,
		AdminUserID: cmd.AdminUserID,
		Name:        cmd.Name,
		Topic:       cmd.Topic,
		PayloadText: cmd.PayloadText,
		QoS:         cmd.QoS,
		Retain:      cmd.Retain,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
}

func (s *QuickActionService) ListQuickActions(ctx context.Context, f domain.QuickActionFilter) (domain.Page[domain.QuickActionDTO], error) {
	f.Page, f.PageSize = domain.NormalizePage(f.Page, f.PageSize)
	return s.repo.ListQuickActions(ctx, f)
}

func (s *QuickActionService) DeleteQuickAction(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return domain.InvalidInput("invalid_quick_action_id", "quick action id must not be empty")
	}
	return s.repo.DeleteQuickAction(ctx, id)
}

func (s *QuickActionService) ExecuteQuickAction(ctx context.Context, id string, adminUserID string) (domain.QuickActionExecuteResult, error) {
	if strings.TrimSpace(id) == "" {
		return domain.QuickActionExecuteResult{}, domain.InvalidInput("invalid_quick_action_id", "quick action id must not be empty")
	}
	action, err := s.repo.GetQuickAction(ctx, id)
	if err != nil {
		return domain.QuickActionExecuteResult{}, err
	}
	device, err := s.devices.GetDevice(ctx, action.DeviceID)
	if err != nil {
		return domain.QuickActionExecuteResult{}, err
	}
	if device.Type != domain.InfraredControllerType {
		return domain.QuickActionExecuteResult{}, domain.InvalidInput("unsupported_device_type", "quick actions are only available for smart infrared controllers")
	}
	result, err := s.publisher.Publish(ctx, domain.PublishCommand{
		AdminUserID: adminUserID,
		Topic:       action.Topic,
		PayloadText: action.PayloadText,
		QoS:         action.QoS,
		Retain:      action.Retain,
	})
	if err != nil {
		return domain.QuickActionExecuteResult{}, err
	}
	return domain.QuickActionExecuteResult{
		ActionID:    action.ID,
		CommandID:   result.CommandID,
		Status:      result.Status,
		PublishedAt: result.PublishedAt,
	}, nil
}
