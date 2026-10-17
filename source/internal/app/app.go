package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"jiahao-toolbox/frontend"
	"jiahao-toolbox/internal/config"
	"jiahao-toolbox/internal/diagnostics"
	"jiahao-toolbox/internal/hardware"
	"jiahao-toolbox/internal/logging"
	"jiahao-toolbox/internal/monitor"
	"jiahao-toolbox/internal/native"
	"jiahao-toolbox/internal/systemtools"
	updater "jiahao-toolbox/internal/update"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var Version = "4.4.0"
var BuildTime = "development"

type App struct {
	Ctx                               context.Context
	Cancel                            context.CancelFunc
	Config                            *config.Store
	Monitor                           *monitor.Service
	Hardware                          *hardware.Cache
	Log                               *log.Logger
	LogFile                           *logging.Writer
	Window                            native.Window
	DataDir, URL, Origin, host, token string
	profileLock                       *os.File
	cookieName                        string
	server                            *http.Server
	listener                          net.Listener
	lastSeen                          atomic.Int64
	started                           time.Time
	mu                                sync.Mutex
	report                            *diagnostics.Report
	action                            sync.Mutex
	browser                           bool
	configWarning                     string
	testMu                            sync.RWMutex
	testCancel                        context.CancelFunc
	testWG                            sync.WaitGroup
	testState                         TestState
	testHistory                       []TestHistory
	updateMu                          sync.RWMutex
	updateState                       updater.Info
	updateClient                      *updater.Client
}

type TestState struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	Status      string    `json:"status"`
	Stage       string    `json:"stage"`
	Progress    float64   `json:"progress"`
	CurrentMbps float64   `json:"currentMbps"`
	CurrentMBps float64   `json:"currentMBps"`
	Result      any       `json:"result,omitempty"`
	Error       string    `json:"error,omitempty"`
	Started     time.Time `json:"started"`
	Finished    time.Time `json:"finished,omitempty"`
}
type TestHistory struct {
	ID     string    `json:"id"`
	Kind   string    `json:"kind"`
	Time   time.Time `json:"time"`
	Result any       `json:"result"`
}

func New(parent context.Context, dataDir string) (*App, error) {
	if dataDir == "" {
		exe, e := os.Executable()
		if e != nil {
			return nil, e
		}
		dataDir = filepath.Join(filepath.Dir(exe), "JiahaoToolbox_Data")
	}
	if e := os.MkdirAll(dataDir, 0700); e != nil {
		fallback, e2 := os.UserConfigDir()
		if e2 != nil {
			return nil, e
		}
		dataDir = filepath.Join(fallback, "JiahaoToolbox")
		if e = os.MkdirAll(dataDir, 0700); e != nil {
			return nil, e
		}
	}
	// Probe write permission before committing to a portable profile directory.
	probe, e := os.CreateTemp(dataDir, ".write-test-")
	if e != nil {
		base, _ := os.UserConfigDir()
		dataDir = filepath.Join(base, "JiahaoToolbox")
		if e = os.MkdirAll(dataDir, 0700); e != nil {
			return nil, e
		}
	} else {
		name := probe.Name()
		probe.Close()
		os.Remove(name)
	}
	lock, e := config.LockProfile(dataDir)
	if e != nil {
		return nil, e
	}
	lf, e := logging.Open(filepath.Join(dataDir, "logs"))
	if e != nil {
		lock.Close()
		return nil, e
	}
	cfg, ce := config.Open(dataDir)
	ctx, cancel := context.WithCancel(parent)
	a := &App{profileLock: lock, Ctx: ctx, Cancel: cancel, DataDir: dataDir, Config: cfg, LogFile: lf, Log: logging.New(lf), Hardware: hardware.NewCache(), started: time.Now(), updateState: updater.Info{CurrentVersion: Version, ReleasePage: updater.ReleasesPage, Status: "not_checked"}, updateClient: updater.NewClient()}
	a.loadTestHistory()
	a.loadUpdateState()
	if ce != nil {
		a.configWarning = "配置读取失败，使用默认设置：" + ce.Error()
		a.Log.Print(a.configWarning)
	}
	c, e := monitor.NewCollector()
	if e != nil {
		lf.Close()
		lock.Close()
		cancel()
		return nil, e
	}
	a.Monitor = monitor.NewService(c, cfg.Get().IntervalMS)
	a.Monitor.Start(ctx)
	a.Hardware.Refresh(ctx)
	secret := make([]byte, 32)
	if _, e = rand.Read(secret); e != nil {
		a.Close()
		return nil, e
	}
	a.token = hex.EncodeToString(secret)
	ln, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		a.Close()
		return nil, e
	}
	a.listener = ln
	a.host = ln.Addr().String()
	a.cookieName = "jiahao_session_" + a.token[:12]
	a.Origin = "http://" + a.host
	a.URL = a.Origin + "/start/" + a.token
	a.lastSeen.Store(time.Now().UnixMilli())
	a.server = &http.Server{Handler: a, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 70 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10, BaseContext: func(net.Listener) context.Context { return ctx }}
	go func() {
		e := a.server.Serve(ln)
		if e != nil && !errors.Is(e, http.ErrServerClosed) {
			a.Log.Printf("HTTP service: %v", e)
			a.Cancel()
		}
	}()
	a.Log.Printf("app=%s build=%s platform=%s/%s backend_ready_ms=%d", Version, BuildTime, runtime.GOOS, runtime.GOARCH, time.Since(a.started).Milliseconds())
	return a, nil
}
func (a *App) Close() {
	a.Cancel()
	a.testMu.Lock()
	if a.testCancel != nil {
		a.testCancel()
	}
	a.testMu.Unlock()
	if a.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		a.server.Shutdown(ctx)
		cancel()
	}
	if a.Monitor != nil {
		a.Monitor.Wait()
	}
	a.testWG.Wait()
	a.Hardware.Wait()
	if a.LogFile != nil {
		a.Log.Print("application stopped; sampler and inventory joined")
		a.LogFile.Close()
	}
	if a.profileLock != nil {
		a.profileLock.Close()
	}
}
func (a *App) SetBrowser() {
	a.browser = true
	go func() {
		t := time.NewTicker(15 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-a.Ctx.Done():
				return
			case <-t.C:
				if time.Now().UnixMilli()-a.lastSeen.Load() > 120000 {
					a.Log.Print("browser heartbeat expired; closing backend")
					a.Cancel()
					return
				}
			}
		}
	}()
}
func (a *App) SetBackground(b bool) {
	s := a.Monitor.State(false)
	a.Monitor.Configure(a.Config.Get().IntervalMS, s.Paused, b)
}
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if e := recover(); e != nil {
			a.Log.Printf("HTTP panic: %v", e)
			http.Error(w, "内部错误，详情已写入本地日志", 500)
		}
	}()
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self'; connect-src 'self'; frame-src 'none'; frame-ancestors 'none'; object-src 'none'; base-uri 'none'; form-action 'none'")
	if r.Host != a.host {
		http.Error(w, "无效主机", 403)
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" && origin != a.Origin {
		http.Error(w, "跨来源请求被拒绝", 403)
		return
	}
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		http.Error(w, "跨站点请求被拒绝", 403)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/start/") && r.Method == "GET" {
		candidate := strings.TrimPrefix(r.URL.Path, "/start/")
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(a.token)) != 1 {
			http.Error(w, "会话无效", 403)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: a.cookieName, Value: a.token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	cookie, e := r.Cookie(a.cookieName)
	if e != nil || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(a.token)) != 1 {
		http.Error(w, "会话失效，请重新打开工具箱", 401)
		return
	}
	if r.Method != "GET" && r.Method != "POST" {
		http.Error(w, "不支持的方法", 405)
		return
	}
	if r.Method == "POST" && (r.Header.Get("Origin") != a.Origin || r.Header.Get("X-Jiahao-Request") != "1") {
		http.Error(w, "写请求校验失败", 403)
		return
	}
	a.lastSeen.Store(time.Now().UnixMilli())
	if strings.HasPrefix(r.URL.Path, "/api/") {
		a.api(w, r)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "只读资源", 405)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	if strings.Contains(path, "..") || strings.Contains(path, "\\") {
		http.NotFound(w, r)
		return
	}
	if path == "index.html" {
		content, err := fs.ReadFile(frontend.Files(), "index.html")
		if err != nil {
			http.Error(w, "界面 HTML 资源加载失败，请重新安装程序。", 500)
			return
		}
		settings := a.Config.Get()
		content = bytes.Replace(content, []byte(`data-theme="dark"`), []byte(`data-theme="`+native.StartupTheme(settings.Theme)+`"`), 1)
		if !settings.Animations || native.ReducedMotion() {
			content = bytes.Replace(content, []byte(`data-motion="on"`), []byte(`data-motion="off"`), 1)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(content)
		return
	}
	http.FileServer(http.FS(frontend.Files())).ServeHTTP(w, r)
}
func jsonOut(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}
func failure(w http.ResponseWriter, status int, e error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	jsonOut(w, map[string]any{"error": e.Error()})
}
func decode(w http.ResponseWriter, r *http.Request, out any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	defer r.Body.Close()
	b, e := io.ReadAll(r.Body)
	if e != nil {
		return e
	}
	b = bytes.TrimSpace(b)
	if len(b) < 2 || b[0] != '{' || b[len(b)-1] != '}' {
		return errors.New("请求必须是 JSON 对象")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if e := d.Decode(out); e != nil {
		return e
	}
	var extra any
	if e := d.Decode(&extra); e != io.EOF {
		return errors.New("请求仅可包含一个 JSON 对象")
	}
	return nil
}
func (a *App) api(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/")
	if r.Method == "GET" {
		if a.handleBenchmarkAPI(w, strings.TrimPrefix(r.URL.Path, "/api/"), r.Context(), func(any) error { return errors.New("GET 接口不接受请求体") }, func(v any) { jsonOut(w, v) }, func(code int, e error) { failure(w, code, e) }) {
			return
		}
		switch path {
		case "bootstrap":
			jsonOut(w, map[string]any{"version": Version, "buildTime": BuildTime, "goVersion": runtime.Version(), "platform": runtime.GOOS, "architecture": runtime.GOARCH, "settings": a.Config.Get(), "tools": systemtools.Tools, "hardware": hardware.Redact(a.Hardware.Get()), "state": a.Monitor.State(true), "startup": systemtools.StartupStatus(), "native": a.Window != nil, "dataDir": a.DataDir, "configWarning": a.configWarning, "limitations": []string{"Beta：Windows 原生窗口、工具入口和采样尚未通过实机验收", "构建工具链版本见本页；完整安全扫描与多机兼容性验收尚未完成", "温度与 SMART：无可靠接口时不提供；无内核驱动", "评分不代表硬件寿命、系统安全或补丁合规性"}})
		case "snapshot":
			jsonOut(w, a.Monitor.State(r.URL.Query().Get("history") == "1"))
		case "hardware":
			jsonOut(w, hardware.Redact(a.Hardware.Get()))
		case "report":
			a.mu.Lock()
			defer a.mu.Unlock()
			jsonOut(w, a.report)
		case "logs":
			b, e := a.LogFile.Read()
			if e != nil {
				failure(w, 500, e)
			} else {
				jsonOut(w, map[string]string{"text": string(b)})
			}
		case "updates/status":
			a.updateMu.RLock()
			jsonOut(w, a.updateState)
			a.updateMu.RUnlock()
		default:
			http.NotFound(w, r)
		}
		return
	}
	if a.handleBenchmarkAPI(w, path, r.Context(), func(out any) error { return decode(w, r, out) }, func(v any) { jsonOut(w, v) }, func(code int, e error) { failure(w, code, e) }) {
		return
	}
	// No-argument commands still require an empty JSON object, not arbitrary data.
	switch path {
	case "hardware/refresh", "hardware/cancel", "diagnose", "ui-ready", "exit":
		var p struct{}
		if e := decode(w, r, &p); e != nil {
			failure(w, 400, e)
			return
		}
	}
	switch path {
	case "settings":
		var s config.Settings
		if e := decode(w, r, &s); e != nil {
			failure(w, 400, e)
			return
		}
		if s.MonitorAnimation == "" {
			s.MonitorAnimation = a.Config.Get().MonitorAnimation
		}
		if e := a.Config.Update(s); e != nil {
			failure(w, 400, e)
			return
		}
		state := a.Monitor.State(false)
		a.Monitor.Configure(s.IntervalMS, state.Paused, state.Background)
		jsonOut(w, s)
	case "updates/check":
		var p struct{}
		if e := decode(w, r, &p); e != nil {
			failure(w, 400, e)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		settings := a.Config.Get()
		client := a.updateClient
		if client == nil { client = updater.NewClient() }
		info, e := client.Check(ctx, Version, settings.AllowBetaUpdates)
		if e != nil && info.Status == "check_failed" {
			info.Status = "network_error"
		}
		a.updateMu.Lock()
		a.updateState = info
		a.updateMu.Unlock()
		a.saveUpdateState(info)
		if e != nil {
			if errors.Is(e, updater.ErrNoRelease) {
				jsonOut(w, info)
				return
			}
			failure(w, 502, e)
			return
		}
		jsonOut(w, info)
	case "updates/open-release":
		var p struct {
			Confirmed bool `json:"confirmed"`
		}
		if e := decode(w, r, &p); e != nil {
			failure(w, 400, e)
			return
		}
		if !p.Confirmed {
			failure(w, 400, errors.New("打开官方 Release 页面需要确认"))
			return
		}
		if e := systemtools.OpenOfficialReleasePage(); e != nil {
			failure(w, 500, e)
			return
		}
		jsonOut(w, map[string]bool{"opened": true})
	case "monitor":
		var p struct {
			Paused     bool `json:"paused"`
			Background bool `json:"background"`
		}
		if e := decode(w, r, &p); e != nil {
			failure(w, 400, e)
			return
		}
		a.Monitor.Configure(a.Config.Get().IntervalMS, p.Paused, p.Background)
		jsonOut(w, map[string]bool{"ok": true})
	case "window":
		var p struct {
			Command string `json:"command"`
		}
		if e := decode(w, r, &p); e != nil {
			failure(w, 400, e)
			return
		}
		if a.Window == nil {
			failure(w, 409, errors.New("当前为浏览器模式，请使用浏览器窗口按钮"))
			return
		}
		if e := a.Window.Command(p.Command); e != nil {
			failure(w, 400, e)
			return
		}
		jsonOut(w, map[string]bool{"accepted": true})
	case "hardware/refresh":
		jsonOut(w, map[string]bool{"started": a.Hardware.Refresh(a.Ctx)})
	case "hardware/cancel":
		jsonOut(w, map[string]bool{"cancelled": a.Hardware.Cancel()})
	case "diagnose":
		report := diagnostics.Run(a.Monitor.State(false).Latest, a.Hardware.Get(), time.Now())
		a.mu.Lock()
		a.report = &report
		a.mu.Unlock()
		jsonOut(w, report)
	case "tool":
		var p struct {
			ID        string `json:"id"`
			Elevate   bool   `json:"elevate"`
			Confirmed bool   `json:"confirmed"`
		}
		if e := decode(w, r, &p); e != nil {
			failure(w, 400, e)
			return
		}
		t, ok := systemtools.Find(p.ID)
		if !ok {
			failure(w, 400, errors.New("未知工具"))
			return
		}
		if (t.Admin || p.Elevate) && !p.Confirmed {
			failure(w, 400, errors.New("管理工具需要用户确认"))
			return
		}
		if !a.action.TryLock() {
			failure(w, 409, errors.New("另一个系统操作尚未完成"))
			return
		}
		defer a.action.Unlock()
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		result, e := systemtools.Launch(ctx, p.ID, p.Elevate)
		a.Log.Printf("tool=%s accepted=%v", p.ID, e == nil)
		if e != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(409)
			jsonOut(w, map[string]any{"error": e.Error(), "needsElevation": result.NeedsElevation})
			return
		}
		jsonOut(w, result)
	case "startup":
		var p struct {
			Enabled   bool `json:"enabled"`
			Confirmed bool `json:"confirmed"`
		}
		if e := decode(w, r, &p); e != nil {
			failure(w, 400, e)
			return
		}
		if !p.Confirmed {
			failure(w, 400, errors.New("需要明确确认修改当前用户的启动快捷方式"))
			return
		}
		if !a.action.TryLock() {
			failure(w, 409, errors.New("系统操作进行中"))
			return
		}
		defer a.action.Unlock()
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if e := systemtools.SetStartup(ctx, p.Enabled); e != nil {
			failure(w, 500, e)
			return
		}
		jsonOut(w, map[string]bool{"enabled": systemtools.StartupStatus()})
	case "network/check":
		var p struct {
			Kind      string `json:"kind"`
			Target    string `json:"target"`
			Confirmed bool   `json:"confirmed"`
		}
		if e := decode(w, r, &p); e != nil {
			failure(w, 400, e)
			return
		}
		if !p.Confirmed {
			failure(w, 400, errors.New("联网测试需要用户确认"))
			return
		}
		if !a.action.TryLock() {
			failure(w, 409, errors.New("另一个检测正在进行"))
			return
		}
		defer a.action.Unlock()
		ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
		defer cancel()
		result, e := systemtools.CheckNetwork(ctx, p.Kind, p.Target)
		if e != nil {
			failure(w, 400, e)
			return
		}
		jsonOut(w, result)
	case "logs/clear":
		var p struct {
			Confirmed bool `json:"confirmed"`
		}
		if e := decode(w, r, &p); e != nil || !p.Confirmed {
			failure(w, 400, errors.New("需要确认清空本程序日志"))
			return
		}
		if e := a.LogFile.Clear(); e != nil {
			failure(w, 500, e)
			return
		}
		jsonOut(w, map[string]bool{"ok": true})
	case "ui-ready":
		if ready, ok := a.Window.(interface{ FrontendReady() error }); ok {
			if err := ready.FrontendReady(); err != nil {
				failure(w, 409, err)
				return
			}
		}
		a.Log.Printf("UI ready elapsed_ms=%d (includes browser/manual delay; not a cold-start benchmark)", time.Since(a.started).Milliseconds())
		jsonOut(w, map[string]bool{"ok": true})
	case "exit":
		jsonOut(w, map[string]bool{"ok": true})
		go func() {
			time.Sleep(100 * time.Millisecond)
			if a.Window != nil {
				a.Window.Command("close")
			}
			a.Cancel()
		}()
	default:
		http.NotFound(w, r)
	}
}
func (a *App) Description() string {
	return fmt.Sprintf("嘉豪工具箱 %s (%s/%s)", Version, runtime.GOOS, runtime.GOARCH)
}
