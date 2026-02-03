package main

import (
	"sync"
	"time"
)

// WindowsCollector 结构体
type WindowsCollector struct {
	mu       sync.RWMutex
	stopChan chan struct{}

	// 缓存数据
	metrics Metric
}

func NewCollector() Collector {
	return &WindowsCollector{
		stopChan: make(chan struct{}),
	}
}

func (wc *WindowsCollector) Start(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				wc.collect()
			case <-wc.stopChan:
				return
			}
		}
	}()
}

func (wc *WindowsCollector) Stop() {
	close(wc.stopChan)
}

func (wc *WindowsCollector) GetMetrics() (*Metric, error) {
	wc.mu.RLock()
	defer wc.mu.RUnlock()
	// 返回副本
	m := wc.metrics
	return &m, nil
}

func (wc *WindowsCollector) collect() {
	wc.mu.Lock()
	defer wc.mu.Unlock()

	// TODO: 实现 Windows 下的指标采集
	// 可以使用 syscall 调用 kernel32.dll 的 GetSystemTimes, GlobalMemoryStatusEx 等
	// 或者使用 golang.org/x/sys/windows 包
	// 这里目前仅作占位，防止编译报错

	wc.metrics.CPUUsagePercent = 0.0 // 待实现
	wc.metrics.MemUsagePercent = 0.0 // 待实现
	wc.metrics.DiskFreeGB = 0.0      // 待实现
	wc.metrics.Uptime = 0.0          // 待实现
}
