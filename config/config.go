// Package config loads and validates configuration from TOML files.
//
// 全ての検証は Load 時に行います．不正な値があれば fail-fast し，
// scheduler に到達した時点では型レベルで正常な値が保証されます．
package config

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"discord-reminder/reminder"
)

// System は config.toml の [system] セクションを検証済み形式で保持します．
type System struct {
	LogLevel     string
	TickInterval time.Duration
	Location     *time.Location
}

// Config は実行時に scheduler が参照する全設定です．
type Config struct {
	System    System
	Reminders []reminder.Reminder
}

// --- TOML 直接 decode 用の raw 型 (パッケージ外には公開しない) ---

type rawSystem struct {
	LogLevel        string `toml:"log_level"`
	TickIntervalSec int    `toml:"tick_interval_sec"`
	Timezone        string `toml:"timezone"`
}

type rawReminder struct {
	Name    string   `toml:"name"`
	Time    string   `toml:"time"`
	Days    []string `toml:"days"`
	Message string   `toml:"message"`
	Webhook string   `toml:"webhook"`
}

type rawConfig struct {
	System    rawSystem     `toml:"system"`
	Reminders []rawReminder `toml:"reminders"`
}

type rawSecrets struct {
	Webhooks map[string]string `toml:"webhooks"`
}

// Load reads and validates configPath and secretsPath, returning a fully
// validated Config. 未知の曜日や存在しない webhook key 等は全てここで弾きます．
func Load(configPath, secretsPath string) (*Config, error) {
	var raw rawConfig
	if _, err := toml.DecodeFile(configPath, &raw); err != nil {
		return nil, fmt.Errorf("load config (%s): %w", configPath, err)
	}

	var sec rawSecrets
	if _, err := toml.DecodeFile(secretsPath, &sec); err != nil {
		return nil, fmt.Errorf("load secrets (%s): %w", secretsPath, err)
	}

	sys, err := buildSystem(raw.System)
	if err != nil {
		return nil, fmt.Errorf("system: %w", err)
	}

	reminders, err := buildReminders(raw.Reminders, sec.Webhooks)
	if err != nil {
		return nil, err
	}
	if len(reminders) == 0 {
		return nil, fmt.Errorf("at least one reminder is required, got 0")
	}

	return &Config{System: sys, Reminders: reminders}, nil
}

func buildSystem(r rawSystem) (System, error) {
	if r.TickIntervalSec <= 0 || r.TickIntervalSec > 60 {
		return System{}, fmt.Errorf("tick_interval_sec must be an integer in 1..60, got %d", r.TickIntervalSec)
	}
	if r.Timezone == "" {
		return System{}, fmt.Errorf("timezone is required")
	}
	loc, err := time.LoadLocation(r.Timezone)
	if err != nil {
		return System{}, fmt.Errorf("timezone %q: %w", r.Timezone, err)
	}
	level := r.LogLevel
	if level == "" {
		level = "info"
	}
	return System{
		LogLevel:     level,
		TickInterval: time.Duration(r.TickIntervalSec) * time.Second,
		Location:     loc,
	}, nil
}

func buildReminders(raws []rawReminder, webhooks map[string]string) ([]reminder.Reminder, error) {
	out := make([]reminder.Reminder, 0, len(raws))
	seen := make(map[string]struct{}, len(raws))

	for i, r := range raws {
		if r.Name == "" {
			return nil, fmt.Errorf("reminders[%d]: name is required", i)
		}
		if _, dup := seen[r.Name]; dup {
			return nil, fmt.Errorf("reminders[%d]: name %q is duplicated", i, r.Name)
		}
		seen[r.Name] = struct{}{}

		hour, minute, err := parseHHMM(r.Time)
		if err != nil {
			return nil, fmt.Errorf("reminder %q: time %q: %w", r.Name, r.Time, err)
		}

		days, err := parseDays(r.Days)
		if err != nil {
			return nil, fmt.Errorf("reminder %q: days: %w", r.Name, err)
		}

		webhookURL, ok := webhooks[r.Webhook]
		if !ok {
			return nil, fmt.Errorf("reminder %q: webhook key %q not found in secrets.toml", r.Name, r.Webhook)
		}
		if _, err := url.ParseRequestURI(webhookURL); err != nil {
			return nil, fmt.Errorf("reminder %q: invalid webhook URL: %w", r.Name, err)
		}

		if r.Message == "" {
			return nil, fmt.Errorf("reminder %q: message is empty", r.Name)
		}

		out = append(out, reminder.Reminder{
			Name:       r.Name,
			Hour:       hour,
			Minute:     minute,
			Days:       days,
			Message:    r.Message,
			WebhookURL: webhookURL,
		})
	}
	return out, nil
}

func parseHHMM(s string) (int, int, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("not in HH:MM format")
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return 0, 0, fmt.Errorf("invalid hour")
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return 0, 0, fmt.Errorf("invalid minute")
	}
	return h, m, nil
}

var dayMap = map[string]time.Weekday{
	"sun": time.Sunday,
	"mon": time.Monday,
	"tue": time.Tuesday,
	"wed": time.Wednesday,
	"thu": time.Thursday,
	"fri": time.Friday,
	"sat": time.Saturday,
}

var allWeekdays = []time.Weekday{
	time.Sunday, time.Monday, time.Tuesday, time.Wednesday,
	time.Thursday, time.Friday, time.Saturday,
}

func parseDays(days []string) ([]time.Weekday, error) {
	if len(days) == 0 {
		return nil, fmt.Errorf("days is empty")
	}
	// "every" が含まれていれば全曜日に展開
	for _, d := range days {
		if d == "every" {
			return allWeekdays, nil
		}
	}
	out := make([]time.Weekday, 0, len(days))
	for _, d := range days {
		wd, ok := dayMap[d]
		if !ok {
			return nil, fmt.Errorf("unknown weekday %q", d)
		}
		out = append(out, wd)
	}
	return out, nil
}
