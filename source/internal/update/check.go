package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	APIBase      = "https://api.github.com/repos/stone2013/JiahaoToolbox"
	ReleasesPage = "https://github.com/stone2013/JiahaoToolbox/releases"
)

var ErrNoRelease = errors.New("暂无已发布的正式版本。")
var versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z.-]+))?(?:\+[0-9A-Za-z.-]+)?$`)

type Version struct {
	Major, Minor, Patch uint64
	Pre                 []string
}

func ParseVersion(raw string) (Version, error) {
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "v"))
	m := versionPattern.FindStringSubmatch(raw)
	if m == nil {
		return Version{}, fmt.Errorf("版本号不是有效的 SemVer：%q", raw)
	}
	nums := make([]uint64, 3)
	for i := range nums {
		n, e := strconv.ParseUint(m[i+1], 10, 64)
		if e != nil {
			return Version{}, e
		}
		nums[i] = n
	}
	v := Version{Major: nums[0], Minor: nums[1], Patch: nums[2]}
	if m[4] != "" {
		v.Pre = strings.Split(m[4], ".")
		for _, id := range v.Pre {
			if id == "" {
				return Version{}, fmt.Errorf("SemVer 预发布标识为空")
			}
			if isNumeric(id) && len(id) > 1 && id[0] == '0' {
				return Version{}, fmt.Errorf("SemVer 数字预发布标识不能有前导零")
			}
		}
	}
	return v, nil
}
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
func compareNumeric(x, y string) int {
	if len(x) < len(y) {
		return -1
	}
	if len(x) > len(y) {
		return 1
	}
	if x < y {
		return -1
	}
	if x > y {
		return 1
	}
	return 0
}
func Compare(a, b Version) int {
	if a.Major != b.Major {
		if a.Major < b.Major {
			return -1
		}
		return 1
	}
	if a.Minor != b.Minor {
		if a.Minor < b.Minor {
			return -1
		}
		return 1
	}
	if a.Patch != b.Patch {
		if a.Patch < b.Patch {
			return -1
		}
		return 1
	}
	if len(a.Pre) == 0 && len(b.Pre) > 0 {
		return 1
	}
	if len(a.Pre) > 0 && len(b.Pre) == 0 {
		return -1
	}
	for i := 0; i < len(a.Pre) && i < len(b.Pre); i++ {
		x, y := a.Pre[i], b.Pre[i]
		if x == y {
			continue
		}
		xn, yn := isNumeric(x), isNumeric(y)
		if xn && yn {
			return compareNumeric(x, y)
		}
		if xn != yn {
			if xn {
				return -1
			}
			return 1
		}
		if x < y {
			return -1
		}
		return 1
	}
	if len(a.Pre) < len(b.Pre) {
		return -1
	}
	if len(a.Pre) > len(b.Pre) {
		return 1
	}
	return 0
}

type Asset struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}
type Release struct {
	Tag         string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	PublishedAt time.Time `json:"published_at"`
	Prerelease  bool      `json:"prerelease"`
	Draft       bool      `json:"draft"`
	Assets      []Asset   `json:"assets"`
}
type Info struct {
	CurrentVersion string    `json:"currentVersion"`
	LatestVersion  string    `json:"latestVersion,omitempty"`
	PublishedAt    time.Time `json:"publishedAt,omitempty"`
	Notes          string    `json:"notes,omitempty"`
	ReleasePage    string    `json:"releasePage"`
	Assets         []Asset   `json:"assets,omitempty"`
	HasUpdate      bool      `json:"hasUpdate"`
	Channel        string    `json:"channel"`
	Status         string    `json:"status"`
	CheckedAt      time.Time `json:"checkedAt"`
}
type Client struct {
	HTTP    *http.Client
	APIBase string
}

func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 || !strings.EqualFold(req.URL.Host, "api.github.com") || req.URL.Scheme != "https" {
			return http.ErrUseLastResponse
		}
		return nil
	}}, APIBase: APIBase}
}
func (c *Client) Check(ctx context.Context, current string, allowBeta bool) (Info, error) {
	now := time.Now()
	info := Info{CurrentVersion: current, ReleasePage: ReleasesPage, Channel: "stable", CheckedAt: now, Status: "check_failed"}
	cv, e := ParseVersion(current)
	if e != nil {
		return info, e
	}
	base := c.APIBase
	if base == "" {
		base = APIBase
	}
	parsed, e := url.Parse(base)
	localTest := false
	if e == nil && parsed.Scheme == "http" {
		host := parsed.Hostname()
		ip := net.ParseIP(host)
		localTest = host == "localhost" || ip != nil && ip.IsLoopback()
	}
	if e != nil || parsed.Host != "api.github.com" && !localTest || parsed.Scheme != "https" && !localTest || parsed.User != nil {
		return info, errors.New("更新服务地址无效")
	}
	endpoint := strings.TrimRight(base, "/") + "/releases/latest"
	if allowBeta {
		info.Channel = "stable + beta"
		endpoint = strings.TrimRight(base, "/") + "/releases?per_page=20"
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if e != nil {
		return info, e
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "JiahaoToolbox-Updater/4.4")
	hc := c.HTTP
	if hc == nil {
		hc = NewClient().HTTP
	}
	resp, e := hc.Do(req)
	if e != nil {
		return info, fmt.Errorf("无法连接 GitHub 更新服务：%w", e)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		info.Status = "no_release"
		return info, ErrNoRelease
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		if resp.Header.Get("X-RateLimit-Remaining") == "0" {
			info.Status = "rate_limited"
			return info, errors.New("GitHub API 请求次数已达上限，请稍后重试")
		}
		info.Status = "api_error"
		return info, fmt.Errorf("GitHub API 拒绝请求（HTTP %d）", resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		info.Status = "api_error"
		return info, fmt.Errorf("GitHub API 返回 HTTP %d", resp.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if e != nil {
		return info, e
	}
	var releases []Release
	if allowBeta {
		if e = json.Unmarshal(b, &releases); e != nil {
			return info, fmt.Errorf("GitHub Release 响应格式无效：%w", e)
		}
	} else {
		var r Release
		if e = json.Unmarshal(b, &r); e != nil {
			return info, fmt.Errorf("GitHub Release 响应格式无效：%w", e)
		}
		releases = []Release{r}
	}
	var selected *Release
	var selectedV Version
	for i := range releases {
		r := &releases[i]
		if r.Draft || (!allowBeta && r.Prerelease) {
			continue
		}
		rv, pe := ParseVersion(r.Tag)
		if pe != nil {
			continue
		}
		if selected == nil || Compare(rv, selectedV) > 0 {
			selected = r
			selectedV = rv
		}
	}
	if selected == nil {
		info.Status = "no_release"
		return info, ErrNoRelease
	}
	info.LatestVersion = selected.Tag
	info.PublishedAt = selected.PublishedAt
	info.Notes = selected.Body
	if len(info.Notes) > 12000 {
		info.Notes = info.Notes[:12000] + "\n…"
	}
	info.Assets = make([]Asset, 0, len(selected.Assets))
	for _, a := range selected.Assets {
		if strings.HasPrefix(a.Name, "JiahaoToolbox_") && (strings.HasSuffix(a.Name, "_Setup_x64.exe") || strings.HasSuffix(a.Name, "_Portable.zip")) {
			info.Assets = append(info.Assets, Asset{Name: a.Name, Size: a.Size})
		}
	}
	info.HasUpdate = Compare(selectedV, cv) > 0
	info.Status = "up_to_date"
	if info.HasUpdate {
		info.Status = "update_available"
	}
	info.ReleasePage = ReleasesPage + "/tag/" + url.PathEscape(selected.Tag)
	return info, nil
}
