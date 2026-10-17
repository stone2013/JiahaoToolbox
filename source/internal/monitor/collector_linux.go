//go:build linux

package monitor

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type linuxCPU struct{ total, idle uint64 }
type linuxNet struct {
	rx, tx uint64
	t      time.Time
}
type platformCollector struct {
	cpu []linuxCPU
	net map[string]linuxNet
}

func NewCollector() (Collector, error)    { return &platformCollector{net: map[string]linuxNet{}}, nil }
func (c *platformCollector) Close() error { return nil }
func (c *platformCollector) Sample() Snapshot {
	start := time.Now()
	s := Empty()
	s.Errors = append(s.Errors, "Linux 开发预览：采集本容器真实数据，不能代表 Windows 实机")
	if b, e := os.ReadFile("/proc/stat"); e == nil {
		next := []linuxCPU{}
		for _, l := range strings.Split(string(b), "\n") {
			f := strings.Fields(l)
			if len(f) < 5 || !strings.HasPrefix(f[0], "cpu") {
				continue
			}
			v := linuxCPU{}
			for i := 1; i < len(f) && i <= 8; i++ {
				n, _ := strconv.ParseUint(f[i], 10, 64)
				v.total += n
				if i == 4 || i == 5 {
					v.idle += n
				}
			}
			u := Unavailable("%", "等待第二次采样")
			idx := len(next)
			if idx < len(c.cpu) {
				o := c.cpu[idx]
				if v.total >= o.total && v.idle >= o.idle {
					t, idle := v.total-o.total, v.idle-o.idle
					if t >= idle {
						u = Percent(t-idle, t)
					}
				}
			}
			next = append(next, v)
			if f[0] == "cpu" {
				s.CPU.Usage = u
			} else {
				s.CPU.Names = append(s.CPU.Names, f[0])
				s.CPU.Logical = append(s.CPU.Logical, u)
			}
		}
		c.cpu = next
	} else {
		s.Errors = append(s.Errors, e.Error())
	}
	if f, e := os.Open("/proc/meminfo"); e == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			var key string
			var n uint64
			fmt.Sscanf(sc.Text(), "%s %d", &key, &n)
			switch key {
			case "MemTotal:":
				s.Memory.Total = n * 1024
			case "MemAvailable:":
				s.Memory.Available = n * 1024
			}
		}
		f.Close()
		if s.Memory.Total >= s.Memory.Available {
			s.Memory.Used = s.Memory.Total - s.Memory.Available
			s.Memory.Usage = Percent(s.Memory.Used, s.Memory.Total)
		}
	}
	var fs syscall.Statfs_t
	if syscall.Statfs("/", &fs) == nil {
		total, free := fs.Blocks*uint64(fs.Bsize), fs.Bfree*uint64(fs.Bsize)
		s.Disks = append(s.Disks, Disk{ID: "/", Name: "容器根分区", FileSystem: "Linux filesystem", Total: total, Free: free, Usage: Percent(total-free, total), Activity: Unsupported("%", "容器未提供可映射的磁盘活动率"), Read: Unsupported("B/s", "容器根目录不能可靠映射到物理设备"), Write: Unsupported("B/s", "容器根目录不能可靠映射到物理设备")})
	}
	ifs, _ := net.Interfaces()
	addr := map[string][]string{}
	up := map[string]bool{}
	for _, n := range ifs {
		a, _ := n.Addrs()
		for _, v := range a {
			addr[n.Name] = append(addr[n.Name], v.String())
		}
		up[n.Name] = n.Flags&net.FlagUp != 0
	}
	if b, e := os.ReadFile("/proc/net/dev"); e == nil {
		next := map[string]linuxNet{}
		for _, l := range strings.Split(string(b), "\n") {
			p := strings.SplitN(l, ":", 2)
			if len(p) != 2 {
				continue
			}
			name := strings.TrimSpace(p[0])
			if name == "lo" {
				continue
			}
			f := strings.Fields(p[1])
			if len(f) < 9 {
				continue
			}
			rx, _ := strconv.ParseUint(f[0], 10, 64)
			tx, _ := strconv.ParseUint(f[8], 10, 64)
			v := Network{ID: name, Name: name, Description: "Linux interface", Up: up[name], Addresses: addr[name], Received: rx, Sent: tx, Download: Unavailable("B/s", "等待差值"), Upload: Unavailable("B/s", "等待差值")}
			if old, ok := c.net[name]; ok {
				dt := start.Sub(old.t).Seconds()
				v.Download = Rate(rx, old.rx, dt)
				v.Upload = Rate(tx, old.tx, dt)
			}
			next[name] = linuxNet{rx, tx, start}
			s.Networks = append(s.Networks, v)
		}
		c.net = next
	}
	if b, e := os.ReadFile("/proc/uptime"); e == nil {
		var u float64
		fmt.Sscanf(string(b), "%f", &u)
		s.Uptime = uint64(u)
	}
	s.DurationMS = float64(time.Since(start).Microseconds()) / 1000
	return s
}
