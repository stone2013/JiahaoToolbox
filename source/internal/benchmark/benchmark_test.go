package benchmark

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSpeedTestUsesSeparateRealDownloadUploadAndProbeStages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cdn-cgi/trace":
			_, _ = io.WriteString(w, "ok")
		case "/__down":
			n, _ := strconv.Atoi(r.URL.Query().Get("bytes"))
			if n < 1 || n > 30_000_000 {
				http.Error(w, "bad size", 400)
				return
			}
			w.Header().Set("Content-Length", strconv.Itoa(n))
			block := strings.NewReader(strings.Repeat("x", 4096))
			_, _ = io.CopyN(w, &repeatingReader{block: block, payload: strings.Repeat("x", 4096)}, int64(n))
		case "/__up":
			_, _ = io.Copy(io.Discard, r.Body)
			_, _ = io.WriteString(w, "ok")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	var stages []string
	result, err := RunSpeedTest(context.Background(), server.URL, func(p SpeedProgress) {
		if len(stages) == 0 || stages[len(stages)-1] != p.Stage {
			stages = append(stages, p.Stage)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.DownloadMbps <= 0 || result.UploadMbps <= 0 || result.TrafficBytes < 35_000_000 {
		t.Fatalf("expected real transfer counters: %+v", result)
	}
	if result.LossPercent != 0 || len(stages) < 4 {
		t.Fatalf("probe and transfer stages not reported: %+v %v", result, stages)
	}
	if !strings.Contains(result.LossMethod, "不是 ICMP") {
		t.Fatal(result.LossMethod)
	}
}

type repeatingReader struct {
	block   *strings.Reader
	payload string
}

func (r *repeatingReader) Read(p []byte) (int, error) {
	if r.block.Len() == 0 {
		r.block = strings.NewReader(r.payload)
	}
	return r.block.Read(p)
}

func TestSpeedTestRejectsNonTLSNonLoopbackServer(t *testing.T) {
	_, err := RunSpeedTest(context.Background(), "http://example.com", nil)
	if err == nil {
		t.Fatal("plain HTTP test server must be rejected")
	}
}

func TestSpeedTestHandlesDisconnectedServerAndCanCancelDuringDownload(t *testing.T) {
	disconnected := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "offline", http.StatusServiceUnavailable) }))
	_, err := RunSpeedTest(context.Background(), disconnected.URL, nil)
	disconnected.Close()
	if err == nil || !strings.Contains(err.Error(), "延迟探测失败") {
		t.Fatalf("offline server should fail clearly: %v", err)
	}

	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cdn-cgi/trace" {
			_, _ = io.WriteString(w, "ok")
			return
		}
		if r.URL.Path == "/__down" {
			flusher, _ := w.(http.Flusher)
			buf := []byte(strings.Repeat("x", 32*1024))
			for i := 0; i < 1000; i++ {
				if _, e := w.Write(buf); e != nil {
					return
				}
				if flusher != nil {
					flusher.Flush()
				}
				time.Sleep(5 * time.Millisecond)
			}
			return
		}
		http.NotFound(w, r)
	}))
	defer slow.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var once atomic.Bool
	_, err = RunSpeedTest(ctx, slow.URL, func(p SpeedProgress) {
		if p.Stage == "下载测试" && p.CurrentMbps > 0 && once.CompareAndSwap(false, true) {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("download test should be cancellable: %v", err)
	}
}

func TestDiskOptionsAndSpaceGuard(t *testing.T) {
	d := t.TempDir()
	o, err := NormalizeDiskOptions(DiskOptions{Directory: d, Mode: "quick", AvailableBytes: 128 << 20})
	if err != nil {
		t.Fatal(err)
	}
	if o.FileSizeMiB != 64 || o.QueueDepth != 1 || o.DurationSeconds != 2 {
		t.Fatalf("unexpected quick mode: %+v", o)
	}
	if _, err = NormalizeDiskOptions(DiskOptions{Directory: d, Mode: "quick", AvailableBytes: 8 << 20}); err == nil {
		t.Fatal("must refuse insufficient free space")
	}
	if _, err = NormalizeDiskOptions(DiskOptions{Directory: d, Mode: "advanced", FileSizeMiB: 2048, QueueDepth: 32, DurationSeconds: 90, AvailableBytes: 4 << 30}); err == nil {
		t.Fatal("must bound advanced settings")
	}
}

func TestDiskTestWritesAndRemovesOnlyOwnedTemporaryFiles(t *testing.T) {
	d := t.TempDir()
	userFile := filepath.Join(d, "keep my file.txt")
	if err := os.WriteFile(userFile, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	o := DiskOptions{Directory: d, Mode: "advanced", FileSizeMiB: 32, QueueDepth: 2, DurationSeconds: 1, AvailableBytes: 128 << 20}
	var stages []string
	res, err := RunDiskTest(context.Background(), o, func(p DiskProgress) { stages = append(stages, p.Stage) })
	if err != nil {
		t.Fatal(err)
	}
	if res.SequentialReadMBps <= 0 || res.SequentialWriteMBps <= 0 || res.RandomReadIOPS <= 0 || res.RandomWriteIOPS <= 0 {
		t.Fatalf("missing file IO results: %+v", res)
	}
	if res.Cleanup == "" {
		t.Fatal("cleanup not confirmed")
	}
	b, _ := os.ReadFile(userFile)
	if string(b) != "preserve" {
		t.Fatal("user file changed")
	}
	entries, _ := os.ReadDir(d)
	if len(entries) != 1 {
		t.Fatalf("temporary files left behind: %v stages=%v", entries, stages)
	}
}

func TestDiskCancelCleansTemporaryDataAndOrphanRecoveryIsNarrow(t *testing.T) {
	d := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := RunDiskTest(ctx, DiskOptions{Directory: d, Mode: "advanced", FileSizeMiB: 32, QueueDepth: 1, DurationSeconds: 1, AvailableBytes: 128 << 20}, nil)
	if err == nil {
		t.Fatal("canceled operation should stop")
	}
	entries, _ := os.ReadDir(d)
	if len(entries) != 0 {
		t.Fatalf("cancellation left test files: %v", entries)
	}
	owned := filepath.Join(d, ".jiahao-benchmark-orphan")
	if err = os.Mkdir(owned, 0700); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(owned, markerName), []byte(markerText), 0600)
	_ = os.WriteFile(filepath.Join(owned, "test-data.bin"), []byte("test"), 0600)
	unknown := filepath.Join(d, ".jiahao-benchmark-user")
	_ = os.Mkdir(unknown, 0700)
	_ = os.WriteFile(filepath.Join(unknown, "notes.txt"), []byte("keep"), 0600)
	n, err := RecoverOrphans(d)
	if err != nil || n != 1 {
		t.Fatalf("recovery %d %v", n, err)
	}
	if _, err = os.Stat(unknown); err != nil {
		t.Fatal("unrecognized directory was removed")
	}
	if _, err = os.Stat(owned); !os.IsNotExist(err) {
		t.Fatal("recognized orphan remains")
	}
}

func TestDiskCancelDuringRandomStage(t *testing.T) {
	d := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	o := DiskOptions{Directory: d, Mode: "advanced", FileSizeMiB: 32, QueueDepth: 1, DurationSeconds: 10, AvailableBytes: 128 << 20}
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		_, _ = RunDiskTest(ctx, o, func(p DiskProgress) {
			if strings.Contains(p.Stage, "4K 随机读取") {
				cancel()
			}
		})
	}()
	<-stopped
	entries, _ := os.ReadDir(d)
	if len(entries) != 0 {
		t.Fatalf("canceled random IO left test files: %v", entries)
	}
}
