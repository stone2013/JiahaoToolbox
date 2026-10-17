package benchmark

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const markerName = "JHTB-v4.3.marker"
const markerText = "JiahaoToolbox v4.3 temporary disk benchmark data\n"

type DiskOptions struct {
	Directory       string `json:"directory"`
	Mode            string `json:"mode"`
	FileSizeMiB     int64  `json:"fileSizeMiB"`
	QueueDepth      int    `json:"queueDepth"`
	DurationSeconds int    `json:"durationSeconds"`
	AvailableBytes  uint64 `json:"availableBytes"`
}
type DiskProgress struct {
	Stage       string  `json:"stage"`
	Progress    float64 `json:"progress"`
	CurrentMBps float64 `json:"currentMBps"`
}
type DiskResult struct {
	Directory           string    `json:"directory"`
	FileSizeBytes       int64     `json:"fileSizeBytes"`
	BlockSizeBytes      int       `json:"blockSizeBytes"`
	QueueDepth          int       `json:"queueDepth"`
	DurationSeconds     int       `json:"durationSeconds"`
	Mode                string    `json:"mode"`
	CachePolicy         string    `json:"cachePolicy"`
	Time                time.Time `json:"time"`
	SequentialReadMBps  float64   `json:"sequentialReadMBps"`
	SequentialWriteMBps float64   `json:"sequentialWriteMBps"`
	RandomReadIOPS      float64   `json:"randomReadIOPS"`
	RandomWriteIOPS     float64   `json:"randomWriteIOPS"`
	Cleanup             string    `json:"cleanup"`
	Note                string    `json:"note"`
}

func NormalizeDiskOptions(o DiskOptions) (DiskOptions, error) {
	if o.Mode == "" {
		o.Mode = "quick"
	}
	switch o.Mode {
	case "quick":
		o.FileSizeMiB = 64
		o.QueueDepth = 1
		o.DurationSeconds = 2
	case "standard":
		o.FileSizeMiB = 256
		o.QueueDepth = 1
		o.DurationSeconds = 4
	case "advanced":
		if o.FileSizeMiB < 32 || o.FileSizeMiB > 1024 {
			return o, errors.New("高级测试文件大小需为 32–1024 MiB")
		}
		if o.QueueDepth != 1 && o.QueueDepth != 2 && o.QueueDepth != 4 && o.QueueDepth != 8 {
			return o, errors.New("队列深度仅支持 1、2、4 或 8")
		}
		if o.DurationSeconds < 1 || o.DurationSeconds > 20 {
			return o, errors.New("高级测试时长需为 1–20 秒")
		}
	default:
		return o, errors.New("未知磁盘测试模式")
	}
	if strings.TrimSpace(o.Directory) == "" {
		return o, errors.New("请选择测试目录")
	}
	abs, e := filepath.Abs(o.Directory)
	if e != nil {
		return o, e
	}
	o.Directory = filepath.Clean(abs)
	st, e := os.Stat(o.Directory)
	if e != nil {
		return o, fmt.Errorf("无法读取所选目录：%w", e)
	}
	if !st.IsDir() {
		return o, errors.New("测试位置必须是文件夹")
	}
	if uint64(o.FileSizeMiB)*1024*1024 > o.AvailableBytes {
		return o, fmt.Errorf("可用空间不足：需要 %d MiB，当前约 %d MiB", o.FileSizeMiB, o.AvailableBytes/1024/1024)
	}
	return o, nil
}

// RunDiskTest uses a unique, exclusive temporary directory under the user's
// chosen folder. It only removes its own marker and data file, never user files.
// Buffered Windows I/O is explicit: OS file cache can affect read results.
func RunDiskTest(ctx context.Context, input DiskOptions, report func(DiskProgress)) (out DiskResult, err error) {
	o, e := NormalizeDiskOptions(input)
	if e != nil {
		return out, e
	}
	if report == nil {
		report = func(DiskProgress) {}
	}
	dir, e := os.MkdirTemp(o.Directory, ".jiahao-benchmark-")
	if e != nil {
		return out, fmt.Errorf("无法在所选文件夹建立独立测试目录：%w", e)
	}
	marker := filepath.Join(dir, markerName)
	if e = os.WriteFile(marker, []byte(markerText), 0600); e != nil {
		_ = os.Remove(dir)
		return out, e
	}
	filePath := filepath.Join(dir, "test-data.bin")
	file, e := os.OpenFile(filePath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if e != nil {
		safeCleanup(dir)
		return out, fmt.Errorf("无法创建独立测试文件：%w", e)
	}
	defer func() {
		if file != nil {
			if ce := file.Close(); err == nil && ce != nil {
				err = ce
			}
		}
		if ce := safeCleanup(dir); ce != nil && err == nil {
			err = fmt.Errorf("测试文件清理失败：%w", ce)
		}
		if err == nil {
			out.Cleanup = "测试文件及本次专用目录已删除"
		}
	}()
	size := o.FileSizeMiB * 1024 * 1024
	out = DiskResult{Directory: o.Directory, FileSizeBytes: size, BlockSizeBytes: 4096, QueueDepth: o.QueueDepth, DurationSeconds: o.DurationSeconds, Mode: o.Mode, CachePolicy: "Windows 缓冲文件 I/O；顺序写入后调用 FlushFileBuffers（File.Sync）。重开文件读取，但 Windows 仍可能缓存；结果代表当前系统文件 I/O 路径，不等于裸设备吞吐。", Time: time.Now(), Note: "性能受磁盘类型、文件系统、加密/压缩、剩余空间、后台负载及缓存影响。测试数据为本程序新建的临时文件，不读取或覆盖用户文件。"}
	buf := make([]byte, 1024*1024)
	for i := range buf {
		buf[i] = byte((i*31 + 17) % 251)
	}
	start := time.Now()
	for off := int64(0); off < size; off += int64(len(buf)) {
		if e = ctx.Err(); e != nil {
			return out, e
		}
		n := min(int64(len(buf)), size-off)
		w, we := file.WriteAt(buf[:n], off)
		if we != nil {
			return out, fmt.Errorf("顺序写入失败：%w", we)
		}
		if int64(w) != n {
			return out, io.ErrShortWrite
		}
		report(DiskProgress{Stage: "顺序写入", Progress: .05 + .32*float64(off+n)/float64(size), CurrentMBps: float64(off+n) / 1024 / 1024 / time.Since(start).Seconds()})
	}
	if e = file.Sync(); e != nil {
		return out, fmt.Errorf("刷新写入缓存失败：%w", e)
	}
	if e = file.Close(); e != nil {
		return out, e
	}
	file = nil
	out.SequentialWriteMBps = float64(size) / 1024 / 1024 / time.Since(start).Seconds()
	file, e = os.OpenFile(filePath, os.O_RDWR, 0600)
	if e != nil {
		return out, e
	}
	start = time.Now()
	readbuf := make([]byte, len(buf))
	for off := int64(0); off < size; off += int64(len(readbuf)) {
		if e = ctx.Err(); e != nil {
			return out, e
		}
		n := min(int64(len(readbuf)), size-off)
		r, re := file.ReadAt(readbuf[:n], off)
		if re != nil && !(re == io.EOF && int64(r) == n) {
			return out, fmt.Errorf("顺序读取失败：%w", re)
		}
		if int64(r) != n {
			return out, io.ErrUnexpectedEOF
		}
		report(DiskProgress{Stage: "顺序读取", Progress: .38 + .28*float64(off+n)/float64(size), CurrentMBps: float64(off+n) / 1024 / 1024 / time.Since(start).Seconds()})
	}
	out.SequentialReadMBps = float64(size) / 1024 / 1024 / time.Since(start).Seconds()
	const block = int64(4096)
	randomIO := func(stage string, base float64, write bool) (float64, error) {
		var ops atomic.Int64
		started := time.Now()
		deadline := started.Add(time.Duration(o.DurationSeconds) * time.Second)
		var wg sync.WaitGroup
		errCh := make(chan error, o.QueueDepth)
		for worker := 0; worker < o.QueueDepth; worker++ {
			wg.Add(1)
			go func(seed int64) {
				defer wg.Done()
				rng := rand.New(rand.NewSource(seed))
				b := make([]byte, block)
				for {
					if ctx.Err() != nil || time.Now().After(deadline) {
						return
					}
					slots := size / block
					if slots < 1 {
						return
					}
					off := int64(rng.Int63n(slots)) * block
					if write {
						for i := range b {
							b[i] = byte((i*17 + int(off)) % 251)
						}
						n, e := file.WriteAt(b, off)
						if e != nil || n != len(b) {
							if e == nil {
								e = io.ErrShortWrite
							}
							select {
							case errCh <- e:
							default:
							}
							return
						}
					} else {
						n, e := file.ReadAt(b, off)
						if e != nil && !(e == io.EOF && n == len(b)) {
							select {
							case errCh <- e:
							default:
							}
							return
						}
						if n != len(b) {
							select {
							case errCh <- io.ErrUnexpectedEOF:
							default:
							}
							return
						}
					}
					ops.Add(1)
				}
			}(time.Now().UnixNano() + int64(worker)*7919)
		}
		tick := time.NewTicker(150 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				wg.Wait()
				return 0, ctx.Err()
			case ioErr := <-errCh:
				wg.Wait()
				return 0, fmt.Errorf("%s失败：%w", stage, ioErr)
			case <-tick.C:
				elapsed := time.Since(started)
				if elapsed >= time.Duration(o.DurationSeconds)*time.Second {
					wg.Wait()
					return float64(ops.Load()) / elapsed.Seconds(), nil
				}
				report(DiskProgress{Stage: stage, Progress: base + .07*float64(elapsed)/float64(time.Duration(o.DurationSeconds)*time.Second)})
			}
		}
	}
	out.RandomReadIOPS, e = randomIO("4K 随机读取", .68, false)
	if e != nil {
		return out, e
	}
	report(DiskProgress{Stage: "4K 随机读取完成", Progress: .76})
	out.RandomWriteIOPS, e = randomIO("4K 随机写入", .77, true)
	if e != nil {
		return out, e
	}
	if e = file.Sync(); e != nil {
		return out, fmt.Errorf("刷新随机写入缓存失败：%w", e)
	}
	report(DiskProgress{Stage: "完成", Progress: 1})
	return out, nil
}

// RecoverOrphans only removes recognized v4.3 benchmark files in a direct,
// uniquely named child directory of the folder the user selected.
func RecoverOrphans(parent string) (int, error) {
	abs, e := filepath.Abs(parent)
	if e != nil {
		return 0, e
	}
	entries, e := os.ReadDir(abs)
	if e != nil {
		return 0, e
	}
	removed := 0
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), ".jiahao-benchmark-") {
			continue
		}
		child := filepath.Join(abs, entry.Name())
		st, e := os.Lstat(child)
		if e != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			continue
		}
		marker := filepath.Join(child, markerName)
		m, e := os.ReadFile(marker)
		if e != nil || string(m) != markerText {
			continue
		}
		data := filepath.Join(child, "test-data.bin")
		if fi, e := os.Lstat(data); e == nil && fi.Mode().IsRegular() {
			if e = os.Remove(data); e != nil {
				continue
			}
		} else if !os.IsNotExist(e) {
			continue
		}
		if e = os.Remove(marker); e != nil {
			continue
		}
		if e = os.Remove(child); e == nil {
			removed++
		}
	}
	return removed, nil
}
func safeCleanup(dir string) error {
	st, e := os.Lstat(dir)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return errors.New("测试目录状态已改变，拒绝清理")
	}
	m, e := os.ReadFile(filepath.Join(dir, markerName))
	if e != nil || string(m) != markerText {
		return errors.New("测试目录标识缺失，拒绝删除目录内容")
	}
	data := filepath.Join(dir, "test-data.bin")
	if _, e = os.Lstat(data); e == nil {
		if e = os.Remove(data); e != nil {
			return e
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	if e = os.Remove(filepath.Join(dir, markerName)); e != nil {
		return e
	}
	return os.Remove(dir)
}
