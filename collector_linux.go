package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// LinuxCollector 结构体保持不变
type LinuxCollector struct {
	mu       sync.RWMutex  //读写锁
	stopChan chan struct{} //停止信号通道

	// 缓存的采样数据

	cpuUsage    float64 // CPU 使用率
	netRxRateKB float64 // 网络接收速率 KB/s
	netTxRateKB float64 // 网络发送速率 KB/s
}

func NewCollector() Collector {
	return &LinuxCollector{stopChan: make(chan struct{})}
}

// Start 方法保持不变  初始化 -> 定时循环 -> 计算与更新
func (lc *LinuxCollector) Start(interval time.Duration) {
	//预热，记录初值
	prevIdle, prevTotal, _ := readCPUSample()
	prevRx, prevTx, _ := readNetSample()
	//启动定时采样goroutine
	ticker := time.NewTicker(interval)
	//开启后台采样协程
	go func() {
		defer ticker.Stop() // 停止ticker
		//死循环直到收到停止信号
		for {
			select {
			case <-ticker.C: //每次定时到达
				//cpu采样
				idle, total, errCPU := readCPUSample()
				var cpuUsage float64
				if errCPU == nil {
					deltaIdle := float64(idle - prevIdle)
					deltaTotal := float64(total - prevTotal)
					if deltaTotal > 0 { //避免除0
						cpuUsage = (1.0 - (deltaIdle / deltaTotal)) * 100.0
					}
					prevIdle, prevTotal = idle, total //更新前值
				}
				//网络采样
				currRx, currTx, errNet := readNetSample()
				var rxRate, txRate float64
				if errNet == nil {
					rxRate = float64(currRx-prevRx) / interval.Seconds() / 1024
					txRate = float64(currTx-prevTx) / interval.Seconds() / 1024
					prevRx, prevTx = currRx, currTx
				}

				//更新缓存数据
				lc.mu.Lock() //写锁
				if errCPU == nil {
					lc.cpuUsage = cpuUsage
				}
				if errNet == nil {
					lc.netRxRateKB = rxRate
					lc.netTxRateKB = txRate
				}
				lc.mu.Unlock() //解锁
			case <-lc.stopChan: //收到停止信号，退出goroutine
				return
			}
		}
	}()
}

// 停止采集器，关闭通道
func (lc *LinuxCollector) Stop() { close(lc.stopChan) }

// GetMetrics 组装所有指标
func (lc *LinuxCollector) GetMetrics() (*Metric, error) {
	m := &Metric{}

	// 1. 缓存数据
	lc.mu.RLock()
	m.CPUUsagePercent = lc.cpuUsage
	m.NetRxRateKB = lc.netRxRateKB
	m.NetTxRateKB = lc.netTxRateKB
	lc.mu.RUnlock()

	// 2. 内存 & Swap [修改]
	memUsage, swapUsage, err := getMemAndSwapUsage()
	if err == nil {
		m.MemUsagePercent = memUsage
		m.SwapUsagePercent = swapUsage
	}

	// 3. 磁盘
	if val, err := getDiskFree("/"); err == nil {
		m.DiskFreeGB = val
	}

	// 4. 负载
	if l1, l5, l15, err := getLoadAvg(); err == nil {
		m.Load1, m.Load5, m.Load15 = l1, l5, l15
	}

	// 5. Uptime
	if val, err := getUptime(); err == nil {
		m.Uptime = val / 3600.0
	}

	// 6. 温度
	if val, err := getCPUTemp(); err == nil {
		m.CPUTempCelsius = val
	}

	// 7. [新增] 文件句柄
	if open, max, err := getFDStats(); err == nil {
		m.FDOpen = open
		m.FDMax = max
	}

	// 8. [新增] 电池
	if pct, status, err := getBatteryInfo(); err == nil {
		m.BatteryPercent = pct
		m.BatteryStatus = status
	}

	return m, nil
}

// --- 以下是读取逻辑 ---

// readCPUSample (保持不变)
func readCPUSample() (uint64, uint64, error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 5 {
			return 0, 0, fmt.Errorf("bad stat")
		}
		var total, idle uint64
		for i, field := range fields[1:] {
			val, _ := strconv.ParseUint(field, 10, 64)
			total += val
			if i == 3 || i == 4 {
				idle += val
			}
		}
		return idle, total, nil
	}
	return 0, 0, fmt.Errorf("empty stat")
}

// readNetSample (保持不变)
func readNetSample() (uint64, uint64, error) {
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()
	var totalRx, totalTx uint64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "|") || strings.Contains(line, "lo:") {
			continue
		}
		fields := strings.Fields(strings.ReplaceAll(line, ":", " "))
		if len(fields) < 10 {
			continue
		}
		rx, _ := strconv.ParseUint(fields[1], 10, 64)
		tx, _ := strconv.ParseUint(fields[9], 10, 64)
		totalRx += rx
		totalTx += tx
	}
	return totalRx, totalTx, nil
}

// [修改] getMemAndSwapUsage 同时解析内存和Swap
func getMemAndSwapUsage() (float64, float64, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	var memTotal, memAvailable, swapTotal, swapFree float64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		val, _ := strconv.ParseFloat(parts[1], 64)

		switch parts[0] {
		case "MemTotal:":
			memTotal = val
		case "MemAvailable:":
			memAvailable = val
		case "SwapTotal:":
			swapTotal = val
		case "SwapFree:":
			swapFree = val
		}
	}

	var memUsage, swapUsage float64
	if memTotal > 0 {
		memUsage = (memTotal - memAvailable) / memTotal * 100.0
	}
	if swapTotal > 0 {
		swapUsage = (swapTotal - swapFree) / swapTotal * 100.0
	}
	// 如果 swapTotal 为 0 (未启用 Swap)，swapUsage 保持 0 即可

	return memUsage, swapUsage, nil
}

// getDiskFree (保持不变)
func getDiskFree(path string) (float64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	return float64(stat.Bavail*uint64(stat.Bsize)) / 1024 / 1024 / 1024, nil
}

// getLoadAvg (保持不变)
func getLoadAvg() (float64, float64, float64, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, err
	}
	parts := strings.Fields(string(data))
	l1, _ := strconv.ParseFloat(parts[0], 64)
	l5, _ := strconv.ParseFloat(parts[1], 64)
	l15, _ := strconv.ParseFloat(parts[2], 64)
	return l1, l5, l15, nil
}

// getUptime (保持不变)
func getUptime() (float64, error) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}
	parts := strings.Fields(string(data))
	return strconv.ParseFloat(parts[0], 64)
}

// getCPUTemp (保持不变)
func getCPUTemp() (float64, error) {
	data, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp")
	if err != nil {
		return 0, err
	}
	raw, _ := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
	return raw / 1000.0, nil
}

// [新增] getFDStats 读取文件句柄
func getFDStats() (uint64, uint64, error) {
	// 格式: 1280    0       131072
	//      (已分配) (未使用) (最大值)
	data, err := os.ReadFile("/proc/sys/fs/file-nr")
	if err != nil {
		return 0, 0, err
	}
	parts := strings.Fields(string(data))
	if len(parts) < 3 {
		return 0, 0, fmt.Errorf("bad file-nr")
	}

	alloc, _ := strconv.ParseUint(parts[0], 10, 64)
	// free, _ := strconv.ParseUint(parts[1], 10, 64) // Linux 2.6+ 这里的 free 总是 0，可以忽略
	max, _ := strconv.ParseUint(parts[2], 10, 64)

	// 注意：实际打开数 = 已分配 - 未使用。
	// 但在现代 Linux 内核中，parts[0] 通常就是实际使用的近似值。
	return alloc, max, nil
}

// [新增] getBatteryInfo 读取电池信息
func getBatteryInfo() (int, string, error) {
	// 通常是 BAT0，部分设备可能是 BAT1
	basePath := "/sys/class/power_supply/BAT0"
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		// 如果没有电池（台式机），返回默认值
		return 100, "AC_Power", nil
	}

	// 读电量
	capData, err := os.ReadFile(basePath + "/capacity")
	if err != nil {
		return 0, "", err
	}
	pct, _ := strconv.Atoi(strings.TrimSpace(string(capData)))

	// 读状态
	statData, err := os.ReadFile(basePath + "/status")
	if err != nil {
		return pct, "Unknown", nil
	}
	status := strings.TrimSpace(string(statData))

	return pct, status, nil
}
