package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"jiahao-toolbox/internal/config"
	updater "jiahao-toolbox/internal/update"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testApp(t *testing.T) *App {
	t.Helper()
	a, e := New(context.Background(), t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(a.Close)
	return a
}
func request(a *App, method, path, body string, authorized, verified bool) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, a.Origin+path, strings.NewReader(body))
	if authorized {
		r.AddCookie(&http.Cookie{Name: a.cookieName, Value: a.token})
	}
	if verified {
		r.Header.Set("Origin", a.Origin)
		r.Header.Set("X-Jiahao-Request", "1")
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}
func TestSessionBootstrapAndSecurityHeaders(t *testing.T) {
	a := testApp(t)
	if w := request(a, "GET", "/api/bootstrap", "", false, false); w.Code != 401 {
		t.Fatal(w.Code)
	}
	w := request(a, "GET", "/start/"+a.token, "", false, false)
	if w.Code != 303 {
		t.Fatal(w.Code)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatal("weak cookie", cookies)
	}
	w = request(a, "GET", "/api/bootstrap", "", true, false)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if !strings.Contains(w.Header().Get("Content-Security-Policy"), "script-src 'self'") || w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("security headers absent")
	}
	var b map[string]any
	if e := json.Unmarshal(w.Body.Bytes(), &b); e != nil || b["version"] != Version {
		t.Fatal(e, b)
	}
}
func TestOriginHostAndCSRFRejected(t *testing.T) {
	a := testApp(t)
	for _, edit := range []func(*http.Request){func(r *http.Request) { r.Host = "evil.example" }, func(r *http.Request) { r.Header.Set("Origin", "https://evil.example") }, func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "cross-site") }} {
		r := httptest.NewRequest("GET", a.Origin+"/api/bootstrap", nil)
		r.AddCookie(&http.Cookie{Name: a.cookieName, Value: a.token})
		edit(r)
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("unsafe request accepted", w.Code)
		}
	}
	if w := request(a, "POST", "/monitor", `{}`, true, false); w.Code != 403 {
		t.Fatal(w.Code)
	}
}
func TestEmbeddedOfflineAssets(t *testing.T) {
	a := testApp(t)
	for _, path := range []string{"/", "/assets/main.js", "/assets/styles/app.css", "/assets/assets/logo.svg", "/assets/assets/mountains.svg"} {
		w := request(a, "GET", path, "", true, false)
		if w.Code != 200 || w.Body.Len() == 0 {
			t.Fatal(path, w.Code)
		}
	}
	for _, path := range []string{"/go.mod", "/internal/app/app.go", "/assets/missing.js", "/assets/../source"} {
		if w := request(a, "GET", path, "", true, false); w.Code != 404 {
			t.Fatal("non-public file exposed", path, w.Code)
		}
	}
}
func TestStrictSettingsAndPersistence(t *testing.T) {
	a := testApp(t)
	for _, payload := range []string{"{}", "null", "[]", `{"unknown":1}`, `{} {}`, strings.Repeat(" ", 17000) + "{}"} {
		w := request(a, "POST", "/api/settings", payload, true, true)
		if w.Code != 400 {
			t.Fatal("bad settings accepted", w.Code)
		}
		if a.Config.Get() != config.Defaults() {
			t.Fatal("invalid settings mutated state")
		}
		if !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
			t.Fatal("incorrect JSON MIME")
		}
	}
	v := config.Defaults()
	v.Theme = "light"
	v.IntervalMS = 500
	b, _ := json.Marshal(v)
	w := request(a, "POST", "/api/settings", string(b), true, true)
	if w.Code != 200 || a.Config.Get() != v {
		t.Fatal(w.Code, w.Body)
	}
	stored, e := os.ReadFile(filepath.Join(a.DataDir, "settings.json"))
	if e != nil || !bytes.Contains(stored, []byte("light")) {
		t.Fatal(e)
	}
}
func TestNoArgumentCommandsRejectExtraData(t *testing.T) {
	a := testApp(t)
	for _, path := range []string{"diagnose", "hardware/refresh", "hardware/cancel", "ui-ready", "exit"} {
		for _, payload := range []string{"null", `{"command":"unexpected"}`, "{}{}"} {
			if w := request(a, "POST", "/api/"+path, payload, true, true); w.Code != 400 {
				t.Fatal(path, payload, w.Code)
			}
		}
	}
}
func TestReadonlyMonitoringAndDiagnostics(t *testing.T) {
	a := testApp(t)
	deadline := time.Now().Add(2 * time.Second)
	for a.Monitor.State(false).Samples == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	w := request(a, "GET", "/api/snapshot?history=1", "", true, false)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	var data struct {
		Samples uint64
		History []any
	}
	if e := json.Unmarshal(w.Body.Bytes(), &data); e != nil || data.Samples == 0 || len(data.History) > 121 {
		t.Fatal(e, data)
	}
	w = request(a, "POST", "/api/monitor", `{"paused":true,"background":false}`, true, true)
	if w.Code != 200 || !a.Monitor.State(false).Paused {
		t.Fatal(w.Code)
	}
	w = request(a, "POST", "/api/diagnose", `{}`, true, true)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	var report struct {
		Checks []any
		Rules  string
	}
	if e := json.Unmarshal(w.Body.Bytes(), &report); e != nil || len(report.Checks) != 5 || report.Rules == "" {
		t.Fatal("diagnostic report missing", e, report)
	}
	w = request(a, "POST", "/api/monitor", `{"paused":false,"background":true}`, true, true)
	if w.Code != 200 || !a.Monitor.State(false).Background {
		t.Fatal(w.Code)
	}
}
func TestDangerousActionsRequireConfirmationAndWhitelist(t *testing.T) {
	a := testApp(t)
	cases := map[string]string{"tool": `{"id":"cmd.exe /c whoami","confirmed":true}`, "startup": `{"enabled":true,"confirmed":false}`, "network/check": `{"kind":"ping","target":"example.com","confirmed":false}`, "logs/clear": `{"confirmed":false}`}
	for path, payload := range cases {
		if w := request(a, "POST", "/api/"+path, payload, true, true); w.Code != 400 {
			t.Fatal(path, w.Code)
		}
	}
	if w := request(a, "POST", "/api/tool", `{"id":"device-manager","confirmed":false}`, true, true); w.Code != 400 {
		t.Fatal("admin tool opened without confirmation")
	}
	if w := request(a, "POST", "/api/network/check", `{"kind":"ping","target":"-n 9999","confirmed":true}`, true, true); w.Code != 400 {
		t.Fatal("untrusted host accepted")
	}
}
func TestRealLoopbackTransport(t *testing.T) {
	a := testApp(t)
	req, e := http.NewRequest("GET", a.Origin+"/api/snapshot", nil)
	if e != nil {
		t.Fatal(e)
	}
	req.AddCookie(&http.Cookie{Name: a.cookieName, Value: a.token})
	client := http.Client{Timeout: 3 * time.Second}
	res, e := client.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || !json.Valid(b) {
		t.Fatal("real loopback request failed", res.StatusCode, string(b))
	}
}
func TestHTTPMethodsAndUnknownEndpoints(t *testing.T) {
	a := testApp(t)
	if w := request(a, "DELETE", "/api/bootstrap", "", true, true); w.Code != 405 {
		t.Fatal(w.Code)
	}
	if w := request(a, "GET", "/api/no-such-endpoint", "", true, true); w.Code != 404 {
		t.Fatal(w.Code)
	}
}
func TestUpdateCheckAPIAndPersistentStatus(t *testing.T) {
 a:=testApp(t)
 remote:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if r.URL.Path!="/releases/latest"{t.Errorf("unexpected GitHub path: %s",r.URL.Path)};w.Header().Set("Content-Type","application/json");_,_=io.WriteString(w,`{"tag_name":"v4.5.0","name":"v4.5.0","body":"release notes","published_at":"2026-10-01T00:00:00Z","assets":[]}`)}));defer remote.Close()
 a.updateClient=&updater.Client{HTTP:remote.Client(),APIBase:remote.URL}
 w:=request(a,http.MethodPost,"/api/updates/check",`{}`,true,true);if w.Code!=200{t.Fatalf("check failed: %d %s",w.Code,w.Body)}
 var info updater.Info;if e:=json.Unmarshal(w.Body.Bytes(),&info);e!=nil{t.Fatal(e)};if !info.HasUpdate||info.LatestVersion!="v4.5.0"||info.CurrentVersion!=Version||info.ReleasePage!="https://github.com/stone2013/JiahaoToolbox/releases/tag/v4.5.0"{t.Fatalf("unexpected update response: %+v",info)}
 status:=request(a,http.MethodGet,"/api/updates/status","",true,false);if status.Code!=200||!strings.Contains(status.Body.String(),"v4.5.0"){t.Fatalf("status did not retain check result: %d %s",status.Code,status.Body)}
 persisted,e:=os.ReadFile(filepath.Join(a.DataDir,"update-check.json"));if e!=nil||!strings.Contains(string(persisted),"v4.5.0"){t.Fatalf("check state not saved: %v",e)}
}

func TestUpdateCheckAPIReportsNoRelease(t *testing.T) {
 a:=testApp(t);remote:=httptest.NewServer(http.NotFoundHandler());defer remote.Close();a.updateClient=&updater.Client{HTTP:remote.Client(),APIBase:remote.URL}
 w:=request(a,http.MethodPost,"/api/updates/check",`{}`,true,true);if w.Code!=200||!strings.Contains(w.Body.String(),"no_release"){t.Fatalf("404 must be rendered as no release: %d %s",w.Code,w.Body)}
}
