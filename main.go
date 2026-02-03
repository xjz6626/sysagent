package main

import (
	"embed"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"time"
)

//以//go开头的是给编译器的指令，告诉它把dashboard.html文件嵌入到可执行文件中

//go:embed dashboard.html
var content embed.FS

func main() {
	//参数解析，有默认参数
	port := flag.String("port", ":8085", "HTTP server port")
	interval := flag.Duration("interval", 1*time.Second, "Sampling interval")
	flag.Parse()

	collector := NewLinuxCollector() //初始化采集器,自定义的
	//启动采集器
	collector.Start(*interval)
	defer collector.Stop()

	// 从内存中拿出html文件，设置路由
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 读取嵌入的文件系统中的 dashboard.html
		data, err := content.ReadFile("dashboard.html")
		if err != nil {
			http.Error(w, "Dashboard not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write(data)
	})
	// 设置 /metrics 路由，返回 JSON 格式的系统指标
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		metrics, err := collector.GetMetrics()
		if err != nil {
			log.Printf("Error: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(metrics)
	})
	// 启动 HTTP 服务器，打印日志
	log.Printf("SysAgent Dashboard available at http://localhost%s", *port)
	if err := http.ListenAndServe(*port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
