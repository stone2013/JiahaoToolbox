package monitor

import (
	"math"
	"time"
)

type Value struct {
	Value  *float64 `json:"value"`
	Unit   string   `json:"unit"`
	Status string   `json:"status"`
	Reason string   `json:"reason,omitempty"`
}

func Valid(v float64, u string) Value {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return Unavailable(u, "读数非有限值")
	}
	return Value{Value: &v, Unit: u, Status: "ok"}
}
func Unavailable(u, r string) Value { return Value{Unit: u, Status: "unavailable", Reason: r} }
func Unsupported(u, r string) Value { return Value{Unit: u, Status: "unsupported", Reason: r} }
func Percent(used, total uint64) Value {
	if total == 0 {
		return Unavailable("%", "总量为零")
	}
	if used > total {
		return Unavailable("%", "已用量超过总量")
	}
	return Valid(float64(used)/float64(total)*100, "%")
}
func Rate(current, previous uint64, seconds float64) Value {
	if seconds <= 0 || current < previous {
		return Unavailable("B/s", "等待有效计数差值（首次采样、重置或设备变化）")
	}
	return Valid(float64(current-previous)/seconds, "B/s")
}
func CPUUsage(idle, kernel, user, oldIdle, oldKernel, oldUser uint64) Value {
	if idle < oldIdle || kernel < oldKernel || user < oldUser {
		return Unavailable("%", "CPU 计数器重置")
	}
	total := (kernel - oldKernel) + (user - oldUser)
	inactive := idle - oldIdle
	if total == 0 || inactive > total {
		return Unavailable("%", "等待 CPU 有效差值")
	}
	return Percent(total-inactive, total)
}

type CPU struct {
	Usage       Value    `json:"usage"`
	Logical     []Value  `json:"logical"`
	Names       []string `json:"names"`
	Frequency   Value    `json:"frequency"`
	Temperature Value    `json:"temperature"`
}
type Memory struct {
	Usage     Value  `json:"usage"`
	Total     uint64 `json:"total"`
	Used      uint64 `json:"used"`
	Available uint64 `json:"available"`
}
type Disk struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	FileSystem string `json:"fileSystem"`
	Total      uint64 `json:"total"`
	Free       uint64 `json:"free"`
	Usage      Value  `json:"usage"`
	Activity   Value  `json:"activity"`
	Read       Value  `json:"read"`
	Write      Value  `json:"write"`
}
type Network struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Up          bool     `json:"up"`
	Physical    bool     `json:"physical"`
	Addresses   []string `json:"addresses"`
	Received    uint64   `json:"received"`
	Sent        uint64   `json:"sent"`
	Download    Value    `json:"download"`
	Upload      Value    `json:"upload"`
}
type GPU struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	VendorID    string `json:"vendorID"`
	Usage       Value  `json:"usage"`
	Memory      Value  `json:"memory"`
	Dedicated   Value  `json:"dedicated"`
	SharedLimit Value  `json:"sharedLimit"`
	Temperature Value  `json:"temperature"`
}
type Snapshot struct {
	Time       int64     `json:"time"`
	CPU        CPU       `json:"cpu"`
	Memory     Memory    `json:"memory"`
	Disks      []Disk    `json:"disks"`
	Networks   []Network `json:"networks"`
	GPUs       []GPU     `json:"gpus"`
	Uptime     uint64    `json:"uptime"`
	DurationMS float64   `json:"durationMS"`
	Errors     []string  `json:"errors"`
}

func Empty() Snapshot {
	return Snapshot{Time: time.Now().UnixMilli(), CPU: CPU{Usage: Unavailable("%", "等待采样"), Logical: []Value{}, Names: []string{}, Frequency: Unsupported("MHz", "无可靠实时频率接口"), Temperature: Unsupported("°C", "未加载内核驱动或厂商温度接口")}, Memory: Memory{Usage: Unavailable("%", "等待采样")}, Disks: []Disk{}, Networks: []Network{}, GPUs: []GPU{}, Errors: []string{}}
}

type Collector interface {
	Sample() Snapshot
	Close() error
}
