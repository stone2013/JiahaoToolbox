package diagnostics

import (
	"fmt"
	"jiahao-toolbox/internal/hardware"
	"jiahao-toolbox/internal/monitor"
	"math"
	"time"
)

type Check struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Explanation string `json:"explanation"`
	Weight      int    `json:"weight"`
	Penalty     int    `json:"penalty"`
}
type Report struct {
	Time       int64   `json:"time"`
	SampleTime int64   `json:"sampleTime"`
	Score      *int    `json:"score"`
	Coverage   int     `json:"coverage"`
	Checks     []Check `json:"checks"`
	Rules      string  `json:"rules"`
	Summary    string  `json:"summary"`
}

const Rules = "系统信息 10、磁盘空间 30、内存可用量 25、本地网卡链路 15、硬件信息完整性 20。提醒扣该项权重的 25%，警告扣 60%（每项扣分四舍五入为整数）；检测失败/不支持不计入分母。至少 4 项且可靠覆盖率 ≥80%、磁盘/内存两项有有效数据，才计算 round(100×(可靠权重−扣分)/可靠权重)。这只是当前检查评分，不是硬件寿命、损坏、安全或系统更新合规结论。"

func Run(s monitor.Snapshot, h hardware.Inventory, now time.Time) Report {
	r := Report{Time: now.UnixMilli(), SampleTime: s.Time, Rules: Rules, Checks: []Check{}}
	fresh := now.UnixMilli()-s.Time <= 15000 && s.Time <= now.UnixMilli()+1000
	sys := Check{ID: "system", Name: "系统版本", Weight: 10, Status: "failed", Explanation: "系统信息尚未获得；不评估系统生命周期或补丁合规性"}
	if g, ok := h.Groups["system"]; ok && g.Status == "ok" && len(g.Items) > 0 {
		sys.Status = "ok"
		sys.Explanation = "已读取实际操作系统版本；这不等于系统仍受支持或补丁最新"
	}
	r.Checks = append(r.Checks, sys)
	disk := Check{ID: "disk", Name: "磁盘空间", Weight: 30, Status: "failed", Explanation: "没有可靠分区容量读数"}
	worst := 100.0
	valid, invalid := 0, 0
	for _, d := range s.Disks {
		if d.Total == 0 || d.Free > d.Total || !validPercent(d.Usage) {
			invalid++
			continue
		}
		valid++
		free := 100 - *d.Usage.Value
		if free < worst {
			worst = free
		}
	}
	if valid > 0 && invalid == 0 && fresh {
		disk.Status = "ok"
		if worst < 5 {
			disk.Status = "warning"
		} else if worst < 15 {
			disk.Status = "notice"
		}
		disk.Explanation = fmt.Sprintf("已检查 %d 个固定分区；最低剩余空间 %.1f%%。低于 15%% 提醒，低于 5%% 警告。", valid, worst)
	}
	r.Checks = append(r.Checks, disk)
	mem := Check{ID: "memory", Name: "内存状态", Weight: 25, Status: "failed", Explanation: "内存读数不可用或过期"}
	if validPercent(s.Memory.Usage) && s.Memory.Total > 0 && s.Memory.Available <= s.Memory.Total && fresh {
		used := *s.Memory.Usage.Value
		mem.Status = "ok"
		if used >= 95 {
			mem.Status = "warning"
		} else if used >= 85 {
			mem.Status = "notice"
		}
		mem.Explanation = fmt.Sprintf("当前物理内存使用率 %.1f%%。85%% 起提醒，95%% 起警告；单次高占用不代表内存损坏。", used)
	}
	r.Checks = append(r.Checks, mem)
	network := Check{ID: "network", Name: "网络连接", Weight: 15, Status: "failed", Explanation: "网卡链路状态不可用"}
	if len(s.Networks) > 0 && fresh {
		network.Status = "notice"
		network.Explanation = "当前无已连接网卡；离线使用不代表设备故障。未进行互联网测试。"
		for _, n := range s.Networks {
			if n.Up && len(n.Addresses) > 0 {
				network.Status = "ok"
				network.Explanation = "至少一张网卡已连接并有 IP 地址；未测试互联网连通性。"
				break
			}
		}
	}
	r.Checks = append(r.Checks, network)
	info := Check{ID: "hardware", Name: "硬件信息", Weight: 20, Status: "failed", Explanation: "硬件信息正在获取或获取失败"}
	if h.Status != "loading" && h.Status != "failed" {
		count := 0
		for _, k := range []string{"cpu", "gpu", "board", "memory", "disk"} {
			g := h.Groups[k]
			if g.Status == "ok" && len(g.Items) > 0 {
				count++
			}
		}
		if count == 5 {
			info.Status = "ok"
			info.Explanation = "已读取 CPU、GPU、主板、内存条和磁盘型号；不包含健康检测。"
		} else if count >= 3 {
			info.Status = "notice"
			info.Explanation = fmt.Sprintf("5 类关键硬件中已获取 %d 类；缺失可能由驱动、虚拟机或权限限制引起。", count)
		}
	}
	r.Checks = append(r.Checks, info)
	count, penalty := 0, 0
	for i := range r.Checks {
		c := &r.Checks[i]
		switch c.Status {
		case "ok", "notice", "warning":
			r.Coverage += c.Weight
			count++
			if c.Status == "notice" {
				c.Penalty = int(math.Round(float64(c.Weight) * .25))
			}
			if c.Status == "warning" {
				c.Penalty = int(math.Round(float64(c.Weight) * .6))
			}
			penalty += c.Penalty
		}
	}
	if count >= 4 && r.Coverage >= 80 && disk.Status != "failed" && mem.Status != "failed" {
		score := int(math.Round(100 * float64(r.Coverage-penalty) / float64(r.Coverage)))
		r.Score = &score
		r.Summary = "当前可检查项目的状态评分；请同时阅读每项说明。"
	} else {
		r.Summary = "可靠数据不足，暂无评分。请等待采样或查看失败说明。"
	}
	return r
}

func validPercent(v monitor.Value) bool {
	return v.Status == "ok" && v.Value != nil && !math.IsNaN(*v.Value) && !math.IsInf(*v.Value, 0) && *v.Value >= 0 && *v.Value <= 100
}
