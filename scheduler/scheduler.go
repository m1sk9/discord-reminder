// Package scheduler runs the periodic tick loop and dispatches notifications
// to a Notifier when reminders should fire.
package scheduler

import (
	"context"
	"log/slog"
	"time"

	"discord-reminder/config"
)

// Notifier は scheduler が必要とする最小インターフェースです．
// (notifier package の Discord はこれを満たします)
type Notifier interface {
	Send(ctx context.Context, webhookURL, message string) error
}

// Scheduler は cfg.Reminders を監視し，マッチした分に Notifier.Send を呼びます．
type Scheduler struct {
	cfg      *config.Config
	notifier Notifier

	// lastFired はリマインダー名 -> 最後に発火した分(秒切り捨て) のマップ．
	// 同分内で複数 tick しても 1 回だけ送るための重複排除に使います．
	// キーはリマインダー数で bounded のため，掃除不要です．
	lastFired map[string]time.Time
}

// New は cfg と notifier を受け取って Scheduler を構築します．
func New(cfg *config.Config, n Notifier) *Scheduler {
	return &Scheduler{
		cfg:       cfg,
		notifier:  n,
		lastFired: make(map[string]time.Time, len(cfg.Reminders)),
	}
}

// Run は ctx がキャンセルされるまでチェックループを回します．
// ctx.Done 受信時は nil を返して正常終了とします．
func (s *Scheduler) Run(ctx context.Context) error {
	ticker := time.NewTicker(s.cfg.System.TickInterval)
	defer ticker.Stop()

	// 起動直後の即時チェック (ticker の最初の tick まで待たない)
	s.tick(ctx, time.Now().In(s.cfg.System.Location))

	for {
		select {
		case <-ctx.Done():
			slog.Info("shutdown signal received, stopping scheduler")
			return nil
		case t := <-ticker.C:
			s.tick(ctx, t.In(s.cfg.System.Location))
		}
	}
}

// tick は now 時点で発火すべき reminder を全て処理します．
// notifier.Send の失敗は log に出すだけで，他のリマインダーの処理は継続します．
func (s *Scheduler) tick(ctx context.Context, now time.Time) {
	currentMinute := now.Truncate(time.Minute)

	for _, r := range s.cfg.Reminders {
		if !r.ShouldFire(now) {
			continue
		}
		if last, ok := s.lastFired[r.Name]; ok && last.Equal(currentMinute) {
			continue
		}
		s.lastFired[r.Name] = currentMinute

		if err := s.notifier.Send(ctx, r.WebhookURL, r.Message); err != nil {
			slog.Error("send failed", "reminder", r.Name, "err", err)
			continue
		}
		slog.Info("send succeeded", "reminder", r.Name)
	}
}
