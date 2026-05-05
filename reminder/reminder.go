// Package reminder defines the Reminder domain type and matching logic.
package reminder

import "time"

// Reminder は config から構築された，検証済みの単一リマインダー定義です．
type Reminder struct {
	Name       string
	Hour       int
	Minute     int
	Days       []time.Weekday // every の場合は 7 曜日全てが入る
	Message    string
	WebhookURL string
}

// ShouldFire は now が当該リマインダーの発火タイミングと一致するかを返します．
// 分粒度で判定し，秒以下は無視します．
func (r Reminder) ShouldFire(now time.Time) bool {
	if now.Hour() != r.Hour || now.Minute() != r.Minute {
		return false
	}
	wd := now.Weekday()
	for _, d := range r.Days {
		if d == wd {
			return true
		}
	}
	return false
}
