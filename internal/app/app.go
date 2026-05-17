package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"mqtter/internal/api"
	"mqtter/internal/broker"
	"mqtter/internal/config"
	"mqtter/internal/realtime"
	"mqtter/internal/service"
	"mqtter/internal/storage/postgres"
)

type Application struct {
	httpServer *http.Server
	broker     *broker.Server
	store      *postgres.Store
	ingestor   *service.Ingestor
	scheduler  *service.ScheduledPublishScheduler
	logger     *slog.Logger
}

func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*Application, error) {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}

	store, err := postgres.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	if err := store.ApplyMigrations(ctx); err != nil {
		store.Close()
		return nil, err
	}

	hub := realtime.NewHub(128)
	clock := service.SystemClock{}
	ids := service.RandomIDGenerator{}
	auth := service.NewAuthService(store, service.BcryptHasher{}, ids, clock, cfg.SessionTTL)
	if err := auth.BootstrapAdmin(ctx, cfg.BootstrapUsername, cfg.BootstrapPassword); err != nil {
		store.Close()
		return nil, err
	}

	ingestor := service.NewIngestor(store, store, hub, clock, service.IngestorConfig{
		Timeout:         cfg.IngestTimeout,
		MaxPayloadBytes: cfg.MaxPayloadBytes,
	})
	mqttBroker, err := broker.NewServer(broker.Config{
		MQTTAddr:        cfg.MQTTAddr,
		StorePath:       cfg.BrokerStorePath,
		MaxPayloadBytes: cfg.MaxPayloadBytes,
	}, ingestor, logger)
	if err != nil {
		store.Close()
		return nil, err
	}

	deviceSvc := service.NewDeviceService(store, store, clock)
	messageSvc := service.NewMessageService(store, clock)
	publishSvc := service.NewPublishService(store, store, mqttBroker, clock, ids, cfg.MaxPayloadBytes)
	scheduledSvc := service.NewScheduledPublishService(store, publishSvc, clock, ids, logger, cfg.MaxPayloadBytes)
	quickActionSvc := service.NewQuickActionService(store, store, publishSvc, clock, ids, cfg.MaxPayloadBytes)
	scheduler := service.NewScheduledPublishScheduler(scheduledSvc, cfg.SchedulerInterval, logger)
	alertSvc := service.NewAlertService(store)

	handler := api.NewRouter(api.Deps{
		Auth:          auth,
		Devices:       deviceSvc,
		DeviceTypes:   deviceSvc,
		Topics:        deviceSvc,
		Messages:      messageSvc,
		Publisher:     publishSvc,
		Commands:      publishSvc,
		Scheduled:     scheduledSvc,
		QuickActions:  quickActionSvc,
		Alerts:        alertSvc,
		Realtime:      hub,
		SessionCookie: cfg.SessionCookieName,
	})

	return &Application{
		httpServer: &http.Server{Addr: cfg.HTTPAddr, Handler: handler},
		broker:     mqttBroker,
		store:      store,
		ingestor:   ingestor,
		scheduler:  scheduler,
		logger:     logger,
	}, nil
}

func (a *Application) Run(ctx context.Context) error {
	if _, err := a.ingestor.MarkStaleOnline(ctx); err != nil {
		return err
	}

	a.logger.Info("mqtt broker starting")
	if err := a.broker.Serve(); err != nil {
		a.store.Close()
		return err
	}
	go a.scheduler.Run(ctx)

	errs := make(chan error, 1)
	go func() {
		a.logger.Info("http server starting", "addr", a.httpServer.Addr)
		err := a.httpServer.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		errs <- err
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), a.shutdownTimeout())
		defer cancel()
		_ = a.httpServer.Shutdown(shutdownCtx)
		_ = a.broker.Close()
		a.store.Close()
		return nil
	case err := <-errs:
		_ = a.broker.Close()
		_ = a.httpServer.Close()
		a.store.Close()
		return err
	}
}

func (a *Application) shutdownTimeout() time.Duration {
	return 10 * time.Second
}
