package main

import "time"

// Metric 定义最终返回的 JSON 结构
type Metric struct {
	// --- 核心资源 ---
	CPUUsagePercent  float64 `json:"cpu_usage_percent"`
	MemUsagePercent  float64 `json:"mem_usage_percent"`
	SwapUsagePercent float64 `json:"swap_usage_percent"` // [新增] Swap 使用率
	DiskFreeGB       float64 `json:"disk_free_gb"`

	// --- 系统状态 ---
	Load1  float64 `json:"load_1"`
	Load5  float64 `json:"load_5"`
	Load15 float64 `json:"load_15"`
	Uptime float64 `json:"uptime_hours"`

	// [新增] 文件句柄
	FDOpen uint64 `json:"fd_open"` // 当前打开的文件句柄数
	FDMax  uint64 `json:"fd_max"`  // 系统允许的最大句柄数

	// --- 传感器与 IO ---
	CPUTempCelsius float64 `json:"cpu_temp_c"`
	// [新增] 电池信息 (仅笔记本有效)
	BatteryPercent int    `json:"battery_percent"` // 电量百分比
	BatteryStatus  string `json:"battery_status"`  // 状态: Charging, Discharging, Full

	NetRxRateKB float64 `json:"net_rx_kb"`
	NetTxRateKB float64 `json:"net_tx_kb"`
}

// Collector 接口
type Collector interface {
	GetMetrics() (*Metric, error) // get方法。上层调用get来获取数据，屏蔽底层实现细节
	Start(interval time.Duration) // 启动采集器，为了计算速率等需要定时采样，不是说直接读取就好
	Stop()                        // 停止采集器，防止泄漏goroutine
}
