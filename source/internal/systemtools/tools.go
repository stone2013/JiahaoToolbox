package systemtools

import (
	"context"
	"errors"
	"net"
	"regexp"
	"strings"
)

type Tool struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Color       string `json:"color"`
	Category    string `json:"category"`
	Admin       bool   `json:"admin"`
}

var Tools = []Tool{
	{"task-manager", "任务管理器", "查看进程与性能", "monitor", "blue", "监控", false},
	{"device-manager", "设备管理器", "查看与管理硬件设备", "device", "cyan", "管理", true},
	{"disk-management", "磁盘管理", "查看分区与存储设备", "disk", "purple", "管理", true},
	{"system-information", "系统信息", "查看系统完整信息", "info", "blue", "监控", false},
	{"windows-update", "Windows 更新", "检查更新与设置", "refresh", "green", "设置", false},
	{"network-settings", "网络设置", "网络连接与配置", "network", "blue", "设置", false},
	{"power-settings", "电源设置", "电源计划与节能", "shield", "green", "设置", false},
	{"startup-apps", "启动应用", "管理开机启动项", "startup", "orange", "设置", false},
	{"services", "服务管理器", "查看与管理系统服务", "settings", "blue", "管理", true},
	{"event-viewer", "事件查看器", "查看系统与应用日志", "event", "purple", "管理", true},
}

func Find(id string) (Tool, bool) {
	for _, t := range Tools {
		if t.ID == id {
			return t, true
		}
	}
	return Tool{}, false
}

type LaunchResult struct {
	Message        string `json:"message"`
	NeedsElevation bool   `json:"needsElevation,omitempty"`
}
type NetResult struct {
	Kind       string   `json:"kind"`
	Target     string   `json:"target"`
	Success    bool     `json:"success"`
	Message    string   `json:"message"`
	Addresses  []string `json:"addresses,omitempty"`
	DurationMS int64    `json:"durationMS"`
}

var validLabel = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9\-]{0,61}[A-Za-z0-9])?$`)

func ValidateHost(s string) error {
	if net.ParseIP(s) != nil {
		return nil
	}
	if len(s) == 0 || len(s) > 253 || strings.ContainsAny(s, ":/\\ %") {
		return errors.New("请输入合法域名或 IP，不包含端口、路径、空格或命令参数")
	}
	for _, label := range strings.Split(s, ".") {
		if !validLabel.MatchString(label) {
			return errors.New("域名格式无效：每一段须为字母、数字或中间连字符")
		}
	}
	return nil
}
func CheckNetwork(ctx context.Context, kind, target string) (NetResult, error) {
	if e := ValidateHost(target); e != nil {
		return NetResult{}, e
	}
	if kind != "ping" && kind != "dns" {
		return NetResult{}, errors.New("检测类型无效")
	}
	return checkNetwork(ctx, kind, target)
}
