package service

import (
	"context"
	"log/slog"
	"time"

	"mqtter/internal/domain"
	"mqtter/internal/ports"
)

type ScheduledPublishService struct {
	repo       ports.ScheduledPublishRepository
	publisher  *PublishService
	clock      ports.Clock
	ids        ports.IDGenerator
	logger     *slog.Logger
	maxPayload int
}

func NewScheduledPublishService(repo ports.ScheduledPublishRepository, publisher *PublishService, clock ports.Clock, ids ports.IDGenerator, logger *slog.Logger, maxPayload int) *ScheduledPublishService {
	if clock == nil {
		clock = SystemClock{}
	}
	if ids == nil {
		ids = RandomIDGenerator{}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &ScheduledPublishService{repo: repo, publisher: publisher, clock: clock, ids: ids, logger: logger, maxPayload: maxPayload}
}

func (s *ScheduledPublishService) CreateScheduledPublish(ctx context.Context, cmd domain.CreateScheduledPublishCommand) (domain.ScheduledPublishTaskDTO, error) {
	if _, err := domain.ValidateTextPayload(cmd.PayloadText, s.maxPayload); err != nil {
		return domain.ScheduledPublishTaskDTO{}, err
	}
	now := s.clock.Now()
	nextRunAt, err := domain.ValidateScheduleCommand(cmd, now)
	if err != nil {
		return domain.ScheduledPublishTaskDTO{}, err
	}
	if cmd.Timezone == "" {
		cmd.Timezone = "Asia/Hong_Kong"
	}
	task := domain.ScheduledPublishTaskDTO{
		ID:           s.ids.NewID(),
		DeviceID:     cmd.DeviceID,
		AdminUserID:  cmd.AdminUserID,
		Name:         cmd.Name,
		Topic:        cmd.Topic,
		PayloadText:  cmd.PayloadText,
		QoS:          cmd.QoS,
		Retain:       cmd.Retain,
		ScheduleType: cmd.ScheduleType,
		RunAt:        cmd.RunAt,
		TimeOfDay:    cmd.TimeOfDay,
		Weekdays:     cmd.Weekdays,
		Timezone:     cmd.Timezone,
		Status:       domain.ScheduledTaskStatusActive,
		NextRunAt:    &nextRunAt,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	return s.repo.CreateScheduledTask(ctx, task)
}

func (s *ScheduledPublishService) ListScheduledPublishes(ctx context.Context, f domain.ScheduledPublishFilter) (domain.Page[domain.ScheduledPublishTaskDTO], error) {
	f.Page, f.PageSize = domain.NormalizePage(f.Page, f.PageSize)
	return s.repo.ListScheduledTasks(ctx, f)
}

func (s *ScheduledPublishService) CancelScheduledPublish(ctx context.Context, id string) (domain.ScheduledPublishTaskDTO, error) {
	if id == "" {
		return domain.ScheduledPublishTaskDTO{}, domain.InvalidInput("invalid_task_id", "task id must not be empty")
	}
	return s.repo.CancelScheduledTask(ctx, id, s.clock.Now())
}

func (s *ScheduledPublishService) RunDue(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 20
	}
	now := s.clock.Now()
	tasks, err := s.repo.ListDueScheduledTasks(ctx, now, limit)
	if err != nil {
		return 0, err
	}
	for _, task := range tasks {
		if err := s.runOne(ctx, task, now); err != nil {
			s.logger.Warn("scheduled publish failed", "task", task.ID, "error", err)
		}
	}
	return len(tasks), nil
}

func (s *ScheduledPublishService) runOne(ctx context.Context, task domain.ScheduledPublishTaskDTO, startedAt time.Time) error {
	scheduledFor := startedAt
	if task.NextRunAt != nil {
		scheduledFor = *task.NextRunAt
	}

	result, err := s.publisher.Publish(ctx, domain.PublishCommand{
		AdminUserID: task.AdminUserID,
		Topic:       task.Topic,
		PayloadText: task.PayloadText,
		QoS:         task.QoS,
		Retain:      task.Retain,
	})

	finishedAt := s.clock.Now()
	runStatus := domain.ScheduledRunStatusPublished
	taskStatus := domain.ScheduledTaskStatusCompleted
	errText := ""
	publishCommandID := result.CommandID
	var nextRunAt *time.Time

	if err != nil {
		runStatus = domain.ScheduledRunStatusFailed
		errText = err.Error()
		if task.ScheduleType == domain.ScheduleTypeOnce {
			taskStatus = domain.ScheduledTaskStatusFailed
		} else {
			taskStatus = domain.ScheduledTaskStatusActive
			next, nextErr := domain.NextRunAfterTask(task, finishedAt)
			if nextErr != nil {
				taskStatus = domain.ScheduledTaskStatusFailed
				errText = errText + "; next run calculation failed: " + nextErr.Error()
			} else {
				nextRunAt = next
			}
		}
	} else if task.ScheduleType == domain.ScheduleTypeOnce {
		taskStatus = domain.ScheduledTaskStatusCompleted
	} else {
		taskStatus = domain.ScheduledTaskStatusActive
		next, nextErr := domain.NextRunAfterTask(task, finishedAt)
		if nextErr != nil {
			taskStatus = domain.ScheduledTaskStatusFailed
			errText = nextErr.Error()
		} else {
			nextRunAt = next
		}
	}

	finish := domain.ScheduledPublishFinish{
		TaskID:           task.ID,
		RunID:            s.ids.NewID(),
		PublishCommandID: publishCommandID,
		RunStatus:        runStatus,
		TaskStatus:       taskStatus,
		Error:            errText,
		ScheduledFor:     scheduledFor,
		StartedAt:        startedAt,
		FinishedAt:       finishedAt,
		NextRunAt:        nextRunAt,
	}
	if finishErr := s.repo.FinishScheduledRun(ctx, finish); finishErr != nil {
		return finishErr
	}
	return err
}

type ScheduledPublishScheduler struct {
	service  *ScheduledPublishService
	interval time.Duration
	logger   *slog.Logger
}

func NewScheduledPublishScheduler(service *ScheduledPublishService, interval time.Duration, logger *slog.Logger) *ScheduledPublishScheduler {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &ScheduledPublishScheduler{service: service, interval: interval, logger: logger}
}

func (s *ScheduledPublishScheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		if _, err := s.service.RunDue(ctx, 20); err != nil && ctx.Err() == nil {
			s.logger.Warn("scheduled publish poll failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
