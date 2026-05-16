package domain

import (
	"testing"
	"time"
)

func TestNextRunForOneTimeSchedule(t *testing.T) {
	now := time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)
	runAt := now.Add(10 * time.Minute)

	next, err := NextRunForCommand(CreateScheduledPublishCommand{
		ScheduleType: ScheduleTypeOnce,
		RunAt:        &runAt,
	}, now)
	if err != nil {
		t.Fatalf("NextRunForCommand returned error: %v", err)
	}
	if !next.Equal(runAt) {
		t.Fatalf("expected %s, got %s", runAt, next)
	}
}

func TestNextRunForWeeklyScheduleUsesWeekdaysAndTimezone(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Hong_Kong")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 5, 16, 10, 0, 0, 0, loc) // Saturday.

	next, err := NextRunForCommand(CreateScheduledPublishCommand{
		ScheduleType: ScheduleTypeWeekly,
		TimeOfDay:    "09:30",
		Weekdays:     []int{1},
		Timezone:     "Asia/Hong_Kong",
	}, now)
	if err != nil {
		t.Fatalf("NextRunForCommand returned error: %v", err)
	}

	want := time.Date(2026, 5, 18, 9, 30, 0, 0, loc).UTC()
	if !next.Equal(want) {
		t.Fatalf("expected %s, got %s", want, next)
	}
}

func TestNextRunForDailyScheduleMovesToTomorrowAfterTimePassed(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Hong_Kong")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 5, 16, 10, 0, 0, 0, loc)

	next, err := NextRunForCommand(CreateScheduledPublishCommand{
		ScheduleType: ScheduleTypeDaily,
		TimeOfDay:    "09:30",
		Timezone:     "Asia/Hong_Kong",
	}, now)
	if err != nil {
		t.Fatalf("NextRunForCommand returned error: %v", err)
	}

	want := time.Date(2026, 5, 17, 9, 30, 0, 0, loc).UTC()
	if !next.Equal(want) {
		t.Fatalf("expected %s, got %s", want, next)
	}
}
