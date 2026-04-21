/**
 * 全局请求统计（RPM 等）
 * 用于统计页面展示每分钟请求数
 */
package handler

import (
	"sync/atomic"
	"time"
)

var globalRequestStats requestStats

type requestStats struct {
	lastMinuteStart atomic.Int64
	lastMinuteCount atomic.Int64
}

// RecordRequest 在每次请求成功完成时调用，用于计算 RPM
func RecordRequest() {
	now := time.Now()
	minStart := now.Truncate(time.Minute).Unix()
	prev := globalRequestStats.lastMinuteStart.Load()
	if prev != minStart {
		if globalRequestStats.lastMinuteStart.CompareAndSwap(prev, minStart) {
			globalRequestStats.lastMinuteCount.Store(0)
		}
	}
	globalRequestStats.lastMinuteCount.Add(1)
}

// GetRPM 返回最近一分钟内的请求数（RPM）
func GetRPM() int64 {
	now := time.Now()
	minStart := now.Truncate(time.Minute).Unix()
	if globalRequestStats.lastMinuteStart.Load() != minStart {
		return 0
	}
	return globalRequestStats.lastMinuteCount.Load()
}
