package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mqtter/internal/domain"
)

type fakeAuth struct{}

func (fakeAuth) Login(context.Context, string, string) (domain.AdminUserDTO, string, time.Time, error) {
	return domain.AdminUserDTO{}, "", time.Time{}, nil
}

func (fakeAuth) Logout(context.Context, string) error { return nil }

func (fakeAuth) ValidateSession(context.Context, string) (domain.AdminUserDTO, error) {
	return domain.AdminUserDTO{ID: "admin-1", Username: "root", Role: "admin"}, nil
}

type fakeAdminPublisher struct {
	cmd domain.PublishCommand
}

func (p *fakeAdminPublisher) Publish(_ context.Context, cmd domain.PublishCommand) (domain.PublishResult, error) {
	p.cmd = cmd
	return domain.PublishResult{CommandID: "cmd-1", Status: domain.PublishStatusPublished, PublishedAt: time.Date(2026, 5, 15, 13, 0, 0, 0, time.UTC)}, nil
}

func TestPublishHandlerUsesAuthenticatedAdminAndReturnsResult(t *testing.T) {
	publisher := &fakeAdminPublisher{}
	router := NewRouter(Deps{
		Auth:          fakeAuth{},
		Publisher:     publisher,
		SessionCookie: "sid",
	})
	body := bytes.NewBufferString(`{"topic":"devices/a/in","payload":"hello","qos":1,"retain":false}`)
	req := httptest.NewRequest(http.MethodPost, "/api/publish", body)
	req.AddCookie(&http.Cookie{Name: "sid", Value: "ok"})
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if publisher.cmd.AdminUserID != "admin-1" {
		t.Fatalf("expected admin id in command, got %q", publisher.cmd.AdminUserID)
	}
	var res domain.PublishResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if res.CommandID != "cmd-1" {
		t.Fatalf("unexpected command id %q", res.CommandID)
	}
}
