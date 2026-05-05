package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"discord-reminder/config"
	"discord-reminder/notifier"
	"discord-reminder/scheduler"
)

const httpTimeout = 10 * time.Second

func main() {
	if err := run(); err != nil {
		slog.Error("startup failed", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load("config.toml", "secrets.toml")
	if err != nil {
		return err
	}

	setupLogger(cfg.System.LogLevel)

	httpClient := &http.Client{Timeout: httpTimeout}
	notif := notifier.NewDiscord(httpClient)
	sched := scheduler.New(cfg, notif)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	slog.Info("scheduler started",
		"reminders", len(cfg.Reminders),
		"interval", cfg.System.TickInterval,
		"timezone", cfg.System.Location.String(),
	)

	return sched.Run(ctx)
}

func setupLogger(level string) {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})))
}
