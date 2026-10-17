package benchmark

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const SpeedServer = "https://speed.cloudflare.com"

type SpeedProgress struct {
	Stage       string  `json:"stage"`
	Progress    float64 `json:"progress"`
	CurrentMbps float64 `json:"currentMbps"`
}
type SpeedResult struct {
	Server       string    `json:"server"`
	Location     string    `json:"location"`
	Time         time.Time `json:"time"`
	DownloadMbps float64   `json:"downloadMbps"`
	UploadMbps   float64   `json:"uploadMbps"`
	LatencyMS    float64   `json:"latencyMS"`
	JitterMS     float64   `json:"jitterMS"`
	LossPercent  float64   `json:"lossPercent"`
	LossMethod   string    `json:"lossMethod"`
	TrafficBytes int64     `json:"trafficBytes"`
	Note         string    `json:"note"`
}

type progressReader struct {
	r             io.Reader
	n             atomic.Int64
	started, last time.Time
	total         int64
	report        func(SpeedProgress)
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, e := r.r.Read(p)
	count := r.n.Add(int64(n))
	if n > 0 && r.report != nil {
		now := time.Now()
		if r.started.IsZero() {
			r.started = now
		}
		if r.last.IsZero() || now.Sub(r.last) >= 100*time.Millisecond {
			elapsed := now.Sub(r.started).Seconds()
			progress := .64 + .31*math.Min(1, float64(count)/float64(r.total))
			if elapsed > 0 {
				rate := float64(count) * 8 / elapsed / 1e6
				r.report(SpeedProgress{Stage: "上传测试", Progress: progress, CurrentMbps: rate})
			}
			r.last = now
		}
	}
	return n, e
}

// RunSpeedTest performs real HTTP throughput tests against Cloudflare's public
// speed-test service. All test bytes are generated in memory; no user files are read.
func RunSpeedTest(ctx context.Context, base string, report func(SpeedProgress)) (SpeedResult, error) {
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "https" && !(u.Scheme == "http" && (strings.HasPrefix(u.Host, "127.0.0.1:") || strings.HasPrefix(u.Host, "localhost:")))) || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		return SpeedResult{}, fmt.Errorf("测速服务器必须是 HTTPS 地址，例如 %s", SpeedServer)
	}
	if report == nil {
		report = func(SpeedProgress) {}
	}
	client := &http.Client{Timeout: 45 * time.Second, Transport: &http.Transport{Proxy: http.ProxyFromEnvironment, DisableKeepAlives: true, ForceAttemptHTTP2: true}, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 || !strings.EqualFold(req.URL.Host, u.Host) || req.URL.Scheme != "https" {
			return http.ErrUseLastResponse
		}
		return nil
	}}
	result := SpeedResult{Server: u.Host, Location: "Cloudflare Anycast 边缘节点（具体城市由服务端网络路由决定）", Time: time.Now(), LossMethod: "HTTP 探测失败率；不是 ICMP/UDP 网络层丢包率", Note: "结果受 Wi-Fi 信号、VPN、路由拥塞、后台流量及测速节点位置影响；测试约下载 25 MB、上传 10 MB。"}
	const probes = 10
	latencies := make([]float64, 0, probes)
	failed := 0
	for i := 0; i < probes; i++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		report(SpeedProgress{Stage: fmt.Sprintf("延迟与抖动探测 %d/%d", i+1, probes), Progress: .02 + .18*float64(i)/probes})
		q, cancel := context.WithTimeout(ctx, 5*time.Second)
		req, e := http.NewRequestWithContext(q, http.MethodGet, strings.TrimRight(base, "/")+"/cdn-cgi/trace?jhtb="+strconv.Itoa(i)+"-"+strconv.FormatInt(time.Now().UnixNano(), 10), nil)
		if e != nil {
			cancel()
			return result, e
		}
		req.Header.Set("Cache-Control", "no-cache")
		started := time.Now()
		resp, e := client.Do(req)
		if e == nil {
			_, e = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				e = fmt.Errorf("服务器返回 HTTP %d", resp.StatusCode)
			}
		}
		elapsed := float64(time.Since(started)) / float64(time.Millisecond)
		cancel()
		if e != nil {
			failed++
		} else {
			latencies = append(latencies, elapsed)
		}
	}
	result.LossPercent = float64(failed) / probes * 100
	if len(latencies) == 0 {
		return result, fmt.Errorf("延迟探测失败，请检查网络连接或稍后重试")
	}
	for _, v := range latencies {
		result.LatencyMS += v
	}
	result.LatencyMS /= float64(len(latencies))
	if len(latencies) > 1 {
		for i := 1; i < len(latencies); i++ {
			result.JitterMS += math.Abs(latencies[i] - latencies[i-1])
		}
		result.JitterMS /= float64(len(latencies) - 1)
	}
	const downBytes = int64(25_000_000)
	report(SpeedProgress{Stage: "下载测试", Progress: .2})
	dctx, dcancel := context.WithTimeout(ctx, 35*time.Second)
	defer dcancel()
	dreq, err := http.NewRequestWithContext(dctx, http.MethodGet, strings.TrimRight(base, "/")+"/__down?bytes="+strconv.FormatInt(downBytes, 10)+"&jhtb="+strconv.FormatInt(time.Now().UnixNano(), 10), nil)
	if err != nil {
		return result, err
	}
	dreq.Header.Set("Cache-Control", "no-cache")
	start := time.Now()
	resp, err := client.Do(dreq)
	if err != nil {
		return result, fmt.Errorf("下载测试失败：%w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return result, fmt.Errorf("下载服务器返回 HTTP %d", resp.StatusCode)
	}
	var read int64
	buf := make([]byte, 64*1024)
	last := start
	for {
		if err := ctx.Err(); err != nil {
			resp.Body.Close()
			return result, err
		}
		n, e := resp.Body.Read(buf)
		read += int64(n)
		now := time.Now()
		if now.Sub(last) >= 100*time.Millisecond {
			elapsed := now.Sub(start).Seconds()
			if elapsed > 0 {
				report(SpeedProgress{Stage: "下载测试", Progress: .2 + .42*math.Min(1, float64(read)/float64(downBytes)), CurrentMbps: float64(read) * 8 / elapsed / 1e6})
			}
			last = now
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			resp.Body.Close()
			return result, fmt.Errorf("下载中断：%w", e)
		}
	}
	resp.Body.Close()
	elapsed := time.Since(start).Seconds()
	if read == 0 || elapsed <= 0 {
		return result, fmt.Errorf("测速服务器未返回下载数据")
	}
	result.DownloadMbps = float64(read) * 8 / elapsed / 1e6
	result.TrafficBytes += read
	const upBytes = 10_000_000
	report(SpeedProgress{Stage: "上传测试", Progress: .64})
	data := make([]byte, upBytes)
	for i := range data {
		data[i] = byte((i*31 + 17) % 251)
	}
	reader := &progressReader{r: bytes.NewReader(data), total: int64(len(data)), report: report}
	ureq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(base, "/")+"/__up", reader)
	if err != nil {
		return result, err
	}
	ureq.ContentLength = int64(len(data))
	ureq.Header.Set("Content-Type", "application/octet-stream")
	ureq.Header.Set("Cache-Control", "no-cache")
	start = time.Now()
	ureqCtx, ucancel := context.WithTimeout(ctx, 35*time.Second)
	ureq = ureq.WithContext(ureqCtx)
	resp, err = client.Do(ureq)
	if err != nil {
		ucancel()
		return result, fmt.Errorf("上传测试失败：%w", err)
	}
	_, err = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	resp.Body.Close()
	ucancel()
	if err != nil {
		return result, fmt.Errorf("上传响应失败：%w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, fmt.Errorf("上传服务器返回 HTTP %d", resp.StatusCode)
	}
	elapsed = time.Since(start).Seconds()
	sent := reader.n.Load()
	if sent == 0 || elapsed <= 0 {
		return result, fmt.Errorf("上传测试未发送数据")
	}
	result.UploadMbps = float64(sent) * 8 / elapsed / 1e6
	result.TrafficBytes += sent
	report(SpeedProgress{Stage: "完成", Progress: 1, CurrentMbps: result.UploadMbps})
	return result, nil
}
