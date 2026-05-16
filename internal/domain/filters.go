package domain

import "time"

type DeviceFilter struct {
	Status   string
	Type     string
	Query    string
	Page     int
	PageSize int
}

type TopicFilter struct {
	Direction string
	Query     string
	Page      int
	PageSize  int
}

type MessageFilter struct {
	DeviceID string
	Topic    string
	From     *time.Time
	To       *time.Time
	Page     int
	PageSize int
}

type CommandFilter struct {
	Topic    string
	Status   string
	Page     int
	PageSize int
}

type AlertFilter struct {
	Level    string
	Status   string
	Page     int
	PageSize int
}

func NormalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}

func ApplyDefaultMessageRange(now time.Time, f MessageFilter) MessageFilter {
	if f.From == nil && f.To == nil {
		from := now.Add(-24 * time.Hour)
		to := now
		f.From = &from
		f.To = &to
	}
	f.Page, f.PageSize = NormalizePage(f.Page, f.PageSize)
	return f
}
