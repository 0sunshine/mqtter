package service

import (
	"context"
	"strings"

	"mqtter/internal/domain"
	"mqtter/internal/ports"
)

type DeviceService struct {
	devices ports.DeviceRepository
	topics  ports.TopicRepository
	clock   ports.Clock
}

func NewDeviceService(devices ports.DeviceRepository, topics ports.TopicRepository, clock ports.Clock) *DeviceService {
	if clock == nil {
		clock = SystemClock{}
	}
	return &DeviceService{devices: devices, topics: topics, clock: clock}
}

func (s *DeviceService) ListDevices(ctx context.Context, f domain.DeviceFilter) (domain.Page[domain.DeviceDTO], error) {
	f.Page, f.PageSize = domain.NormalizePage(f.Page, f.PageSize)
	return s.devices.ListDevices(ctx, f)
}

func (s *DeviceService) GetDevice(ctx context.Context, id string) (domain.DeviceDTO, error) {
	if strings.TrimSpace(id) == "" {
		return domain.DeviceDTO{}, domain.InvalidInput("invalid_device_id", "device id must not be empty")
	}
	return s.devices.GetDevice(ctx, id)
}

func (s *DeviceService) ChangeDeviceType(ctx context.Context, cmd domain.ChangeDeviceTypeCommand) (domain.DeviceDTO, error) {
	if strings.TrimSpace(cmd.DeviceID) == "" {
		return domain.DeviceDTO{}, domain.InvalidInput("invalid_device_id", "device id must not be empty")
	}
	if strings.TrimSpace(cmd.Type) == "" {
		return domain.DeviceDTO{}, domain.InvalidInput("invalid_device_type", "device type must not be empty")
	}
	return s.devices.ChangeDeviceType(ctx, cmd, s.clock.Now())
}

func (s *DeviceService) ListDeviceTypes(ctx context.Context) ([]domain.DeviceTypeDTO, error) {
	return s.devices.ListDeviceTypes(ctx)
}

func (s *DeviceService) ListDeviceTopics(ctx context.Context, deviceID string, f domain.TopicFilter) (domain.Page[domain.ObservedTopicDTO], error) {
	if strings.TrimSpace(deviceID) == "" {
		return domain.Page[domain.ObservedTopicDTO]{}, domain.InvalidInput("invalid_device_id", "device id must not be empty")
	}
	f.Page, f.PageSize = domain.NormalizePage(f.Page, f.PageSize)
	return s.topics.ListDeviceTopics(ctx, deviceID, f)
}

func (s *DeviceService) ListTopics(ctx context.Context, f domain.TopicFilter) (domain.Page[domain.ObservedTopicDTO], error) {
	f.Page, f.PageSize = domain.NormalizePage(f.Page, f.PageSize)
	return s.topics.ListTopics(ctx, f)
}
