package domain

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type ScheduleType string

const (
	ScheduleTypeOnce   ScheduleType = "once"
	ScheduleTypeDaily  ScheduleType = "daily"
	ScheduleTypeWeekly ScheduleType = "weekly"
)

type ScheduledTaskStatus string

const (
	ScheduledTaskStatusActive    ScheduledTaskStatus = "active"
	ScheduledTaskStatusCanceled  ScheduledTaskStatus = "canceled"
	ScheduledTaskStatusCompleted ScheduledTaskStatus = "completed"
	ScheduledTaskStatusFailed    ScheduledTaskStatus = "failed"
)

type ScheduledRunStatus string

const (
	ScheduledRunStatusPublished ScheduledRunStatus = "published"
	ScheduledRunStatusFailed    ScheduledRunStatus = "failed"
)

type CreateScheduledPublishCommand struct {
	AdminUserID  string       `json:"-"`
	DeviceID     string       `json:"deviceId"`
	Name         string       `json:"name"`
	Topic        string       `json:"topic"`
	PayloadText  string       `json:"payload"`
	QoS          byte         `json:"qos"`
	Retain       bool         `json:"retain"`
	ScheduleType ScheduleType `json:"scheduleType"`
	RunAt        *time.Time   `json:"runAt,omitempty"`
	TimeOfDay    string       `json:"timeOfDay,omitempty"`
	Weekdays     []int        `json:"weekdays,omitempty"`
	Timezone     string       `json:"timezone"`
}

type ScheduledPublishTaskDTO struct {
	ID           string              `json:"id"`
	DeviceID     string              `json:"deviceId"`
	ClientID     string              `json:"clientId,omitempty"`
	AdminUserID  string              `json:"adminUserId"`
	Name         string              `json:"name"`
	Topic        string              `json:"topic"`
	PayloadText  string              `json:"payload"`
	QoS          byte                `json:"qos"`
	Retain       bool                `json:"retain"`
	ScheduleType ScheduleType        `json:"scheduleType"`
	RunAt        *time.Time          `json:"runAt,omitempty"`
	TimeOfDay    string              `json:"timeOfDay,omitempty"`
	Weekdays     []int               `json:"weekdays,omitempty"`
	Timezone     string              `json:"timezone"`
	Status       ScheduledTaskStatus `json:"status"`
	NextRunAt    *time.Time          `json:"nextRunAt,omitempty"`
	LastRunAt    *time.Time          `json:"lastRunAt,omitempty"`
	LastError    string              `json:"lastError,omitempty"`
	RunCount     int                 `json:"runCount"`
	CreatedAt    time.Time           `json:"createdAt"`
	UpdatedAt    time.Time           `json:"updatedAt"`
}

type ScheduledPublishRunDTO struct {
	ID               string             `json:"id"`
	TaskID           string             `json:"taskId"`
	PublishCommandID string             `json:"publishCommandId,omitempty"`
	Status           ScheduledRunStatus `json:"status"`
	Error            string             `json:"error,omitempty"`
	ScheduledFor     time.Time          `json:"scheduledFor"`
	StartedAt        time.Time          `json:"startedAt"`
	FinishedAt       time.Time          `json:"finishedAt"`
}

type ScheduledPublishFilter struct {
	DeviceID string
	Status   string
	Page     int
	PageSize int
}

type ScheduledPublishFinish struct {
	TaskID           string
	RunID            string
	PublishCommandID string
	RunStatus        ScheduledRunStatus
	TaskStatus       ScheduledTaskStatus
	Error            string
	ScheduledFor     time.Time
	StartedAt        time.Time
	FinishedAt       time.Time
	NextRunAt        *time.Time
}

func ValidateScheduleCommand(cmd CreateScheduledPublishCommand, now time.Time) (time.Time, error) {
	if strings.TrimSpace(cmd.DeviceID) == "" {
		return time.Time{}, InvalidInput("invalid_device_id", "device id must not be empty")
	}
	if err := ValidatePublishTopic(cmd.Topic); err != nil {
		return time.Time{}, err
	}
	if err := ValidateQoS(cmd.QoS); err != nil {
		return time.Time{}, err
	}
	switch cmd.ScheduleType {
	case ScheduleTypeOnce, ScheduleTypeDaily, ScheduleTypeWeekly:
	default:
		return time.Time{}, InvalidInput("invalid_schedule_type", "scheduleType must be once, daily, or weekly")
	}
	return NextRunForCommand(cmd, now)
}

func NextRunForCommand(cmd CreateScheduledPublishCommand, now time.Time) (time.Time, error) {
	switch cmd.ScheduleType {
	case ScheduleTypeOnce:
		if cmd.RunAt == nil {
			return time.Time{}, InvalidInput("invalid_run_at", "runAt is required for one-time schedules")
		}
		if !cmd.RunAt.After(now) {
			return time.Time{}, InvalidInput("invalid_run_at", "runAt must be in the future")
		}
		return cmd.RunAt.UTC(), nil
	case ScheduleTypeDaily:
		return nextDailyRun(cmd.TimeOfDay, cmd.Timezone, now)
	case ScheduleTypeWeekly:
		return nextWeeklyRun(cmd.TimeOfDay, cmd.Timezone, cmd.Weekdays, now)
	default:
		return time.Time{}, InvalidInput("invalid_schedule_type", "scheduleType must be once, daily, or weekly")
	}
}

func NextRunAfterTask(task ScheduledPublishTaskDTO, after time.Time) (*time.Time, error) {
	if task.ScheduleType == ScheduleTypeOnce {
		return nil, nil
	}
	cmd := CreateScheduledPublishCommand{
		ScheduleType: task.ScheduleType,
		TimeOfDay:    task.TimeOfDay,
		Weekdays:     task.Weekdays,
		Timezone:     task.Timezone,
	}
	next, err := NextRunForCommand(cmd, after)
	if err != nil {
		return nil, err
	}
	return &next, nil
}

func nextDailyRun(timeOfDay, timezone string, now time.Time) (time.Time, error) {
	hour, minute, err := parseTimeOfDay(timeOfDay)
	if err != nil {
		return time.Time{}, err
	}
	loc, err := scheduleLocation(timezone)
	if err != nil {
		return time.Time{}, err
	}
	localNow := now.In(loc)
	candidate := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), hour, minute, 0, 0, loc)
	if !candidate.After(localNow) {
		candidate = candidate.AddDate(0, 0, 1)
	}
	return candidate.UTC(), nil
}

func nextWeeklyRun(timeOfDay, timezone string, weekdays []int, now time.Time) (time.Time, error) {
	hour, minute, err := parseTimeOfDay(timeOfDay)
	if err != nil {
		return time.Time{}, err
	}
	if len(weekdays) == 0 {
		return time.Time{}, InvalidInput("invalid_weekdays", "at least one weekday is required")
	}
	allowed := map[int]bool{}
	for _, day := range weekdays {
		if day < 1 || day > 7 {
			return time.Time{}, InvalidInput("invalid_weekdays", "weekdays must be 1-7")
		}
		allowed[day] = true
	}
	loc, err := scheduleLocation(timezone)
	if err != nil {
		return time.Time{}, err
	}
	localNow := now.In(loc)
	for offset := 0; offset < 8; offset++ {
		day := localNow.AddDate(0, 0, offset)
		weekday := int(day.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		if !allowed[weekday] {
			continue
		}
		candidate := time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, loc)
		if candidate.After(localNow) {
			return candidate.UTC(), nil
		}
	}
	return time.Time{}, InvalidInput("invalid_weekdays", "could not calculate next weekly run")
}

func parseTimeOfDay(value string) (int, int, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0, 0, InvalidInput("invalid_time_of_day", "timeOfDay must use HH:mm")
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, InvalidInput("invalid_time_of_day", "timeOfDay must use HH:mm")
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, InvalidInput("invalid_time_of_day", "timeOfDay must use HH:mm")
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, 0, InvalidInput("invalid_time_of_day", "timeOfDay must use HH:mm")
	}
	return hour, minute, nil
}

func scheduleLocation(timezone string) (*time.Location, error) {
	if strings.TrimSpace(timezone) == "" {
		timezone = "Asia/Hong_Kong"
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, InvalidInput("invalid_timezone", fmt.Sprintf("unsupported timezone %q", timezone))
	}
	return loc, nil
}

func EncodeWeekdays(days []int) string {
	if len(days) == 0 {
		return ""
	}
	parts := make([]string, 0, len(days))
	for _, day := range days {
		parts = append(parts, strconv.Itoa(day))
	}
	return strings.Join(parts, ",")
}

func DecodeWeekdays(value string) []int {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		day, err := strconv.Atoi(strings.TrimSpace(part))
		if err == nil {
			out = append(out, day)
		}
	}
	return out
}
