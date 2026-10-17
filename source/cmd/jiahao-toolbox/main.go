package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"jiahao-toolbox/internal/app"
	"jiahao-toolbox/internal/native"
	"jiahao-toolbox/internal/systemtools"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"time"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		if runtime.GOOS == "windows" {
			native.Alert(e.Error() + "\n\n可使用随附的启动兼容模式.cmd，通过系统浏览器运行同一套真实监控后端。\nWebView2 官方运行组件说明见 README.md。")
		}
		os.Exit(1)
	}
}
func run() error {
	serve := flag.Bool("serve", false, "仅启动本地服务，用于开发验证")
	browser := flag.Bool("browser", false, "使用系统浏览器（兼容模式）")
	data := flag.String("data-dir", "", "指定本程序配置目录")
	reportPath := flag.String("report-file", "", "自检结果 JSON 文件；只与 --self-check 一起使用，不覆盖已有文件")
	duration := flag.Duration("self-check", 0, "无界面采样并在指定时间后退出，如 10s；只读")
	flag.Parse()
	if *reportPath != "" && *duration <= 0 {
		return errors.New("--report-file 需要 --self-check")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	a, e := app.New(ctx, *data)
	if e != nil {
		return e
	}
	defer a.Close()
	if *duration > 0 {
		fmt.Println(a.Description())
		start := time.Now()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		goroutinesBefore := runtime.NumGoroutine()
		timer := time.NewTimer(*duration)
		defer timer.Stop()
		select {
		case <-ctx.Done():
		case <-timer.C:
		}
		s := a.Monitor.State(false)
		runtime.ReadMemStats(&after)
		if *reportPath != "" {
			result := map[string]any{"version": app.Version, "buildTime": app.BuildTime, "goVersion": runtime.Version(), "platform": runtime.GOOS, "architecture": runtime.GOARCH, "scope": "read-only backend self-check; excludes WebView2 and child processes", "started": start.UTC(), "elapsedSeconds": time.Since(start).Seconds(), "samples": s.Samples, "paused": s.Paused, "lastSampleTime": s.Latest.Time, "lastSampleDurationMS": s.Latest.DurationMS, "latest": s.Latest, "goHeapStart": before.HeapAlloc, "goHeapEnd": after.HeapAlloc, "goGoroutinesStart": goroutinesBefore, "goGoroutinesEnd": runtime.NumGoroutine(), "hardwareStatus": a.Hardware.Get().Status}
			path := filepath.Clean(*reportPath)
			f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if e != nil {
				return e
			}
			enc := json.NewEncoder(f)
			enc.SetIndent("", "  ")
			e = enc.Encode(result)
			ce := f.Close()
			if e != nil {
				return e
			}
			if ce != nil {
				return ce
			}
		}
		fmt.Printf("samples=%d sampleDurationMS=%.3f memoryBytes=%d errors=%v\n", s.Samples, s.Latest.DurationMS, s.Latest.Memory.Total, s.Latest.Errors)
		return nil
	}
	if *serve || *browser {
		if *browser {
			a.SetBrowser()
			if e := systemtools.OpenBrowser(a.URL); e != nil {
				return e
			}
		}
		fmt.Println(a.URL)
		fmt.Println("本地会话地址仅供本机使用。按 Ctrl+C 退出；兼容模式也可点击应用内退出按钮。")
		<-a.Ctx.Done()
		return nil
	}
	w := native.New(native.Options{Version: app.Version, URL: a.URL, Origin: a.Origin, DataDir: a.DataDir, Config: a.Config, Log: a.Log, OnBackground: a.SetBackground})
	a.Window = w
	return w.Run(a.Ctx)
}
