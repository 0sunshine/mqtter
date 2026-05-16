package service

import (
	"context"

	"mqtter/internal/domain"
	"mqtter/internal/ports"
)

type AlertService struct {
	alerts ports.AlertRepository
}

func NewAlertService(alerts ports.AlertRepository) *AlertService {
	return &AlertService{alerts: alerts}
}

func (s *AlertService) ListAlerts(ctx context.Context, f domain.AlertFilter) (domain.Page[domain.SystemAlert], error) {
	f.Page, f.PageSize = domain.NormalizePage(f.Page, f.PageSize)
	return s.alerts.ListAlerts(ctx, f)
}
