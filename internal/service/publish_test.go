package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"mqtter/internal/domain"
)

type fakeCommandRepo struct {
	created []domain.PublishCommand
	marked  []domain.PublishStatus
}

func (r *fakeCommandRepo) CreatePublishCommand(_ context.Context, id string, cmd domain.PublishCommand, createdAt time.Time) (domain.PublishCommandDTO, error) {
	r.created = append(r.created, cmd)
	return domain.PublishCommandDTO{ID: id, AdminUserID: cmd.AdminUserID, Topic: cmd.Topic, Status: domain.PublishStatusPending, CreatedAt: createdAt}, nil
}

func (r *fakeCommandRepo) MarkPublishCommand(_ context.Context, _ string, status domain.PublishStatus, _ string, _ *time.Time) error {
	r.marked = append(r.marked, status)
	return nil
}

func (r *fakeCommandRepo) ListPublishCommands(context.Context, domain.CommandFilter) (domain.Page[domain.PublishCommandDTO], error) {
	return domain.Page[domain.PublishCommandDTO]{}, nil
}

type fakeTopicRepo struct {
	adminTopics []string
}

func (r *fakeTopicRepo) ListDeviceTopics(context.Context, string, domain.TopicFilter) (domain.Page[domain.ObservedTopicDTO], error) {
	return domain.Page[domain.ObservedTopicDTO]{}, nil
}

func (r *fakeTopicRepo) ListTopics(context.Context, domain.TopicFilter) (domain.Page[domain.ObservedTopicDTO], error) {
	return domain.Page[domain.ObservedTopicDTO]{}, nil
}

func (r *fakeTopicRepo) RecordAdminPublishTopic(_ context.Context, _ string, topic string, _ byte, _ time.Time) error {
	r.adminTopics = append(r.adminTopics, topic)
	return nil
}

type fakeMQTTPublisher struct {
	called bool
	err    error
}

func (p *fakeMQTTPublisher) Publish(context.Context, string, string, domain.PublishOptions) error {
	p.called = true
	return p.err
}

type fixedIDs struct{}

func (fixedIDs) NewID() string { return "cmd-1" }

func TestPublishServicePublishesAndAuditsCommand(t *testing.T) {
	commands := &fakeCommandRepo{}
	topics := &fakeTopicRepo{}
	publisher := &fakeMQTTPublisher{}
	svc := NewPublishService(commands, topics, publisher, fixedClock{t: time.Date(2026, 5, 15, 13, 0, 0, 0, time.UTC)}, fixedIDs{}, 1024)

	res, err := svc.Publish(context.Background(), domain.PublishCommand{
		AdminUserID: "admin-1",
		Topic:       "devices/a/in",
		PayloadText: "hello",
		QoS:         1,
	})
	if err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}
	if res.CommandID != "cmd-1" || res.Status != domain.PublishStatusPublished {
		t.Fatalf("unexpected publish result: %#v", res)
	}
	if !publisher.called {
		t.Fatal("expected MQTT publisher to be called")
	}
	if len(commands.marked) != 1 || commands.marked[0] != domain.PublishStatusPublished {
		t.Fatalf("expected published command mark, got %#v", commands.marked)
	}
	if len(topics.adminTopics) != 1 || topics.adminTopics[0] != "devices/a/in" {
		t.Fatalf("expected admin topic to be recorded, got %#v", topics.adminTopics)
	}
}

func TestPublishServiceRejectsWildcardTopicBeforeAudit(t *testing.T) {
	commands := &fakeCommandRepo{}
	publisher := &fakeMQTTPublisher{}
	svc := NewPublishService(commands, nil, publisher, fixedClock{t: time.Now().UTC()}, fixedIDs{}, 1024)

	_, err := svc.Publish(context.Background(), domain.PublishCommand{
		AdminUserID: "admin-1",
		Topic:       "devices/#",
		PayloadText: "hello",
	})
	if err == nil {
		t.Fatal("expected invalid topic error")
	}
	if publisher.called || len(commands.created) != 0 {
		t.Fatal("invalid command should not be audited or published")
	}
}

func TestPublishServiceMarksCommandFailedWhenBrokerFails(t *testing.T) {
	commands := &fakeCommandRepo{}
	publisher := &fakeMQTTPublisher{err: errors.New("broker down")}
	svc := NewPublishService(commands, nil, publisher, fixedClock{t: time.Now().UTC()}, fixedIDs{}, 1024)

	_, err := svc.Publish(context.Background(), domain.PublishCommand{
		AdminUserID: "admin-1",
		Topic:       "devices/a/in",
		PayloadText: "hello",
	})
	if err == nil {
		t.Fatal("expected broker error")
	}
	if len(commands.marked) != 1 || commands.marked[0] != domain.PublishStatusFailed {
		t.Fatalf("expected failed command mark, got %#v", commands.marked)
	}
}
