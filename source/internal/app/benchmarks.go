package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"jiahao-toolbox/internal/benchmark"
	"jiahao-toolbox/internal/native"
)

func (a *App) historyPath() string { return filepath.Join(a.DataDir, "benchmarks-v4.3.json") }
func (a *App) loadTestHistory() {
	b, e := os.ReadFile(a.historyPath())
	if e != nil {
		return
	}
	var h []TestHistory
	if json.Unmarshal(b, &h) == nil && len(h) <= 30 {
		a.testHistory = h
	}
}
func (a *App) saveTestHistoryLocked() {
	b, e := json.MarshalIndent(a.testHistory, "", "  ")
	if e != nil {
		return
	}
	tmp, e := os.CreateTemp(a.DataDir, ".benchmarks-*.tmp")
	if e != nil {
		return
	}
	name := tmp.Name()
	if _, e = tmp.Write(b); e == nil {
		e = tmp.Sync()
	}
	ce := tmp.Close()
	if e == nil {
		e = ce
	}
	if e == nil {
		e = os.Rename(name, a.historyPath())
	}
	if e != nil {
		_ = os.Remove(name)
	}
}
func (a *App) testSnapshot() TestState {
	a.testMu.RLock()
	defer a.testMu.RUnlock()
	return a.testState
}
func (a *App) historySnapshot() []TestHistory {
	a.testMu.RLock()
	defer a.testMu.RUnlock()
	return append([]TestHistory(nil), a.testHistory...)
}
func (a *App) startTest(kind string, run func(context.Context, func(string, float64, float64, float64)) (any, error)) error {
	if a.Ctx.Err() != nil {
		return errors.New("程序正在退出")
	}
	a.testMu.Lock()
	defer a.testMu.Unlock()
	if a.testState.Status == "running" || a.testState.Status == "canceling" {
		return errors.New("另一个测速任务正在运行")
	}
	var idBytes [12]byte
	if _, e := rand.Read(idBytes[:]); e != nil {
		return e
	}
	id := hex.EncodeToString(idBytes[:])
	ctx, cancel := context.WithCancel(a.Ctx)
	a.testCancel = cancel
	a.testState = TestState{ID: id, Kind: kind, Status: "running", Stage: "准备中", Started: time.Now()}
	a.testWG.Add(1)
	go func() {
		defer a.testWG.Done()
		update := func(stage string, progress, currentMbps, currentMBps float64) {
			a.testMu.Lock()
			defer a.testMu.Unlock()
			if a.testState.ID == id && a.testState.Status == "running" {
				a.testState.Stage = stage
				a.testState.Progress = progress
				a.testState.CurrentMbps = currentMbps
				a.testState.CurrentMBps = currentMBps
			}
		}
		result, err := run(ctx, update)
		a.testMu.Lock()
		defer a.testMu.Unlock()
		if a.testState.ID != id {
			return
		}
		a.testState.Finished = time.Now()
		a.testCancel = nil
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
				a.testState.Status = "canceled"
				a.testState.Stage = "已取消"
			} else {
				a.testState.Status = "error"
				a.testState.Error = err.Error()
				a.testState.Stage = "测试失败"
			}
			return
		}
		a.testState.Status = "complete"
		a.testState.Stage = "已完成"
		a.testState.Progress = 1
		a.testState.Result = result
		a.testHistory = append([]TestHistory{{ID: id, Kind: kind, Time: a.testState.Finished, Result: result}}, a.testHistory...)
		if len(a.testHistory) > 30 {
			a.testHistory = a.testHistory[:30]
		}
		a.saveTestHistoryLocked()
		a.Log.Printf("benchmark=%s completed", kind)
	}()
	return nil
}
func (a *App) cancelTest() bool {
	a.testMu.Lock()
	defer a.testMu.Unlock()
	if a.testCancel == nil {
		return false
	}
	a.testState.Status = "canceling"
	a.testState.Stage = "正在取消并清理临时数据"
	a.testCancel()
	return true
}
func selectedDirectory(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" || !filepath.IsAbs(raw) {
		return "", errors.New("请使用文件夹选择器选择本机目录")
	}
	abs, e := filepath.Abs(raw)
	if e != nil {
		return "", e
	}
	st, e := os.Stat(abs)
	if e != nil {
		return "", fmt.Errorf("所选目录不可用：%w", e)
	}
	if !st.IsDir() {
		return "", errors.New("所选路径不是文件夹")
	}
	return filepath.Clean(abs), nil
}
func selectedVolume(a *App, id, dir string) (uint64, error) {
	if !strings.EqualFold(filepath.VolumeName(dir), strings.TrimSuffix(id, "\\")) {
		return 0, errors.New("所选目录与目标分区不一致，请重新选择目录")
	}
	state := a.Monitor.State(false)
	if time.Since(time.UnixMilli(state.Latest.Time)) > 30*time.Second {
		return 0, errors.New("分区可用空间数据已过期，请等待监控更新")
	}
	for _, d := range state.Latest.Disks {
		if strings.EqualFold(d.ID, id) {
			return d.Free, nil
		}
	}
	return 0, errors.New("目标分区未出现在当前 Windows 固定磁盘清单中")
}
func (a *App) handleBenchmarkAPI(w http.ResponseWriter, rPath string, rctx context.Context, decodeFn func(any) error, write func(any), fail func(int, error)) bool {
	switch rPath {
	case "tests/status":
		write(a.testSnapshot())
		return true
	case "tests/history":
		write(a.historySnapshot())
		return true
	case "tests/cancel":
		var p struct{}
		if e := decodeFn(&p); e != nil {
			fail(400, e)
			return true
		}
		write(map[string]bool{"cancelRequested": a.cancelTest()})
		return true
	case "speed/start":
		var p struct {
			Confirmed bool `json:"confirmed"`
		}
		if e := decodeFn(&p); e != nil {
			fail(400, e)
			return true
		}
		if !p.Confirmed {
			fail(400, errors.New("联网测速需要明确确认"))
			return true
		}
		e := a.startTest("speed", func(ctx context.Context, progress func(string, float64, float64, float64)) (any, error) {
			return benchmark.RunSpeedTest(ctx, benchmark.SpeedServer, func(p benchmark.SpeedProgress) { progress(p.Stage, p.Progress, p.CurrentMbps, 0) })
		})
		if e != nil {
			fail(409, e)
			return true
		}
		write(map[string]bool{"started": true})
		return true
	case "disk/pick-folder", "cmd/pick-folder":
		var p struct{}
		if e := decodeFn(&p); e != nil {
			fail(400, e)
			return true
		}
		dir, e := native.SelectFolder(rctx)
		if e != nil {
			fail(400, e)
			return true
		}
		write(map[string]string{"directory": dir})
		return true
	case "disk/start":
		var p struct {
			benchmark.DiskOptions
			VolumeID         string `json:"volumeId"`
			Confirmed        bool   `json:"confirmed"`
			RecoverConfirmed bool   `json:"recoverConfirmed"`
		}
		if e := decodeFn(&p); e != nil {
			fail(400, e)
			return true
		}
		dir, e := selectedDirectory(p.Directory)
		if e != nil {
			fail(400, e)
			return true
		}
		p.Directory = dir
		free, e := selectedVolume(a, p.VolumeID, dir)
		if e != nil {
			fail(400, e)
			return true
		}
		p.AvailableBytes = free
		if !p.Confirmed {
			fail(400, errors.New("磁盘测速需要确认临时写入及 SSD 写入损耗提示"))
			return true
		}
		if p.RecoverConfirmed {
			_, e = benchmark.RecoverOrphans(dir)
			if e != nil {
				fail(400, e)
				return true
			}
		}
		if _, e = benchmark.NormalizeDiskOptions(p.DiskOptions); e != nil {
			fail(400, e)
			return true
		}
		e = a.startTest("disk", func(ctx context.Context, progress func(string, float64, float64, float64)) (any, error) {
			return benchmark.RunDiskTest(ctx, p.DiskOptions, func(v benchmark.DiskProgress) { progress(v.Stage, v.Progress, 0, v.CurrentMBps) })
		})
		if e != nil {
			fail(409, e)
			return true
		}
		write(map[string]bool{"started": true})
		return true
	case "disk/recover":
		var p struct {
			Directory string `json:"directory"`
			VolumeID  string `json:"volumeId"`
			Confirmed bool   `json:"confirmed"`
		}
		if e := decodeFn(&p); e != nil {
			fail(400, e)
			return true
		}
		if !p.Confirmed {
			fail(400, errors.New("清理前请确认所选目录"))
			return true
		}
		dir, e := selectedDirectory(p.Directory)
		if e != nil {
			fail(400, e)
			return true
		}
		if _, e = selectedVolume(a, p.VolumeID, dir); e != nil {
			fail(400, e)
			return true
		}
		n, e := benchmark.RecoverOrphans(dir)
		if e != nil {
			fail(400, e)
			return true
		}
		write(map[string]int{"removed": n})
		return true
	case "cmd/open", "cmd/scan":
		var e error
		var p struct {
			Directory string `json:"directory"`
			Confirmed bool   `json:"confirmed"`
		}
		if e := decodeFn(&p); e != nil {
			fail(400, e)
			return true
		}
		dir := p.Directory
		if rPath == "cmd/open" && dir == "" {
			dir, e = os.UserHomeDir()
			if e != nil {
				fail(500, e)
				return true
			}
		}
		dir, e = selectedDirectory(dir)
		if e != nil {
			fail(400, e)
			return true
		}
		if rPath == "cmd/scan" && !p.Confirmed {
			fail(400, errors.New("经典目录扫描需要确认所选目录"))
			return true
		}
		if e = native.LaunchClassicCMD(dir, rPath == "cmd/scan"); e != nil {
			fail(500, e)
			return true
		}
		write(map[string]string{"message": func() string {
			if rPath == "cmd/scan" {
				return "已在独立绿色 CMD 窗口启动所选目录的 dir /s；关闭窗口可结束扫描"
			}
			return "已打开经典绿色 CMD 窗口"
		}()})
		return true
	}
	return false
}
