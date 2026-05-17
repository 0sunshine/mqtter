package service

import (
	"context"
	"testing"
	"time"

	"mqtter/internal/domain"
)

type fakeQuickActionRepo struct {
	created []domain.QuickActionDTO
	actions map[string]domain.QuickActionDTO
	deleted string
}

func (r *fakeQuickActionRepo) CreateQuickAction(_ context.Context, action domain.QuickActionDTO) (domain.QuickActionDTO, error) {
	r.created = append(r.created, action)
	action.ClientID = "client-1"
	if r.actions == nil {
		r.actions = map[string]domain.QuickActionDTO{}
	}
	r.actions[action.ID] = action
	return action, nil
}

func (r *fakeQuickActionRepo) ListQuickActions(context.Context, domain.QuickActionFilter) (domain.Page[domain.QuickActionDTO], error) {
	return domain.Page[domain.QuickActionDTO]{}, nil
}

func (r *fakeQuickActionRepo) GetQuickAction(_ context.Context, id string) (domain.QuickActionDTO, error) {
	return r.actions[id], nil
}

func (r *fakeQuickActionRepo) DeleteQuickAction(_ context.Context, id string) error {
	r.deleted = id
	return nil
}

type fakeQuickActionDevices struct {
	device domain.DeviceDTO
}

func (d fakeQuickActionDevices) GetDevice(context.Context, string) (domain.DeviceDTO, error) {
	return d.device, nil
}

type fakeQuickActionPublisher struct {
	cmd domain.PublishCommand
}

func (p *fakeQuickActionPublisher) Publish(_ context.Context, cmd domain.PublishCommand) (domain.PublishResult, error) {
	p.cmd = cmd
	return domain.PublishResult{CommandID: "published-1", Status: domain.PublishStatusPublished, PublishedAt: time.Date(2026, 5, 17, 9, 0, 0, 0, time.UTC)}, nil
}

func TestQuickActionServiceCreatesInfraredAction(t *testing.T) {
	repo := &fakeQuickActionRepo{}
	svc := NewQuickActionService(repo, fakeQuickActionDevices{device: domain.DeviceDTO{ID: "dev-1", Type: domain.InfraredControllerType}}, &fakeQuickActionPublisher{}, fixedClock{t: time.Date(2026, 5, 17, 8, 0, 0, 0, time.UTC)}, fixedIDs{}, 1024)

	action, err := svc.CreateQuickAction(context.Background(), domain.CreateQuickActionCommand{
		AdminUserID: "admin-1",
		DeviceID:    "dev-1",
		Name:        " 客厅空调 ",
		Topic:       "devices/a/in",
		PayloadText: `{"action":"emit","data":{"no":3},"type":"infrared"}`,
		QoS:         1,
	})
	if err != nil {
		t.Fatalf("CreateQuickAction returned error: %v", err)
	}
	if action.Name != "客厅空调" || action.ID != "cmd-1" {
		t.Fatalf("unexpected action: %#v", action)
	}
	if len(repo.created) != 1 {
		t.Fatalf("expected action to be persisted, got %d", len(repo.created))
	}
}

func TestQuickActionServiceRejectsUnknownDeviceType(t *testing.T) {
	repo := &fakeQuickActionRepo{}
	svc := NewQuickActionService(repo, fakeQuickActionDevices{device: domain.DeviceDTO{ID: "dev-1", Type: "unknown"}}, &fakeQuickActionPublisher{}, fixedClock{t: time.Now().UTC()}, fixedIDs{}, 1024)

	_, err := svc.CreateQuickAction(context.Background(), domain.CreateQuickActionCommand{
		AdminUserID: "admin-1",
		DeviceID:    "dev-1",
		Name:        "客厅空调",
		Topic:       "devices/a/in",
		PayloadText: `{"action":"emit","data":{"no":3},"type":"infrared"}`,
	})
	if err == nil || domain.ErrorCode(err) != "unsupported_device_type" {
		t.Fatalf("expected unsupported_device_type, got %v", err)
	}
	if len(repo.created) != 0 {
		t.Fatal("unsupported action should not be persisted")
	}
}

func TestQuickActionServiceExecutesWithCurrentAdmin(t *testing.T) {
	publisher := &fakeQuickActionPublisher{}
	repo := &fakeQuickActionRepo{actions: map[string]domain.QuickActionDTO{
		"action-1": {
			ID:          "action-1",
			DeviceID:    "dev-1",
			AdminUserID: "owner-1",
			Name:        "客厅空调",
			Topic:       "devices/a/in",
			PayloadText: `{"action":"emit","data":{"no":3},"type":"infrared"}`,
			QoS:         1,
		},
	}}
	svc := NewQuickActionService(repo, fakeQuickActionDevices{device: domain.DeviceDTO{ID: "dev-1", Type: domain.InfraredControllerType}}, publisher, fixedClock{}, fixedIDs{}, 1024)

	res, err := svc.ExecuteQuickAction(context.Background(), "action-1", "admin-2")
	if err != nil {
		t.Fatalf("ExecuteQuickAction returned error: %v", err)
	}
	if res.CommandID != "published-1" {
		t.Fatalf("unexpected result: %#v", res)
	}
	if publisher.cmd.AdminUserID != "admin-2" || publisher.cmd.Topic != "devices/a/in" || publisher.cmd.QoS != 1 {
		t.Fatalf("unexpected publish command: %#v", publisher.cmd)
	}
}
