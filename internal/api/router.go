package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"mqtter/internal/domain"
)

type AuthService interface {
	Login(ctx context.Context, username, password string) (domain.AdminUserDTO, string, time.Time, error)
	Logout(ctx context.Context, token string) error
	ValidateSession(ctx context.Context, token string) (domain.AdminUserDTO, error)
}

type DeviceReader interface {
	ListDevices(ctx context.Context, f domain.DeviceFilter) (domain.Page[domain.DeviceDTO], error)
	GetDevice(ctx context.Context, id string) (domain.DeviceDTO, error)
}

type DeviceTypeChanger interface {
	ChangeDeviceType(ctx context.Context, cmd domain.ChangeDeviceTypeCommand) (domain.DeviceDTO, error)
	ListDeviceTypes(ctx context.Context) ([]domain.DeviceTypeDTO, error)
}

type TopicReader interface {
	ListDeviceTopics(ctx context.Context, deviceID string, f domain.TopicFilter) (domain.Page[domain.ObservedTopicDTO], error)
	ListTopics(ctx context.Context, f domain.TopicFilter) (domain.Page[domain.ObservedTopicDTO], error)
}

type MessageReader interface {
	QueryMessages(ctx context.Context, f domain.MessageFilter) (domain.Page[domain.MessageDTO], error)
}

type AdminPublisher interface {
	Publish(ctx context.Context, cmd domain.PublishCommand) (domain.PublishResult, error)
}

type CommandReader interface {
	ListPublishCommands(ctx context.Context, f domain.CommandFilter) (domain.Page[domain.PublishCommandDTO], error)
}

type ScheduledPublisher interface {
	CreateScheduledPublish(ctx context.Context, cmd domain.CreateScheduledPublishCommand) (domain.ScheduledPublishTaskDTO, error)
	ListScheduledPublishes(ctx context.Context, f domain.ScheduledPublishFilter) (domain.Page[domain.ScheduledPublishTaskDTO], error)
	CancelScheduledPublish(ctx context.Context, id string) (domain.ScheduledPublishTaskDTO, error)
}

type AlertReader interface {
	ListAlerts(ctx context.Context, f domain.AlertFilter) (domain.Page[domain.SystemAlert], error)
}

type Deps struct {
	Auth          AuthService
	Devices       DeviceReader
	DeviceTypes   DeviceTypeChanger
	Topics        TopicReader
	Messages      MessageReader
	Publisher     AdminPublisher
	Commands      CommandReader
	Scheduled     ScheduledPublisher
	Alerts        AlertReader
	Realtime      http.Handler
	SessionCookie string
}

type Router struct {
	deps Deps
}

func NewRouter(deps Deps) http.Handler {
	if deps.SessionCookie == "" {
		deps.SessionCookie = "mqtter_session"
	}
	rt := &Router{deps: deps}
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Route("/api", func(r chi.Router) {
		r.Post("/auth/login", rt.login)
		r.Group(func(r chi.Router) {
			r.Use(rt.requireAuth)
			r.Post("/auth/logout", rt.logout)
			r.Get("/me", rt.me)
			r.Get("/devices", rt.listDevices)
			r.Get("/devices/{deviceID}", rt.getDevice)
			r.Patch("/devices/{deviceID}/type", rt.changeDeviceType)
			r.Get("/devices/{deviceID}/topics", rt.listDeviceTopics)
			r.Get("/topics", rt.listTopics)
			r.Get("/messages", rt.queryMessages)
			r.Post("/publish", rt.publish)
			r.Get("/scheduled-publishes", rt.listScheduledPublishes)
			r.Post("/scheduled-publishes", rt.createScheduledPublish)
			r.Post("/scheduled-publishes/{taskID}/cancel", rt.cancelScheduledPublish)
			r.Get("/commands", rt.listCommands)
			r.Get("/alerts", rt.listAlerts)
			r.Get("/device-types", rt.listDeviceTypes)
			if deps.Realtime != nil {
				r.Get("/realtime", deps.Realtime.ServeHTTP)
			}
		})
	})

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	return r
}

func (rt *Router) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	user, token, expiresAt, err := rt.deps.Auth.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		writeError(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     rt.deps.SessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (rt *Router) logout(w http.ResponseWriter, r *http.Request) {
	token := rt.tokenFromCookie(r)
	if err := rt.deps.Auth.Logout(r.Context(), token); err != nil {
		writeError(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     rt.deps.SessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (rt *Router) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, currentUser(r.Context()))
}

func (rt *Router) listDevices(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, pageSize := pageParams(r)
	res, err := rt.deps.Devices.ListDevices(r.Context(), domain.DeviceFilter{
		Status:   q.Get("status"),
		Type:     q.Get("type"),
		Query:    q.Get("q"),
		Page:     page,
		PageSize: pageSize,
	})
	writeResult(w, r, res, err)
}

func (rt *Router) getDevice(w http.ResponseWriter, r *http.Request) {
	res, err := rt.deps.Devices.GetDevice(r.Context(), chi.URLParam(r, "deviceID"))
	writeResult(w, r, res, err)
}

func (rt *Router) changeDeviceType(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type string `json:"type"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	user := currentUser(r.Context())
	res, err := rt.deps.DeviceTypes.ChangeDeviceType(r.Context(), domain.ChangeDeviceTypeCommand{
		DeviceID: chi.URLParam(r, "deviceID"),
		Type:     req.Type,
		ActorID:  user.ID,
	})
	writeResult(w, r, res, err)
}

func (rt *Router) listDeviceTopics(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, pageSize := pageParams(r)
	res, err := rt.deps.Topics.ListDeviceTopics(r.Context(), chi.URLParam(r, "deviceID"), domain.TopicFilter{
		Direction: q.Get("direction"),
		Query:     q.Get("q"),
		Page:      page,
		PageSize:  pageSize,
	})
	writeResult(w, r, res, err)
}

func (rt *Router) listTopics(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, pageSize := pageParams(r)
	res, err := rt.deps.Topics.ListTopics(r.Context(), domain.TopicFilter{
		Direction: q.Get("direction"),
		Query:     q.Get("q"),
		Page:      page,
		PageSize:  pageSize,
	})
	writeResult(w, r, res, err)
}

func (rt *Router) queryMessages(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, pageSize := pageParams(r)
	from, err := parseOptionalTime(q.Get("from"))
	if err != nil {
		writeError(w, r, domain.InvalidInput("invalid_from", "from must be RFC3339 time"))
		return
	}
	to, err := parseOptionalTime(q.Get("to"))
	if err != nil {
		writeError(w, r, domain.InvalidInput("invalid_to", "to must be RFC3339 time"))
		return
	}
	res, err := rt.deps.Messages.QueryMessages(r.Context(), domain.MessageFilter{
		DeviceID: q.Get("deviceId"),
		Topic:    q.Get("topic"),
		From:     from,
		To:       to,
		Page:     page,
		PageSize: pageSize,
	})
	writeResult(w, r, res, err)
}

func (rt *Router) publish(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Topic           string `json:"topic"`
		Payload         string `json:"payload"`
		PayloadEncoding string `json:"payloadEncoding"`
		QoS             byte   `json:"qos"`
		Retain          bool   `json:"retain"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.PayloadEncoding != "" && req.PayloadEncoding != "utf8" && req.PayloadEncoding != "json" {
		writeError(w, r, domain.InvalidInput("unsupported_payload_encoding", "payloadEncoding must be utf8 or json"))
		return
	}
	user := currentUser(r.Context())
	res, err := rt.deps.Publisher.Publish(r.Context(), domain.PublishCommand{
		AdminUserID: user.ID,
		Topic:       req.Topic,
		PayloadText: req.Payload,
		QoS:         req.QoS,
		Retain:      req.Retain,
	})
	writeResult(w, r, res, err)
}

func (rt *Router) listCommands(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, pageSize := pageParams(r)
	res, err := rt.deps.Commands.ListPublishCommands(r.Context(), domain.CommandFilter{
		Topic:    q.Get("topic"),
		Status:   q.Get("status"),
		Page:     page,
		PageSize: pageSize,
	})
	writeResult(w, r, res, err)
}

func (rt *Router) listScheduledPublishes(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, pageSize := pageParams(r)
	res, err := rt.deps.Scheduled.ListScheduledPublishes(r.Context(), domain.ScheduledPublishFilter{
		DeviceID: q.Get("deviceId"),
		Status:   q.Get("status"),
		Page:     page,
		PageSize: pageSize,
	})
	writeResult(w, r, res, err)
}

func (rt *Router) createScheduledPublish(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID        string     `json:"deviceId"`
		Name            string     `json:"name"`
		Topic           string     `json:"topic"`
		Payload         string     `json:"payload"`
		PayloadEncoding string     `json:"payloadEncoding"`
		QoS             byte       `json:"qos"`
		Retain          bool       `json:"retain"`
		ScheduleType    string     `json:"scheduleType"`
		RunAt           *time.Time `json:"runAt"`
		TimeOfDay       string     `json:"timeOfDay"`
		Weekdays        []int      `json:"weekdays"`
		Timezone        string     `json:"timezone"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.PayloadEncoding != "" && req.PayloadEncoding != "utf8" && req.PayloadEncoding != "json" {
		writeError(w, r, domain.InvalidInput("unsupported_payload_encoding", "payloadEncoding must be utf8 or json"))
		return
	}
	user := currentUser(r.Context())
	res, err := rt.deps.Scheduled.CreateScheduledPublish(r.Context(), domain.CreateScheduledPublishCommand{
		AdminUserID:  user.ID,
		DeviceID:     req.DeviceID,
		Name:         req.Name,
		Topic:        req.Topic,
		PayloadText:  req.Payload,
		QoS:          req.QoS,
		Retain:       req.Retain,
		ScheduleType: domain.ScheduleType(req.ScheduleType),
		RunAt:        req.RunAt,
		TimeOfDay:    req.TimeOfDay,
		Weekdays:     req.Weekdays,
		Timezone:     req.Timezone,
	})
	writeResult(w, r, res, err)
}

func (rt *Router) cancelScheduledPublish(w http.ResponseWriter, r *http.Request) {
	res, err := rt.deps.Scheduled.CancelScheduledPublish(r.Context(), chi.URLParam(r, "taskID"))
	writeResult(w, r, res, err)
}

func (rt *Router) listAlerts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, pageSize := pageParams(r)
	res, err := rt.deps.Alerts.ListAlerts(r.Context(), domain.AlertFilter{
		Level:    q.Get("level"),
		Status:   q.Get("status"),
		Page:     page,
		PageSize: pageSize,
	})
	writeResult(w, r, res, err)
}

func (rt *Router) listDeviceTypes(w http.ResponseWriter, r *http.Request) {
	res, err := rt.deps.DeviceTypes.ListDeviceTypes(r.Context())
	writeResult(w, r, res, err)
}

func (rt *Router) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := rt.deps.Auth.ValidateSession(r.Context(), rt.tokenFromCookie(r))
		if err != nil {
			writeError(w, r, domain.InvalidInput("unauthorized", "authentication required"))
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey{}, user)))
	})
}

func (rt *Router) tokenFromCookie(r *http.Request) string {
	cookie, err := r.Cookie(rt.deps.SessionCookie)
	if err != nil {
		return ""
	}
	return cookie.Value
}

type userContextKey struct{}

func currentUser(ctx context.Context) domain.AdminUserDTO {
	user, _ := ctx.Value(userContextKey{}).(domain.AdminUserDTO)
	return user
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, r, domain.InvalidInput("invalid_json", "request body must be valid JSON"))
		return false
	}
	return true
}

func writeResult(w http.ResponseWriter, r *http.Request, result any, err error) {
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	code := domain.ErrorCode(err)
	if code == "unauthorized" {
		status = http.StatusUnauthorized
	} else if errors.As(err, new(*domain.AppError)) {
		status = http.StatusBadRequest
	}
	requestID := middleware.GetReqID(r.Context())
	writeJSON(w, status, map[string]any{
		"error": map[string]string{
			"code":      code,
			"message":   domain.ErrorMessage(err),
			"requestId": requestID,
		},
	})
}

func pageParams(r *http.Request) (int, int) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	pageSize, _ := strconv.Atoi(q.Get("pageSize"))
	return domain.NormalizePage(page, pageSize)
}

func parseOptionalTime(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
