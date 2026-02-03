# SysAgent - 轻量级跨平台系统监控探针

这是一个使用 Go 语言编写的轻量级系统监控 Agent，目前支持 **Linux** 和 **Windows** (基础框架)。它能够实时采集系统的各项核心指标（CPU、内存、磁盘、网络等），并通过内置的 HTTP 服务器展示在一个美观的 Web 仪表盘上。

项目旨在学习 Go 语言的**并发模型**、**系统编程**以及**Web 服务开发**。

---

## 🚀 快速开始

### 1. 运行
直接运行 Go 程序：
```bash
go run .
# 或者指定端口和刷新间隔
go run . -port :9090 -interval 2s
```

### 2. 访问
打开浏览器访问：`http://localhost:8085` (默认端口)

### 3. 构建
将 HTML 资源打包进二进制文件（得益于 `go:embed`）：
```bash
go build -o sysagent
./sysagent
```

---

## 📂 项目结构

```text
.
├── main.go               # 程序入口，HTTP 服务与路由配置
├── collector.go          # 定义通用的 Collector 接口与 Metric 结构体
├── collector_linux.go    # Linux 平台具体实现 (读取 /proc)
├── collector_windows.go  # Windows 平台具体实现 (骨架)
├── dashboard.html        # 前端界面 (Vue3 + ECharts)
└── go.mod                # 依赖管理
```

---

## 💡 构建思路与学习笔记

### 1. 架构设计：生产者-消费者模型
为了不让 HTTP 请求阻塞采集过程，也不让采集过程因为计算耗时影响接口响应，项目采用了**存算分离**的设计思路。

-   **采集器 (Producer)**: 后台启动一个 Goroutine，利用 `time.Ticker` 每秒采集一次 `/proc` 文件系统的数据，计算由于时间差产生的速率指标（如 CPU 使用率、网速），并将结果更新到内存结构体中。
-   **HTTP 服务 (Consumer)**: 当用户访问 `/metrics` 接口时，Handler 直接读取内存中最新的指标数据返回，响应速度极快。
-   **并发安全**: 由于采集器在该写数据，HTTP 在读数据，必须使用 `sync.RWMutex` 读写锁来保证数据竞争安全。

### 2. Go 核心知识点应用

#### A. `go:embed` 静态资源嵌入
在 `main.go` 中使用了 `//go:embed dashboard.html` 指令。
-   **作用**: 将 HTML 文件在编译时打包进二进制文件。
-   **好处**: 部署时只需要拷贝一个可执行文件，不需要携带静态资源文件夹，极大简化了分发流程。

#### B. 系统编程 (Collector)
学习了如何不通过第三方库，直接通过读取 Linux 的「伪文件系统」`/proc` 来获取内核数据：
-   **CPU 使用率**: 读取 `/proc/stat`。通过计算 `Total CPU Time` 和 `Idle Time` 在两个时间点之间的差值来算出使用率。
    -   公式：`Usage = 1 - (idleDelta / totalDelta)`
-   **内存/Swap**: 读取 `/proc/meminfo`。
-   **负载 (Load Average)**: 读取 `/proc/loadavg`。
-   **网络速率**: 读取 `/proc/net/dev`。通过计算 `Receive/Transmit Bytes` 的差值除以时间间隔 (`interval`) 得到速率。

#### C. Canvas 图表与前端交互
-   前端使用 **Vue 3** 进行数据绑定，极大简化了 DOM 操作。
-   使用 **ECharts** 绘制实时 CPU/内存 波形图。
-   实现了前端轮询机制，前接口抽象与跨平台)

项目定义了一个通用的 `Collector` 接口，利用 Go 语言的 **Build Tags** (文件名后缀 `_linux.go`, `_windows.go`) 来实现跨平台编译。

```go
// collector.go
type Collector interface {
    GetMetrics() (*Metric, error)
    Start(interval time.Duration)
    Stop()
}
```

- **Linux**: 读取 `/proc` 文件系统。
- **Windows**: (已添加骨架) 预留了 Win32 API 调用位置，使用工厂模式自动适配。

在 `main.go` 中调用工厂方法 `NewCollector()`，编译器会自动选择对应的平台实现。 Stop()                         // 优雅停止
}
```

在 `Start` 方法中，巧妙地使用了 `select` 配合 `time.Ticker`，这是 Go 语言处理定时任务的标准范式：

```go
go func() {
    for {
        select {
        case <-ticker.C:
            // 执行采集逻辑...
        case <-lc.stopChan:
            // 收到停止信号，退出循环
            return
        }
    }
}()
```

---

## 📝 TODO 改进计划
- [ ] 增加历史数据存储 (简单的内存时序数组)，让图表展示更长时间跨度。
- [ ] 支持 WebSocket 推送模式，替代前端轮询，降低网络开销。
- [ ] 添加简单的身份验证 (Basic Auth)。
