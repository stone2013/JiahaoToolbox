//go:build windows && amd64

package monitor

import (
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var kernelDLL = syscall.NewLazyDLL("kernel32.dll")
var systemTimes = kernelDLL.NewProc("GetSystemTimes")
var activeProcessors = kernelDLL.NewProc("GetActiveProcessorCount")
var memoryStatus = kernelDLL.NewProc("GlobalMemoryStatusEx")
var diskSpace = kernelDLL.NewProc("GetDiskFreeSpaceExW")
var logicalDrives = kernelDLL.NewProc("GetLogicalDrives")
var driveType = kernelDLL.NewProc("GetDriveTypeW")
var volumeInfo = kernelDLL.NewProc("GetVolumeInformationW")
var tickCount = kernelDLL.NewProc("GetTickCount64")
var ipHelper = syscall.NewLazyDLL("iphlpapi.dll")
var getIfTable2 = ipHelper.NewProc("GetIfTable2")
var freeMibTable = ipHelper.NewProc("FreeMibTable")

type memStatus struct {
	Length, Load                                                                          uint32
	TotalPhys, AvailPhys, TotalPage, AvailPage, TotalVirtual, AvailVirtual, AvailExtended uint64
}
type mibIfRow struct {
	Luid                                                                                                                                                                                                                               uint64
	Index                                                                                                                                                                                                                              uint32
	GUID                                                                                                                                                                                                                               [16]byte
	Alias                                                                                                                                                                                                                              [257]uint16
	Description                                                                                                                                                                                                                        [257]uint16
	PhysLen                                                                                                                                                                                                                            uint32
	Phys                                                                                                                                                                                                                               [32]byte
	Permanent                                                                                                                                                                                                                          [32]byte
	MTU, Type, Tunnel, Media, Physical, Access, Direction                                                                                                                                                                              uint32
	Flags                                                                                                                                                                                                                              byte
	_                                                                                                                                                                                                                                  [3]byte
	Oper, Admin, MediaConnect                                                                                                                                                                                                          uint32
	NetworkGUID                                                                                                                                                                                                                        [16]byte
	Connection                                                                                                                                                                                                                         uint32
	_2                                                                                                                                                                                                                                 uint32
	TxLink, RxLink, InOctets, InUcast, InNUcast, InDiscards, InErrors, InUnknown, InUcastOctets, InMulticast, InBroadcast, OutOctets, OutUcast, OutNUcast, OutDiscards, OutErrors, OutUcastOctets, OutMulticast, OutBroadcast, OutQLen uint64
}
type winNet struct {
	rx, tx uint64
	t      time.Time
}
type diskCounters struct{ idle, read, write uintptr }
type platformCollector struct {
	q                           *pdhQuery
	total, cores, freq          uintptr
	disks                       map[string]diskCounters
	net                         map[string]winNet
	oldIdle, oldKernel, oldUser uint64
	warm                        bool
	lastDisk                    time.Time
	diskCache                   []Disk
	lastAddresses               time.Time
	addresses                   map[int][]string
	pdhError                    string
	gpuEngines, gpuMem          uintptr
	gpuCache                    []GPU
	lastGPU                     time.Time
}

func NewCollector() (Collector, error) {
	c := &platformCollector{disks: map[string]diskCounters{}, net: map[string]winNet{}, addresses: map[int][]string{}}
	q, e := newPDH()
	c.q = q
	if e != nil {
		c.pdhError = e.Error()
	}
	c.total = q.add(`\Processor Information(_Total)\% Processor Time`)
	c.cores = q.add(`\Processor Information(*)\% Processor Time`)
	if c.total == 0 {
		c.total = q.add(`\Processor(_Total)\% Processor Time`)
	}
	if c.cores == 0 {
		c.cores = q.add(`\Processor(*)\% Processor Time`)
	}
	c.freq = q.add(`\Processor Information(_Total)\Processor Frequency`)
	c.gpuEngines = q.add(`\GPU Engine(*)\Utilization Percentage`)
	c.gpuMem = q.add(`\GPU Adapter Memory(*)\Dedicated Usage`)
	return c, nil
}
func (c *platformCollector) Close() error { c.q.close(); return nil }
func (c *platformCollector) Sample() Snapshot {
	start := time.Now()
	s := Empty()
	s.Time = start.UnixMilli()
	if start.Sub(c.lastDisk) > 5*time.Second {
		c.refreshDisks()
		c.lastDisk = start
	}
	pdhOK := c.q.collect() == nil
	if c.pdhError != "" {
		s.Errors = append(s.Errors, c.pdhError)
	}
	var idle, kernel, user uint64
	r, _, e := systemTimes.Call(uintptr(unsafe.Pointer(&idle)), uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user)))
	if r != 0 {
		if c.warm {
			s.CPU.Usage = CPUUsage(idle, kernel, user, c.oldIdle, c.oldKernel, c.oldUser)
		}
		c.oldIdle, c.oldKernel, c.oldUser = idle, kernel, user
		c.warm = true
	} else {
		s.CPU.Usage = Unavailable("%", e.Error())
	}
	// GetSystemTimes may cover only the current processor group above 64 CPUs.
	if activeProcessors.Find() == nil {
		if n, _, _ := activeProcessors.Call(0xffff); n > 64 {
			s.CPU.Usage = Unavailable("%", "多处理器组系统需要有效 PDH 总体计数，不能用单组时间替代全机负载")
		}
	}
	if pdhOK {
		if v := counter(c.total, "%"); v.Value != nil {
			s.CPU.Usage = clampPercent(v)
		}
		cores := counterArray(c.cores)
		names := make([]string, 0, len(cores))
		for n := range cores {
			if !strings.Contains(n, "_Total") {
				names = append(names, n)
			}
		}
		sort.Slice(names, func(i, j int) bool { return numericName(names[i]) < numericName(names[j]) })
		for _, n := range names {
			s.CPU.Names = append(s.CPU.Names, n)
			s.CPU.Logical = append(s.CPU.Logical, clampPercent(Valid(cores[n], "%")))
		}
		s.CPU.Frequency = counter(c.freq, "MHz")
	}
	var m memStatus
	m.Length = uint32(unsafe.Sizeof(m))
	r, _, e = memoryStatus.Call(uintptr(unsafe.Pointer(&m)))
	if r != 0 && m.TotalPhys >= m.AvailPhys {
		s.Memory = Memory{Total: m.TotalPhys, Available: m.AvailPhys, Used: m.TotalPhys - m.AvailPhys, Usage: Percent(m.TotalPhys-m.AvailPhys, m.TotalPhys)}
	} else {
		s.Memory.Usage = Unavailable("%", e.Error())
	}
	s.Disks = append([]Disk{}, c.diskCache...)
	for i := range s.Disks {
		d := &s.Disks[i]
		h := c.disks[d.ID]
		if pdhOK {
			v := counter(h.idle, "%")
			if v.Value != nil {
				d.Activity = clampPercent(Valid(100-*v.Value, "%"))
			} else {
				d.Activity = v
			}
			d.Read = counter(h.read, "B/s")
			d.Write = counter(h.write, "B/s")
		}
	}
	if nets, e := c.networks(start); e == nil {
		s.Networks = nets
	} else {
		s.Errors = append(s.Errors, "网络采样失败："+e.Error())
	}
	ticks, _, _ := tickCount.Call()
	s.Uptime = uint64(ticks) / 1000
	if start.Sub(c.lastGPU) >= 30*time.Second {
		g, e := readGPUs()
		c.gpuCache = g
		c.lastGPU = start
		if e != nil {
			s.Errors = append(s.Errors, "GPU 枚举不可用："+e.Error())
		}
	}
	s.GPUs = append([]GPU{}, c.gpuCache...)
	if pdhOK {
		loads := gpuLoads(counterArray(c.gpuEngines))
		memory := gpuMemory(counterArray(c.gpuMem))
		for i := range s.GPUs {
			g := &s.GPUs[i]
			if v, ok := loads[g.ID]; ok {
				g.Usage = v
			}
			if v, ok := memory[g.ID]; ok {
				g.Memory = v
			}
		}
	}
	s.DurationMS = float64(time.Since(start).Microseconds()) / 1000
	return s
}
func numericName(n string) int {
	p := strings.Split(n, ",")
	a, _ := strconv.Atoi(p[0])
	if len(p) == 2 {
		b, _ := strconv.Atoi(p[1])
		return a*65536 + b
	}
	return a
}
func clampPercent(v Value) Value {
	if v.Value == nil {
		return v
	}
	n := *v.Value
	if n < 0 {
		n = 0
	}
	if n > 100 {
		n = 100
	}
	return Valid(n, "%")
}
func (c *platformCollector) refreshDisks() {
	mask, _, _ := logicalDrives.Call()
	out := []Disk{}
	for i := 0; i < 26; i++ {
		if mask&(1<<i) == 0 {
			continue
		}
		id := string(rune('A'+i)) + ":"
		path := syscall.StringToUTF16Ptr(id + `\`)
		kind, _, _ := driveType.Call(uintptr(unsafe.Pointer(path)))
		if kind != 3 {
			continue
		} // fixed disks only; avoid network/removable blocking.
		d := Disk{ID: id, Name: id, Usage: Unavailable("%", "无法读取容量"), Activity: Unsupported("%", "逻辑磁盘性能计数器不可用"), Read: Unsupported("B/s", "性能计数器不可用"), Write: Unsupported("B/s", "性能计数器不可用")}
		var avail, total, free uint64
		r, _, _ := diskSpace.Call(uintptr(unsafe.Pointer(path)), uintptr(unsafe.Pointer(&avail)), uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&free)))
		if r != 0 && total >= free {
			d.Total = total
			d.Free = free
			d.Usage = Percent(total-free, total)
		}
		name, fs := make([]uint16, 261), make([]uint16, 64)
		r, _, _ = volumeInfo.Call(uintptr(unsafe.Pointer(path)), uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)), 0, 0, 0, uintptr(unsafe.Pointer(&fs[0])), uintptr(len(fs)))
		if r != 0 {
			label := syscall.UTF16ToString(name)
			if label != "" {
				d.Name = label + " (" + id + ")"
			}
			d.FileSystem = syscall.UTF16ToString(fs)
		}
		if _, ok := c.disks[id]; !ok {
			c.disks[id] = diskCounters{c.q.add(`\LogicalDisk(` + id + `)\% Idle Time`), c.q.add(`\LogicalDisk(` + id + `)\Disk Read Bytes/sec`), c.q.add(`\LogicalDisk(` + id + `)\Disk Write Bytes/sec`)}
		}
		out = append(out, d)
	}
	c.diskCache = out
}
func (c *platformCollector) networks(now time.Time) ([]Network, error) {
	if now.Sub(c.lastAddresses) > 5*time.Second {
		a := map[int][]string{}
		ifs, e := net.Interfaces()
		if e == nil {
			for _, n := range ifs {
				ips, _ := n.Addrs()
				for _, ip := range ips {
					a[n.Index] = append(a[n.Index], ip.String())
				}
			}
			c.addresses = a
		}
		c.lastAddresses = now
	}
	var table unsafe.Pointer
	r, _, _ := getIfTable2.Call(uintptr(unsafe.Pointer(&table)))
	if r != 0 {
		return nil, fmt.Errorf("GetIfTable2 %d", r)
	}
	if table == nil {
		return nil, fmt.Errorf("空接口表")
	}
	defer freeMibTable.Call(uintptr(table))
	count := *(*uint32)(table)
	if count > 65536 {
		return nil, fmt.Errorf("接口表长度异常")
	}
	rows := unsafe.Slice((*mibIfRow)(unsafe.Add(table, 8)), int(count))
	out := []Network{}
	next := map[string]winNet{}
	for _, row := range rows {
		if row.Type == 24 {
			continue
		}
		id := strconv.FormatUint(row.Luid, 10)
		n := Network{ID: id, Name: syscall.UTF16ToString(row.Alias[:]), Description: syscall.UTF16ToString(row.Description[:]), Physical: row.Flags&1 != 0, Up: row.Oper == 1, Addresses: append([]string{}, c.addresses[int(row.Index)]...), Received: row.InOctets, Sent: row.OutOctets, Download: Unavailable("B/s", "等待第二次采样"), Upload: Unavailable("B/s", "等待第二次采样")}
		if old, ok := c.net[id]; ok {
			dt := now.Sub(old.t).Seconds()
			n.Download = Rate(row.InOctets, old.rx, dt)
			n.Upload = Rate(row.OutOctets, old.tx, dt)
		}
		next[id] = winNet{row.InOctets, row.OutOctets, now}
		out = append(out, n)
	}
	c.net = next
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Up != out[j].Up {
			return out[i].Up
		}
		if out[i].Physical != out[j].Physical {
			return out[i].Physical
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}
