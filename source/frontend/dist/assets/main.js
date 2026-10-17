import { SearchService, highlight, highlightTitle, dispatchSearch } from './services/search.js';
import { api, ApiError } from './services/api.js';
import { esc, bytes, pct, reason, uptime, time, text, html } from './services/format.js';
import { icon } from './components/icons.js';
import { toast, confirm, copyText, showText, download } from './components/dialog.js';
import { mini, plot } from './charts/charts.js';
import { MonitorAnimator } from './services/monitor-animation.js';
const nav = [['overview', '系统总览', 'home'], ['cpu', 'CPU 监控', 'cpu'], ['gpu', 'GPU 监控', 'gpu'], ['memory', '内存监控', 'memory'], ['disk', '磁盘监控', 'disk'], ['network', '网络监控', 'network'], ['speedtest', '网络测速', 'speedtest'], ['disk-bench', '磁盘测速', 'disk'], ['classic-cmd', '经典工具', 'terminal'], ['hardware', '硬件详情', 'hardware'], ['tools', '系统工具', 'tools'], ['diagnostics', '系统诊断', 'shield'], ['settings', '设置', 'settings'], ['about', '关于', 'info']];
let boot, settings, state, hardware, history = [], report = null, page = 'overview', metric = 'cpu';
let online = true, stopped = false, polling = false, timer, pollCount = 0, hardwareTime = 0;
let netTask = null;
let searchIndex = 0;
let searchMatches = [];
let searchQuery = '';
let searchService;
let testProgress, testHistory = [], benchmarkTimer, selectedTestDirectory = '';
let speedSamples = [];
let updateInfo;
const motion = new MonitorAnimator(() => settings?.monitorAnimation ?? 'smooth', () => !settings?.animations || matchMedia('(prefers-reduced-motion: reduce)').matches);
const requestFrame = (cb) => typeof window.requestAnimationFrame === 'function' ? window.requestAnimationFrame(cb) : setTimeout(() => cb(performance.now()), 16);
const cancelFrame = (id) => typeof window.cancelAnimationFrame === 'function' ? window.cancelAnimationFrame(id) : clearTimeout(id);
const sampleFresh = () => !!state && Date.now() - latest().time <= Math.max(15000, settings.intervalMS * 3);
const numberMotion = (id, v, digits = 0, format) => { const el = document.getElementById(id); if (!el)
    return; const target = v?.value; if (target == null || !Number.isFinite(target)) {
    motion.animate('value:' + id, null, 0, () => { el.textContent = pct(v, digits); el.title = reason(v); });
    return;
} const duration = !online || !sampleFresh() || settings.monitorAnimation === 'off' ? 0 : settings.monitorAnimation === 'standard' ? 300 : 600; const actual = format ? format(target) : `${target.toFixed(digits)}%`; el.title = `${reason(v)}${reason(v) ? ' · ' : ''}最近真实采样：${actual}`; motion.animate('value:' + id, target, duration, n => { el.textContent = format ? format(n) : `${n.toFixed(digits)}%`; }); };
function ringMotion(id, v) { const host = document.getElementById(id + '-ring'); if (!host)
    return; if (!host.querySelector('svg'))
    host.innerHTML = '<svg viewBox="0 0 80 80" aria-hidden="true"><circle class="track" cx="40" cy="40" r="31"/><circle class="progress" cx="40" cy="40" r="31" stroke-dasharray="194.78" stroke-dashoffset="194.78"/></svg>'; const circle = host.querySelector('.progress'); if (!circle)
    return; const target = v?.value; if (target == null || !Number.isFinite(target)) {
    motion.animate('ring:' + id, null, 0, () => { circle.setAttribute('stroke-dashoffset', '194.78'); });
    return;
} const duration = id === 'disk' || !online || !sampleFresh() || settings.monitorAnimation === 'off' ? 0 : settings.monitorAnimation === 'standard' ? 300 : 600; motion.animate('ring:' + id, target, duration, n => circle.setAttribute('stroke-dashoffset', (194.78 * (1 - Math.max(0, Math.min(100, n)) / 100)).toFixed(2))); }
const $ = (s) => document.querySelector(s);
const latest = () => state.latest;
const si = () => settings.units === 'si';
const fmt = (n, rate = false) => bytes(n, si(), rate);
const group = (id) => hardware.groups[id];
const item = (id) => group(id)?.items?.[0] ?? {};
const number = (id, key) => { const n = id === 'cpu' && ['NumberOfCores', 'NumberOfLogicalProcessors'].includes(key) ? (group(id)?.items ?? []).reduce((sum, row) => sum + (Number(row[key]) || 0), 0) : Number(item(id)[key]); return Number.isFinite(n) && n > 0 ? n : undefined; };
function names(id, key = 'Name') { const a = group(id)?.items?.map(i => String(i[key] ?? '')).filter(Boolean) ?? []; return a.join(' / ') || '暂不可用'; }
function diskID() { return settings.disk || latest().disks.find(d => d.id === 'C:')?.id || latest().disks[0]?.id || ''; }
function networkID() { return settings.interface || latest().networks.find(n => n.up && n.physical)?.id || latest().networks.find(n => n.up)?.id || latest().networks[0]?.id || ''; }
function disk() { return latest().disks.find(d => d.id === diskID()); }
function network() { return latest().networks.find(n => n.id === networkID()); }
function chartOptions() { return { diskID: diskID(), networkID: networkID(), si: si(), interval: settings.intervalMS }; }
function fieldCard(label, value, sub, ico, valueID = '', subID = '') { return `<section class="card metric-box"><div class="label">${icon(ico)}${esc(label)}</div><div class="value"${valueID ? ` id="${esc(valueID)}"` : ''}>${esc(value)}</div><div class="sub"${subID ? ` id="${esc(subID)}"` : ''}>${esc(sub)}</div></section>`; }
function head(title, subtitle, actions = '') { return `<header class="page-head"><div><h1>${esc(title)}</h1><p>${esc(subtitle)}</p></div><div class="page-head-actions">${actions}</div></header>`; }
function button(label, action, ico = '', primary = false) { return `<button class="button${primary ? ' primary' : ''}" data-action="${action}">${ico ? icon(ico) : ''}${esc(label)}</button>`; }
function unavailable(message) { return `<div class="empty">${icon('info')}<p>${esc(message)}</p></div>`; }
const classicBAT = '@echo off\r\ncolor a\r\ntitle Jiahao Classic CMD\r\ncmd /k\r\n';
const legacyBAT = '@echo off\r\ncolor a\r\ndir /s\r\npause\r\n';
function speedTestPage() { return head('网络测速', 'Cloudflare Anycast 网络测速 · 独立测下载、上传与 HTTP 延迟探测') + `<div class="notice">${icon('privacy')}测速为可选联网功能。需连接 speed.cloudflare.com，预计下载约 25 MB、上传约 10 MB；不会读取或上传个人文件。结果受 Wi-Fi、VPN、后台流量和服务节点路由影响。丢包项按 HTTP 探测失败率估算，不代表 ICMP/UDP 丢包率。</div><section class="card speed-test-card"><div class="speed-gauge"><div class="speed-gauge-inner"><span id="speed-current">—</span><small id="speed-unit">Mbps</small><strong id="speed-stage">准备就绪</strong></div></div><div class="speed-progress"><div class="progress-track"><span id="speed-progress-bar" style="width:0%"></span></div><div class="progress-meta"><span id="speed-progress-label">尚未开始</span><span id="speed-progress-percent">0%</span></div></div><canvas id="speed-chart" class="speed-chart" aria-label="实时下载上传速度"></canvas><div class="speed-actions">${button('开始测速', 'speed-start', 'play', true)}${button('取消', 'cancel-benchmark', 'close')}<span id="speed-server">服务器：Cloudflare speed.cloudflare.com</span></div><div class="speed-results"><div class="speed-result"><small>下载</small><strong id="speed-download">—</strong><span>Mbps</span></div><div class="speed-result"><small>上传</small><strong id="speed-upload">—</strong><span>Mbps</span></div><div class="speed-result"><small>延迟</small><strong id="speed-latency">—</strong><span>ms</span></div><div class="speed-result"><small>抖动</small><strong id="speed-jitter">—</strong><span>ms</span></div><div class="speed-result"><small>HTTP 探测失败</small><strong id="speed-loss">—</strong><span>%</span></div></div><p class="muted speed-result-note" id="speed-note">每次测试独立进行；完成后显示服务器和时间。</p></section><section class="card test-history"><div class="card-heading"><h2>测速历史</h2><span class="badge">仅保存在本机</span></div><div id="speed-history-list"></div></section>`; }
function diskBenchPage() { const disks = latest().disks; const options = disks.map(d => `<option value="${esc(d.id)}">${esc(d.id)} · ${esc(d.name)} · 可用 ${esc(fmt(d.free))}</option>`).join(''); return head('磁盘测速', '对所选文件夹执行真实文件 I/O 测试 · 不访问原始磁盘设备') + `<div class="notice">${icon('warning')}将写入独立临时测试文件并在结束时删除。快速模式约写入 64 MiB，标准模式约 256 MiB；SSD 写入会产生少量写入损耗。高级模式最大 1 GiB，不默认启用。Windows 缓冲与系统文件缓存会影响结果，详情会随结果显示。请避免在测试期间运行其他磁盘密集任务。</div><section class="card benchmark-panel"><div class="toolbar"><label for="diskbench-volume">目标分区</label><select id="diskbench-volume">${options || '<option value="">当前没有可用固定分区</option>'}</select><button class="button" data-action="disk-browse">${icon('folder')}选择测试文件夹</button></div><div class="selected-path"><span>测试位置</span><strong id="diskbench-directory">${selectedTestDirectory ? esc(selectedTestDirectory) : '尚未选择文件夹'}</strong></div><div class="bench-mode-grid"><label class="mode-card"><input type="radio" name="diskbench-mode" value="quick" checked><span><strong>快速测试</strong><small>64 MiB · QD1 · 随机测试各 2 秒</small></span></label><label class="mode-card"><input type="radio" name="diskbench-mode" value="standard"><span><strong>标准测试</strong><small>256 MiB · QD1 · 随机测试各 4 秒</small></span></label><label class="mode-card"><input type="radio" name="diskbench-mode" value="advanced"><span><strong>高级测试</strong><small>自行设置文件大小、队列和时长</small></span></label></div><div class="bench-advanced" id="diskbench-advanced" hidden><label>文件大小 MiB <input id="diskbench-size" type="number" min="32" max="1024" step="32" value="256"></label><label>队列深度 <select id="diskbench-queue"><option>1</option><option>2</option><option>4</option><option>8</option></select></label><label>随机测试秒数 <input id="diskbench-duration" type="number" min="1" max="20" value="4"></label></div><div class="progress-track disk-progress"><span id="diskbench-progress-bar" style="width:0%"></span></div><div class="progress-meta"><span id="diskbench-progress-label">等待开始</span><span id="diskbench-progress-percent">0%</span></div><div class="speed-actions">${button('开始磁盘测试', 'disk-start', 'play', true)}${button('取消测试', 'cancel-benchmark', 'close')}${button('检查并清理本工具遗留测试文件', 'disk-recover', 'trash')}</div><div id="diskbench-results" class="diskbench-results"></div></section><section class="card test-history"><div class="card-heading"><h2>磁盘测速历史</h2><span class="badge">最多保存 30 条</span></div><div id="disk-history-list"></div></section>`; }
function classicCmdPage() { return head('经典工具 · 绿色 CMD', '保留初代嘉豪工具箱的绿色终端风格 · 不需要管理员权限') + `<section class="card classic-terminal"><div class="terminal-top"><span></span><span></span><span></span><strong>Jiahao Classic CMD</strong></div><div class="terminal-content"><p>C:\\Jiahao&gt; color a</p><p>C:\\Jiahao&gt; <span class="terminal-caret">_</span></p><pre id="classic-code">${esc(classicBAT)}</pre><p class="terminal-hint">命令预览：绿色终端；生成的 BAT 默认只保存，不会自动运行。</p></div><div class="classic-actions">${button('打开绿色 CMD', 'cmd-open', 'terminal', true)}${button('运行经典 dir /s', 'cmd-scan', 'search')}${button('复制代码', 'cmd-copy', 'copy')}${button('保存 Jiahao_Classic_CMD.bat', 'cmd-save-bat', 'download')}${button('生成初代嘉豪工具箱 BAT', 'cmd-save-legacy', 'download')}</div><p class="muted">目录扫描只作用于你在系统文件夹选择器中明确选定的目录。扫描会在独立 CMD 窗口运行；可按 Ctrl+C 或关闭终端窗口停止。生成的初代 BAT 不会自动执行。</p></section>`; }
function renderTestHistory() { const speed = document.getElementById('speed-history-list'), disk = document.getElementById('disk-history-list'); const speeds = testHistory.filter(x => x.kind === 'speed').slice(0, 10), disks = testHistory.filter(x => x.kind === 'disk').slice(0, 10); if (speed)
    speed.innerHTML = speeds.length ? speeds.map(x => { const r = x.result; return `<div class="history-row"><strong>${new Date(x.time).toLocaleString()}</strong><span>↓ ${Number(r.downloadMbps).toFixed(1)} Mbps</span><span>↑ ${Number(r.uploadMbps).toFixed(1)} Mbps</span><span>${esc(r.server)}</span></div>`; }).join('') : '<p class="muted">完成测速后会在此显示结果。</p>'; if (disk)
    disk.innerHTML = disks.length ? disks.map(x => { const r = x.result; return `<div class="history-row"><strong>${new Date(x.time).toLocaleString()}</strong><span>读取 ${Number(r.sequentialReadMBps).toFixed(1)} MB/s</span><span>写入 ${Number(r.sequentialWriteMBps).toFixed(1)} MB/s</span><span>${esc(r.directory)}</span></div>`; }).join('') : '<p class="muted">完成磁盘测试后会在此显示结果。</p>'; }
function drawSpeedCurve() { const c = document.getElementById('speed-chart'); if (!c)
    return; const r = c.getBoundingClientRect(), dpr = window.devicePixelRatio || 1; c.width = Math.max(1, Math.floor(r.width * dpr)); c.height = Math.max(1, Math.floor(r.height * dpr)); const x = c.getContext('2d'); if (!x)
    return; x.scale(dpr, dpr); const w = r.width, h = r.height; x.clearRect(0, 0, w, h); x.strokeStyle = 'rgba(110,150,195,.22)'; x.lineWidth = 1; for (let i = 1; i < 4; i++) {
    x.beginPath();
    x.moveTo(0, h * i / 4);
    x.lineTo(w, h * i / 4);
    x.stroke();
} if (speedSamples.length < 2)
    return; const max = Math.max(...speedSamples, 1); x.strokeStyle = '#56a9ff'; x.lineWidth = 2; x.beginPath(); speedSamples.forEach((v, i) => { const px = i / (speedSamples.length - 1) * w, py = h - 6 - Math.min(1, v / max) * (h - 12); if (i)
    x.lineTo(px, py);
else
    x.moveTo(px, py); }); x.stroke(); }
function renderBenchmarkStatus() { if (!testProgress)
    return; const job = testProgress, bar = job.kind === 'speed' ? document.getElementById('speed-progress-bar') : document.getElementById('diskbench-progress-bar'), label = job.kind === 'speed' ? document.getElementById('speed-progress-label') : document.getElementById('diskbench-progress-label'), percent = job.kind === 'speed' ? document.getElementById('speed-progress-percent') : document.getElementById('diskbench-progress-percent'); if (bar)
    bar.style.width = `${Math.max(0, Math.min(100, job.progress * 100))}%`; if (label)
    label.textContent = job.status === 'error' ? job.error || job.stage : job.stage; if (percent)
    percent.textContent = `${Math.round(job.progress * 100)}%`; if (job.kind === 'speed') {
    text('speed-stage', job.stage);
    text('speed-current', job.currentMbps ? job.currentMbps.toFixed(1) : '—');
    if (job.currentMbps > 0) {
        speedSamples.push(job.currentMbps);
        if (speedSamples.length > 100)
            speedSamples.shift();
        drawSpeedCurve();
    }
    if (job.result) {
        const r = job.result;
        text('speed-download', Number(r.downloadMbps).toFixed(1));
        text('speed-upload', Number(r.uploadMbps).toFixed(1));
        text('speed-latency', Number(r.latencyMS).toFixed(1));
        text('speed-jitter', Number(r.jitterMS).toFixed(1));
        text('speed-loss', Number(r.lossPercent).toFixed(1));
        text('speed-server', `服务器：${r.server} · ${r.location}`);
        text('speed-note', `${new Date(String(r.time)).toLocaleString()} · 流量约 ${(Number(r.trafficBytes) / 1e6).toFixed(1)} MB · ${r.note}`);
    }
}
else if (job.kind === 'disk' && job.result) {
    const r = job.result, el = document.getElementById('diskbench-results');
    if (el)
        el.innerHTML = `<div class="speed-results"><div class="speed-result"><small>顺序读取</small><strong>${Number(r.sequentialReadMBps).toFixed(1)}</strong><span>MB/s</span></div><div class="speed-result"><small>顺序写入</small><strong>${Number(r.sequentialWriteMBps).toFixed(1)}</strong><span>MB/s</span></div><div class="speed-result"><small>4K 随机读取</small><strong>${Number(r.randomReadIOPS).toFixed(0)}</strong><span>IOPS</span></div><div class="speed-result"><small>4K 随机写入</small><strong>${Number(r.randomWriteIOPS).toFixed(0)}</strong><span>IOPS</span></div></div><p class="muted">文件 ${Number(r.fileSizeBytes) / 1024 / 1024} MiB · 块 ${r.blockSizeBytes} B · QD ${r.queueDepth} · ${r.durationSeconds}s · ${esc(r.cachePolicy)} · ${esc(r.cleanup)}</p><p class="muted">${esc(r.note)}</p>`;
} }
async function refreshTestHistory() { testHistory = await api('tests/history'); renderTestHistory(); }
function scheduleTestPoll() { if (benchmarkTimer)
    clearTimeout(benchmarkTimer); benchmarkTimer = setTimeout(async () => { try {
    testProgress = await api('tests/status');
    renderBenchmarkStatus();
    if (['complete', 'error', 'canceled'].includes(testProgress.status))
        await refreshTestHistory();
}
catch { } if (testProgress?.status === 'running' || testProgress?.status === 'canceling')
    scheduleTestPoll(); }, 250); }
function toolsMarkup(all = false) { return boot.tools.map(t => `<button class="tool-tile" ${boot.platform !== 'windows' ? 'disabled aria-label="仅 Windows 可用"' : ''} data-tool="${t.id}" title="${esc(t.description)}${t.admin ? '；可能需要管理员权限' : ''}"><span class="tool-icon ${t.color}">${icon(t.icon)}</span><span class="tool-text"><span class="tool-name">${esc(t.name)}</span><span class="tool-desc">${esc(t.description)}</span></span></button>`).join(''); }
function chartMarkup(detailed = false, allowTabs = true) { return `<section class="card chart-card${detailed ? ' detail-chart' : ''}"><div class="chart-top"><h2 id="chart-title">${detailed ? '实时监控' : '实时监控'}</h2><div class="chart-actions">${allowTabs ? `<div class="segmented" role="tablist" aria-label="监控指标">${['cpu', 'memory', 'disk', 'network'].map((m, i) => `<button role="tab" aria-selected="${metric === m}" class="${metric === m ? 'active' : ''}" data-metric="${m}">${['CPU', '内存', '磁盘', '网络'][i]}</button>`).join('')}</div>` : ''}<button id="pause-button" class="icon-button" data-action="pause" aria-label="暂停采样" title="暂停/恢复采样">${icon('pause')}</button></div></div><div class="chart-meta"><span id="chart-caption">最近 60 秒 · 真实采样</span><span id="chart-time">等待数据</span></div><div class="chart-content"><div id="main-plot" class="plot"></div><div id="main-legend" class="plot-legend"></div></div></section>`; }
function shell() {
    document.getElementById('app').innerHTML = `
 <header class="topbar" id="titlebar">
 <div class="brand" data-drag><img src="/assets/assets/logo.svg" alt="" width="35" height="39"><span class="brand-name">嘉豪工具箱</span><span class="brand-version">v${esc(boot.version)}</span></div>
 <div class="search-wrap"><label class="global-search">${icon('search')}<input id="global-search" type="search" role="combobox" aria-autocomplete="list" aria-controls="search-results" aria-expanded="false" placeholder="搜索中文、拼音、首字母或工具…" aria-label="搜索功能、工具或设置" autocomplete="off"><kbd>Ctrl K</kbd></label><div id="search-results" class="search-results" role="listbox" hidden></div></div>
 <div class="top-actions"><div class="theme-switch" aria-label="主题切换"><button data-theme="light" title="浅色主题" aria-label="浅色主题">${icon('sun')}</button><button data-theme="dark" title="深色主题" aria-label="深色主题">${icon('moon')}</button></div><button class="icon-button top-settings" data-route="settings" title="设置" aria-label="设置">${icon('settings')}</button><div class="window-actions"><button data-window="minimize" aria-label="最小化" title="${boot.native ? '最小化' : '浏览器模式：使用浏览器窗口按钮'}" ${boot.native ? '' : 'disabled'}>${icon('minimize')}</button><button data-window="maximize" aria-label="最大化或还原" title="${boot.native ? '最大化或还原' : '浏览器模式：使用浏览器窗口按钮'}" ${boot.native ? '' : 'disabled'}>${icon('maximize')}</button><button class="window-close" data-action="exit" aria-label="退出工具箱" title="退出工具箱">${icon('close')}</button></div></div>
 </header><div class="app-body"><aside class="sidebar"><nav class="nav-list" aria-label="主导航">${nav.map(n => `<button class="nav-item" data-route="${n[0]}" title="${n[1]}" aria-label="${n[1]}">${icon(n[2])}<span class="nav-label">${n[1]}</span></button>`).join('')}</nav><div class="sidebar-footer"><div class="status-box"><div class="status-top"><span id="status-dot" class="dot live-dot"></span><span id="status-text" class="status-text">连接本地服务</span></div><div class="status-version">嘉豪工具箱 v${esc(boot.version)} · Beta</div></div></div></aside><main class="main" id="main" tabindex="-1"></main></div>`;
    applyTheme();
    bindEvents();
}
function dashboard() {
    return `<div class="dashboard">
 <section class="hero"><div class="hero-content"><div class="hero-title">${icon('monitor')}<div><h1>系统总览</h1><p>实时监控 · 硬件信息 · 系统状态</p></div></div><div class="system-meta"><strong id="os-caption">正在读取系统信息…</strong><dl><dt>版本</dt><dd id="os-version">—</dd><dt>设备名称</dt><dd id="os-host">—</dd><dt>运行时间</dt><dd id="os-uptime">—</dd><dt>系统类型</dt><dd id="os-arch">—</dd></dl></div></div><div class="hero-brand">${icon('monitor')}<div><strong>嘉豪工具箱</strong><small>v${esc(boot.version)}</small><p>专业 · 高效 · 可靠</p></div></div><div class="hero-corner">${icon('privacy')}核心监控离线运行</div></section>
 <section class="stat-grid" aria-label="实时资源概览">
 ${[['cpu', 'CPU', 'blue'], ['memory', '内存', 'green'], ['disk', '磁盘', 'purple']].map(([id, title, color]) => `<article class="card stat-card ${color}"><button class="stat-title" data-route="${id}" aria-label="打开${title}监控"><span class="icon-disc">${icon(id)}</span><span>${title}</span></button><div class="stat-body"><div class="stat-ring" id="${id}-ring"></div><div><div id="${id}-number" class="stat-number">—</div><div class="stat-label">${id === 'disk' ? '容量使用率' : '使用率'}</div></div></div><div id="${id}-meta" class="stat-meta"></div><div id="${id}-spark" class="spark"></div>${id === 'disk' ? '<span class="disk-spark-caption">曲线：磁盘活动率</span>' : ''}</article>`).join('')}
 <article class="card stat-card blue"><button class="stat-title" data-route="network" aria-label="打开网络监控"><span class="icon-disc">${icon('network')}</span><span>网络</span></button><div class="network-rates"><span>${icon('up')} <span id="net-up">—</span></span><span>${icon('down')} <span id="net-down">—</span></span></div><div id="network-spark" class="spark network-spark"></div><div id="network-ip" class="network-ip">等待网卡信息</div></article>
 </section>
 <div class="dashboard-lower"><div class="left-stack">${chartMarkup()}<section class="card quick-card"><div class="quick-heading"><h2>快捷工具</h2><button class="text-button" data-route="tools">全部工具 ${icon('chevron')}</button></div><div class="quick-grid">${toolsMarkup()}</div></section></div>
 <div class="right-stack"><section class="card hardware-card"><div class="card-heading"><h2>硬件信息</h2><button class="text-button" data-route="hardware">详细 ${icon('chevron')}</button></div><div id="hardware-summary" class="hardware-list"></div></section><section class="card diagnostic-card"><div class="diagnostic-top"><h2>系统诊断</h2><button class="button primary" data-action="diagnose">开始诊断</button></div><div class="diagnostic-content"><div id="checks-mini" class="check-mini"></div><div id="mini-score"></div></div><div class="diagnostic-foot"><button class="text-button" data-route="diagnostics">查看详细报告 ${icon('chevron')}</button><span>只读检查</span></div></section></div></div></div>`;
}
function mount() {
    stopChart();
    motion.reset();
    motion.setVisible(!document.hidden);
    const main = document.getElementById('main');
    document.querySelectorAll('.nav-item').forEach(el => { const active = el.dataset.route === page; el.classList.toggle('active', active); if (active)
        el.setAttribute('aria-current', 'page');
    else
        el.removeAttribute('aria-current'); });
    const preview = boot.platform !== 'windows' ? '<div class="preview-notice">Linux 开发预览 · 以下为当前容器的真实数据，不是你的 Windows 电脑数据；Windows 工具入口在此环境不可用。</div>' : '';
    let content = '';
    switch (page) {
        case 'overview':
            content = dashboard();
            break;
        case 'cpu':
            content = head('CPU 监控', '总体与逻辑处理器负载 · 统一采样服务 · 最近 60 秒') + '<div id="cpu-metrics" class="page-grid"></div>' + chartMarkup(true, false) + `<section class="card"><div class="card-heading"><h2>各逻辑处理器</h2><span class="badge" id="logical-count">等待 PDH</span></div><div id="core-grid" class="core-grid"></div></section><div class="notice" style="margin-top:16px">${icon('info')}温度只在可靠接口支持时提供。本版不安装内核驱动，不以主板 ACPI 热区代替 CPU 温度。</div>`;
            break;
        case 'memory':
            content = head('内存监控', '物理内存使用情况 · 已用与可用量使用同一套 Windows API 定义') + '<div id="memory-metrics" class="page-grid"></div>' + chartMarkup(true, false) + '<section class="card info-section"><h2>' + icon('memory') + '内存条信息</h2><div id="memory-details"></div></section>';
            break;
        case 'gpu':
            content = head('GPU 监控', 'DXGI 适配器识别 · WDDM 性能计数器 · 不依赖 NVIDIA 专属工具') + `<div class="notice">${icon('info')}负载按同一引擎的进程占用求和，再取最忙引擎；不同引擎不相加。专用显存容量来自 64 位 DXGI 字段，共享上限不是已使用量。缺失指标显示不可用。</div><div id="gpu-list" class="network-list" style="margin-top:16px"></div>${chartMarkup(true, false)}<div id="gpu-details" class="details-grid"></div>`;
            break;
        case 'disk':
            content = head('磁盘监控', '容量占用 ≠ 磁盘活动率 · 容量每 5 秒刷新 · 实时速率以 B/s 为基础单位') + `<div class="toolbar"><label for="disk-select">监控分区</label><select id="disk-select"></select><div class="segmented"><button data-diskmetric="disk" class="active">活动率</button><button data-diskmetric="diskio">读写速率</button></div></div><div id="disk-list" class="disk-list"></div>${chartMarkup(true, false)}<section class="card info-section"><h2>${icon('disk')}物理磁盘</h2><div id="disk-details"></div></section><div class="notice" style="margin-top:16px">${icon('info')}仅对固定分区读取容量，避免离线网络盘或可移动设备阻塞。SSD/HDD 仅按 Storage CIM 的 MediaType 分类；SMART 在本版未接入，不作健康推断。</div>`;
            break;
        case 'network':
            content = head('网络监控', '按单一网卡计算流量差值，不将虚拟网卡和物理网卡重复累加') + `<div class="toolbar"><label for="network-select">监控网卡</label><select id="network-select"></select></div>${chartMarkup(true, false)}<div id="network-list" class="network-list"></div><section class="card network-check"><h2>连接检查</h2><p>这是手动联网测试。DNS 会查询系统配置的解析器；Ping 会向你指定的目标发送 ICMP 请求。核心监控不会主动进行此类测试。</p><form id="network-form"><input id="network-target" aria-label="检测目标域名或 IP" placeholder="输入域名或 IP，例如 192.168.1.1" maxlength="253" required><button class="button" type="submit" value="dns" name="kind">DNS 检查</button><button class="button primary" type="submit" value="ping" name="kind">Ping 检查</button><button class="button" type="button" data-action="cancel-network" id="cancel-network" disabled>取消</button></form><div id="network-result"></div></section>`;
            break;
        case 'speedtest':
            content = speedTestPage();
            break;
        case 'disk-bench':
            content = diskBenchPage();
            break;
        case 'classic-cmd':
            content = classicCmdPage();
            break;
        case 'hardware':
            content = head('硬件详情', '只读取本机信息 · 未采集序列号与 UUID · 缺失项保留原因', button('刷新', 'refresh-hardware', 'refresh') + button('取消查询', 'cancel-hardware', 'close') + button('复制', 'copy-hardware', 'copy') + button('导出 JSON', 'export-hardware', 'download', true)) + `<div class="toolbar"><input id="hardware-filter" type="search" placeholder="搜索型号、厂商、字段或设备类型…" aria-label="搜索硬件详情"><span id="inventory-state" class="badge"></span></div><div id="hardware-details" class="details-grid"></div>`;
            break;
        case 'tools':
            content = head('系统工具', '全部入口共用白名单后端 · 不执行一键优化或未确认的修改') + `<div class="notice">${icon('shield')}管理类工具可能请求 Windows 管理员权限。工具箱只负责打开系统入口；外部工具中的修改由你决定。</div><div class="quick-grid all-tools">${toolsMarkup(true)}</div>`;
            break;
        case 'diagnostics':
            content = head('系统诊断', '依据当前有效数据进行只读检查，不扫描驱动或修改系统', button('开始诊断', 'diagnose', 'shield', true) + button('导出报告', 'export-report', 'download')) + `<div class="diag-layout"><section id="diagnostic-list" class="card diag-full"></section><aside class="card diag-score"><div id="full-score"></div><p id="diag-summary"></p><p id="diag-coverage"></p></aside></div><section class="card rules"><h3>透明评分规则</h3><p id="score-rules">系统信息 10、磁盘空间 30、内存状态 25、网卡链路 15、硬件信息 20。提醒扣该项权重的 25%，警告扣 60%。检测失败与不支持不计入分母；至少 4 项、覆盖率 ≥80%，且内存和磁盘读数有效时才计算评分。</p><p style="margin-top:8px">评分仅代表本次可检查项目的状态，不是设备寿命、系统安全、补丁合规性或硬件损坏判定。</p></section>`;
            break;
        case 'settings':
            content = settingsPage();
            break;
        case 'about':
            content = aboutPage();
            break;
    }
    main.innerHTML = preview + `<div class="page">${content}</div>`;
    main.scrollTop = 0;
    hardwareTime = 0;
    updateLive();
    updateHardware();
    updateDiagnostics();
    renderSettingsState();
    observePlot();
    if (page === 'disk-bench') {
        const s = document.getElementById('diskbench-volume');
        if (s && diskID())
            s.value = diskID();
        document.querySelectorAll('input[name="diskbench-mode"]').forEach(el => el.addEventListener('change', () => { const advanced = document.getElementById('diskbench-advanced'); if (advanced)
            advanced.toggleAttribute('hidden', el.value !== 'advanced' || !el.checked); }));
    }
    if (page === 'speedtest' || page === 'disk-bench') {
        void refreshTestHistory();
        void api('tests/status').then(j => { testProgress = j; if (j.status === 'running' || j.status === 'canceling')
            scheduleTestPoll(); renderBenchmarkStatus(); }).catch(() => { });
    }
}
function navigate(id) { if (!nav.some(n => n[0] === id))
    return; page = id; metric = page === 'gpu' ? 'gpu' : page === 'memory' ? 'memory' : page === 'disk' ? 'disk' : page === 'network' ? 'network' : 'cpu'; closeSearch(); mount(); const main = document.getElementById('main'); main.focus({ preventScroll: true }); }
function applyTheme() { const theme = settings.theme === 'system' ? (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light') : settings.theme; document.documentElement.dataset.theme = theme; document.documentElement.classList.toggle('no-animation', !settings.animations); document.querySelectorAll('[data-theme]').forEach(el => { if (el.tagName === 'BUTTON') {
    el.classList.toggle('active', el.dataset.theme === theme);
    el.setAttribute('aria-pressed', String(el.dataset.theme === theme));
} }); }
function updateLive() {
    if (!state)
        return;
    const s = latest(), d = disk(), n = network();
    text('status-text', stopped ? '已退出' : !online ? '连接中断' : state.paused ? '采样已暂停' : Date.now() - s.time > Math.max(15000, settings.intervalMS * 3) ? '采样延迟' : boot.platform !== 'windows' ? '开发预览' : s.cpu.usage.value == null || s.memory.usage.value == null ? '部分数据不可用' : '运行正常');
    const dot = document.getElementById('status-dot');
    if (dot)
        dot.style.color = !online ? '#f78383' : state.paused ? '#ddb16d' : 'var(--green)';
    if (page === 'overview') {
        text('os-uptime', uptime(s.uptime));
        for (const [id, v] of [['cpu', s.cpu.usage], ['memory', s.memory.usage], ['disk', d?.usage]]) {
            numberMotion(id + '-number', v);
            ringMotion(id, v);
            html(id + '-spark', mini(history, id, chartOptions(), id));
        }
        const logical = number('cpu', 'NumberOfLogicalProcessors') ?? s.cpu.logical.length;
        const cores = number('cpu', 'NumberOfCores');
        html('cpu-meta', `${s.cpu.frequency.value == null ? '实时频率不可用' : (s.cpu.frequency.value / 1000).toFixed(2) + ' GHz'}<br>${cores ? cores + ' 核 / ' : ''}${logical ? logical + ' 逻辑处理器' : '逻辑处理器读取中'}`);
        text('memory-meta', s.memory.usage.value == null ? '容量暂不可用' : `${fmt(s.memory.used)} / ${fmt(s.memory.total)}`);
        text('disk-meta', d && d.usage.value != null ? `${fmt(d.total - d.free)} / ${fmt(d.total)}` : '未找到选中分区');
        numberMotion('net-up', n?.upload, 1, v => fmt(v, true));
        numberMotion('net-down', n?.download, 1, v => fmt(v, true));
        html('network-spark', mini(history, 'network', chartOptions(), 'network'));
        const ip = n?.addresses.find(a => !a.includes(':')) || n?.addresses[0];
        text('network-ip', n ? `${n.name}  ·  ${ip || '无 IP 地址'}` : '暂无可用网卡');
    }
    if (page === 'cpu') {
        const count = s.cpu.logical.length;
        html('cpu-metrics', fieldCard('总使用率', pct(s.cpu.usage, 1), names('cpu'), 'cpu', 'cpu-detail-usage') + fieldCard('实时频率', s.cpu.frequency.value == null ? '暂不可用' : (s.cpu.frequency.value / 1000).toFixed(2) + ' GHz', s.cpu.frequency.value == null ? reason(s.cpu.frequency) : '来自 PDH Processor Frequency', 'clock') + fieldCard('温度', s.cpu.temperature.value == null ? '不支持' : s.cpu.temperature.value.toFixed(1) + ' °C', reason(s.cpu.temperature), 'info'));
        numberMotion('cpu-detail-usage', s.cpu.usage, 1);
        text('logical-count', count ? `${count} 个逻辑处理器` : '暂不可用');
        html('core-grid', count ? s.cpu.logical.map((v, i) => `<div class="core">逻辑处理器 ${esc(s.cpu.names[i] ?? i)}<strong>${pct(v)}</strong><div class="bar"><span style="width:${v.value ?? 0}%"></span></div></div>`).join('') : unavailable('未取得每逻辑处理器负载。总体负载仍按可用接口显示。'));
    }
    if (page === 'gpu') {
        html('gpu-list', s.gpus.length ? s.gpus.map((g, i) => `<article class="card device-card"><div class="device-title">${icon('gpu')}<h3>${esc(g.name)}</h3><span class="badge">${esc(g.vendorID)}</span></div><dl class="kv"><dt>最忙引擎负载</dt><dd id="gpu-usage-${i}" title="${esc(reason(g.usage))}">${pct(g.usage, 1)}</dd><dt>专用显存容量</dt><dd>${fmt(g.dedicated.value)}</dd><dt>专用显存已用</dt><dd title="${esc(reason(g.memory))}">${fmt(g.memory.value)}</dd><dt>共享内存上限</dt><dd>${fmt(g.sharedLimit.value)}（不是已用量）</dd><dt>温度</dt><dd title="${esc(reason(g.temperature))}">${g.temperature.value == null ? '未接入' : g.temperature.value + ' °C'}</dd></dl></article>`).join('') : unavailable('未取得 DXGI 适配器信息；下面仍会显示可读取的 CIM 驱动信息。'));
        s.gpus.forEach((g, i) => numberMotion(`gpu-usage-${i}`, g.usage, 1));
    }
    if (page === 'memory') {
        html('memory-metrics', fieldCard('总物理内存', s.memory.usage.value == null ? '暂不可用' : fmt(s.memory.total), '系统可见的物理内存容量', 'memory') + fieldCard('已使用', s.memory.usage.value == null ? '暂不可用' : fmt(s.memory.used), pct(s.memory.usage, 1) + ' · 总量减去可用量', 'memory', '', 'memory-detail-pct') + fieldCard('可用内存', s.memory.usage.value == null ? '暂不可用' : fmt(s.memory.available), '可立即提供给应用的物理内存', 'memory'));
        numberMotion('memory-detail-pct', s.memory.usage, 1, n => `${n.toFixed(1)}% · 总量减去可用量`);
    }
    if (page === 'disk') {
        updateSelectors();
        html('disk-list', s.disks.length ? s.disks.map((d, i) => `<article class="card device-card"><div class="device-title">${icon('disk')}<h3>${esc(d.name)}</h3><span class="badge">${esc(d.fileSystem || '文件系统未知')}</span></div><div class="bar"><span style="width:${d.usage.value ?? 0}%;background:var(--purple)"></span></div><div class="device-stats"><div>容量使用率<strong>${pct(d.usage, 1)}</strong></div><div>磁盘活动率<strong id="disk-activity-${i}" title="${esc(reason(d.activity))}">${pct(d.activity, 1)}</strong></div><div>剩余 / 总量<strong style="font-size:14px">${fmt(d.free)} / ${fmt(d.total)}</strong></div></div><div class="muted" style="font-size:11px">读取 ${fmt(d.read.value, true)}　写入 ${fmt(d.write.value, true)}</div></article>`).join('') : unavailable('没有可读取的固定分区'));
        s.disks.forEach((d, i) => numberMotion(`disk-activity-${i}`, d.activity, 1));
    }
    if (page === 'network') {
        updateSelectors();
        html('network-list', s.networks.length ? s.networks.map((n, i) => `<article class="card device-card"><div class="device-title">${icon('network')}<h3>${esc(n.name)}</h3><span class="badge ${n.up ? 'status-ok' : ''}">${n.up ? '已连接' : '未连接'}</span></div><p class="muted" style="font-size:11px;margin-bottom:12px;overflow-wrap:anywhere">${esc(n.description)}</p><dl class="kv"><dt>IP 地址</dt><dd>${n.addresses.map(esc).join('<br>') || '未分配'}</dd><dt>下载 / 上传</dt><dd><span id="network-detail-down-${i}">${fmt(n.download.value, true)}</span> / <span id="network-detail-up-${i}">${fmt(n.upload.value, true)}</span></dd><dt>接口累计接收</dt><dd>${fmt(n.received)}</dd><dt>接口累计发送</dt><dd>${fmt(n.sent)}</dd><dt>统计口径</dt><dd>自接口计数器初始化以来，非本次会话</dd></dl></article>`).join('') : unavailable('没有可读取的网卡'));
        s.networks.forEach((n, i) => { numberMotion(`network-detail-down-${i}`, n.download, 1, v => fmt(v, true)); numberMotion(`network-detail-up-${i}`, n.upload, 1, v => fmt(v, true)); });
    }
    drawPlot();
}
let resizeObserver, chartFrame = 0;
function observePlot() { resizeObserver?.disconnect(); const el = document.getElementById('main-plot'); if (!el)
    return; resizeObserver = new ResizeObserver(() => requestFrame(drawPlot)); resizeObserver.observe(el); }
function syncPlotPaths(svgText) { const target = document.getElementById('main-plot'), svg = target?.querySelector('svg'); if (!target || !svg)
    return false; const parsed = new DOMParser().parseFromString(svgText, 'image/svg+xml').querySelector('svg'); if (!parsed)
    return false; const oldPaths = [...svg.querySelectorAll('path[data-series-path]')], newPaths = [...parsed.querySelectorAll('path[data-series-path]')]; if (oldPaths.length !== newPaths.length)
    return false; newPaths.forEach((p, i) => oldPaths[i].setAttribute('d', p.getAttribute('d') || '')); return true; }
function tickChart(now) { chartFrame = 0; if (stopped || document.hidden || !online || state?.paused || !sampleFresh() || settings.monitorAnimation !== 'smooth' || !settings.animations || matchMedia('(prefers-reduced-motion: reduce)').matches)
    return; const target = document.getElementById('main-plot'); if (!target || !target.isConnected)
    return; const result = plot(history, metric, chartOptions(), 'main-chart', target.clientWidth || 550, target.clientHeight || 195, performance.timeOrigin + now); if (!syncPlotPaths(result.svg)) {
    drawPlot();
    return;
} chartFrame = requestFrame(tickChart); }
function stopChart() { if (chartFrame)
    cancelFrame(chartFrame); chartFrame = 0; }
function drawPlot() { const target = document.getElementById('main-plot'); if (!target || !state)
    return; const height = target.clientHeight || 195, width = target.clientWidth || 550; const result = plot(history, metric, chartOptions(), 'main-chart', width, height); if (!syncPlotPaths(result.svg))
    html('main-plot', result.svg); html('main-legend', result.legend); const extra = metric === 'disk' || metric === 'diskio' ? ` · ${disk()?.name || '未选分区'}${metric === 'disk' ? ' · 非容量使用率' : ''}` : metric === 'network' ? ` · ${network()?.name || '无网卡'}` : ''; text('chart-caption', `最近 60 秒 · ${state.background ? '后台 5' : settings.intervalMS / 1000} 秒采样${extra}`); text('chart-time', state.paused ? '已暂停' : `更新 ${time(latest().time)}`); const btn = document.getElementById('pause-button'); if (btn) {
    btn.innerHTML = icon(state.paused ? 'play' : 'pause');
    btn.setAttribute('aria-label', state.paused ? '恢复采样' : '暂停采样');
    btn.title = state.paused ? '恢复采样' : '暂停采样';
} document.querySelectorAll('[data-metric]').forEach(el => { const a = el.dataset.metric === metric; el.classList.toggle('active', a); el.setAttribute('aria-selected', String(a)); }); stopChart(); if (!document.hidden && online && !state.paused && sampleFresh() && settings.monitorAnimation === 'smooth' && settings.animations && !matchMedia('(prefers-reduced-motion: reduce)').matches)
    chartFrame = requestFrame(tickChart); }
function updateSelectors() { const update = (id, options, selected, autoLabel) => { const el = document.getElementById(id); if (!el)
    return; const v = `<option value="">${autoLabel}</option>` + options.map(d => `<option value="${esc(d.id)}">${esc(d.name)}</option>`).join('') + (selected && !options.some(d => d.id === selected) ? `<option value="${esc(selected)}">已移除的设备（${esc(selected)}）</option>` : ''); if (el.innerHTML !== v)
    el.innerHTML = v; el.value = selected; }; update('disk-select', latest().disks, settings.disk, '自动：系统分区优先'); update('network-select', latest().networks, settings.interface, '自动：已连接物理网卡优先'); }
const labels = { Name: '名称', Caption: '系统名称', Version: '版本', BuildNumber: '内部版本', OSArchitecture: '架构', LastBootUpTime: '启动时间', Manufacturer: '厂商', NumberOfCores: '物理核心', NumberOfLogicalProcessors: '逻辑处理器', MaxClockSpeed: '标称最高频率 (MHz)', SocketDesignation: '插槽', AdapterCompatibility: '显卡厂商', DriverVersion: '驱动版本', DriverDate: '驱动日期', VideoProcessor: '图形处理器', CurrentHorizontalResolution: '当前水平分辨率', CurrentVerticalResolution: '当前垂直分辨率', CurrentRefreshRate: '刷新率 (Hz)', Product: '型号', Capacity: '容量', Speed: '速率 (MHz)', ConfiguredClockSpeed: '配置速率 (MHz)', SMBIOSMemoryType: 'SMBIOS 内存类型编号', DeviceLocator: '位置', BankLabel: '内存插槽', PartNumber: '部件型号', Model: '型号', Size: '容量', InterfaceType: '接口', MediaType: '介质类型', FirmwareRevision: '固件', Index: '设备序号', FriendlyName: '设备名称', BusType: '总线类型编号', ScreenWidth: '宽度', ScreenHeight: '高度', MonitorManufacturer: '显示器厂商', MonitorType: '显示器类型', Status: '设备状态', NetConnectionID: '连接名称', NetEnabled: '连接启用', PhysicalAdapter: '物理网卡', SMBIOSBIOSVersion: 'BIOS 版本', ReleaseDate: '发布日期', FirmwareType: '固件启动模式' };
function inventoryMarkup(id, filter = '') { const g = group(id); if (!g)
    return unavailable(hardware.status === 'loading' ? '正在异步读取硬件…' : '暂无硬件信息'); const rows = g.items ?? []; const q = filter.trim().toLocaleLowerCase(); if (q && !JSON.stringify(g).toLocaleLowerCase().includes(q) && !Object.keys(labels).some(k => rows.some(r => k in r) && labels[k].includes(q)))
    return ''; const title = esc(g.name); if (g.status !== 'ok' || !rows.length)
    return `<section class="card info-section"><h2>${icon(id === 'board' ? 'hardware' : id)}${title}<span class="badge">${g.status === 'unsupported' ? '不支持' : '暂不可用'}</span></h2>${unavailable(g.error || '未取得有效数据')}</section>`; return `<section class="card info-section"><h2>${icon(id === 'board' ? 'hardware' : id)}${title}<span class="badge">${rows.length} 项</span></h2>${rows.map((row, i) => `<div class="inventory-device">${rows.length > 1 ? `<h3>${esc(row.Name ?? row.Model ?? row.FriendlyName ?? row.DeviceLocator ?? ('设备 ' + (i + 1)))}</h3>` : ''}<dl class="kv">${Object.entries(row).map(([key, v]) => { let value = v === null || v === '' ? '暂不可用' : typeof v === 'object' ? JSON.stringify(v) : String(v); if ((key === 'Capacity' || key === 'Size') && Number(v) > 0)
    value = fmt(Number(v)); if (key === 'MediaType' && id === 'storage')
    value = Number(v) === 4 ? 'SSD' : Number(v) === 3 ? 'HDD' : Number(v) === 5 ? 'SCM' : '类型未指定'; return `<dt title="${esc(key)}">${esc(labels[key] ?? key)}</dt><dd title="${esc(value)}">${esc(value)}</dd>`; }).join('')}</dl></div>`).join('')}${id === 'gpu' ? '<p class="muted" style="font-size:12px;margin-top:16px">型号与驱动信息来自 CIM；上方负载与显存来自 DXGI/PDH，不使用 32 位 AdapterRAM 字段。温度接口未接入。</p>' : ''}</section>`; }
function updateHardware(force = false) {
    if (!hardware)
        return;
    const h = hardware, loading = h.status === 'loading';
    text('os-caption', String(item('system').Caption ?? (loading ? '正在读取系统信息…' : '系统信息暂不可用')));
    text('os-version', `${item('system').Version ?? '暂不可用'}${item('system').BuildNumber ? ' (' + item('system').BuildNumber + ')' : ''}`);
    text('os-host', h.hostname || '暂不可用');
    text('os-arch', `${item('system').OSArchitecture ?? h.architecture} · ${h.platform}`);
    if (page === 'overview') {
        const summary = [['cpu', '处理器', names('cpu'), 'cpu'], ['gpu', '显卡', names('gpu'), 'gpu'], ['board', '主板', names('board', 'Product'), 'hardware'], ['memory', '内存', latest().memory.total ? `${fmt(latest().memory.total)}${group('memory')?.status === 'ok' ? ' · ' + group('memory').items.length + ' 条内存' : ''}` : '暂不可用', 'memory'], ['disk', '磁盘', names('disk', 'Model'), 'disk'], ['display', '显示器', names('display'), 'monitor'], ['audio', '声卡', names('audio'), 'audio'], ['network', '网卡', names('network'), 'network']];
        html('hardware-summary', summary.map(([id, label, v, ico]) => `<div class="hardware-row">${icon(ico)}<span class="hardware-key">${label}</span><span class="hardware-value" title="${esc(v)}">${esc(v)}${id === 'cpu' && number('cpu', 'NumberOfCores') ? `<small>${number('cpu', 'NumberOfCores')} 核 / ${number('cpu', 'NumberOfLogicalProcessors') ?? '—'} 逻辑处理器</small>` : ''}</span></div>`).join(''));
    }
    const filter = document.getElementById('hardware-filter')?.value ?? '';
    if (force || hardwareTime !== h.time || loading) {
        if (page === 'hardware') {
            const markup = Object.keys(h.groups).map(id => inventoryMarkup(id, filter)).join('');
            html('hardware-details', markup || unavailable(loading ? '正在异步查询硬件，可取消查询…' : filter ? '没有匹配的硬件项目' : '暂无信息'));
            text('inventory-state', loading ? '查询中…' : h.status === 'failed' ? '查询失败' : `更新 ${time(h.time)}`);
        }
        if (page === 'memory')
            html('memory-details', inventoryMarkup('memory'));
        if (page === 'gpu')
            html('gpu-details', inventoryMarkup('gpu'));
        if (page === 'disk')
            html('disk-details', inventoryMarkup('disk') + inventoryMarkup('storage'));
        hardwareTime = h.time;
    }
    document.querySelectorAll('[data-action="refresh-hardware"]').forEach(b => b.disabled = loading);
    document.querySelectorAll('[data-action="cancel-hardware"]').forEach(b => b.disabled = !loading);
}
const statusNames = { ok: '正常', notice: '提醒', warning: '警告', failed: '检测失败', unsupported: '不支持' };
function scoreMarkup() { const score = report?.score; return `<div class="score-area"><div class="score-ring"><svg viewBox="0 0 120 120" aria-hidden="true"><circle class="track" cx="60" cy="60" r="50"/><circle class="progress" cx="60" cy="60" r="50" stroke-dasharray="${score == null ? 0 : Math.max(0, Math.min(100, score)) * Math.PI} 314.159"/></svg><div class="score-value">${score ?? '—'}<small>${score == null ? '暂无评分' : '状态评分'}</small></div></div><p class="score-caption">${report ? score == null ? '数据不足' : '依据本次检查' : '尚未诊断'}</p></div>`; }
function updateDiagnostics() { const placeholder = [['system', '系统信息'], ['disk', '磁盘空间'], ['memory', '内存状态'], ['network', '网络连接'], ['hardware', '硬件信息']]; html('checks-mini', placeholder.map(([id, name]) => { const c = report?.checks.find(c => c.id === id); return `<div class="check-mini-row" title="${esc(c?.explanation ?? '尚未执行诊断')}">${icon(id === 'system' ? 'shield' : id)}<span>${name}</span><span class="check-status status-${c?.status ?? 'pending'}">${c?.status === 'ok' ? icon('check') : ''}${c ? statusNames[c.status] ?? c.status : '待检查'}</span></div>`; }).join('')); html('mini-score', scoreMarkup()); html('full-score', scoreMarkup()); html('diagnostic-list', report ? report.checks.map(c => `<div class="check-full"><div class="check-full-head"><h3>${esc(c.name)}</h3><span class="badge status-${c.status}">${esc(statusNames[c.status] ?? c.status)}</span></div><p>${esc(c.explanation)}</p><small>权重 ${c.weight} · 本次扣分 ${c.penalty}${c.status === 'failed' || c.status === 'unsupported' ? ' · 不计入有效分母' : ''}</small></div>`).join('') : unavailable('尚未进行诊断。点击“开始诊断”读取当前有效样本并生成检查报告。')); text('diag-summary', report?.summary ?? '暂无诊断报告'); text('diag-coverage', report ? `有效覆盖率 ${report.coverage}% · 检测时间 ${new Date(report.time).toLocaleString('zh-CN')}` : '诊断不会自动修改系统'); if (report)
    text('score-rules', report.rules); document.querySelectorAll('[data-action="export-report"]').forEach(b => b.disabled = !report); }
function existingSettingsPage() { return head('设置', '偏好设置仅保存到本机 · 简体中文') + `<section class="card settings-panel"><div class="setting-row"><div><h3>外观主题</h3><p>深蓝专业主题、浅色主题，或跟随系统配色。</p></div><select id="setting-theme" aria-label="主题"><option value="dark">深色</option><option value="light">浅色</option><option value="system">跟随系统</option></select></div><div class="setting-row"><div><h3>采样间隔</h3><p>采样与图表渲染分离。后台至少间隔 5 秒；图表最多保留最近 60 秒。</p></div><select id="setting-interval" aria-label="采样间隔"><option value="500">0.5 秒</option><option value="1000">1 秒（默认）</option><option value="2000">2 秒</option><option value="5000">5 秒</option></select></div><div class="setting-row"><div><h3>监控动画</h3><p>仅影响显示，不改写真实采样、历史和诊断数据；系统减少动态效果时自动关闭。</p></div><select id="setting-monitor-animation" aria-label="监控动画"><option value="off">关闭</option><option value="standard">标准 · 300ms</option><option value="smooth">流畅 · 600ms</option></select></div><div class="setting-row"><div><h3>界面动画</h3><p>只影响过渡动画，不改变真实数据；同时尊重系统的减少动态效果设置。</p></div><button class="switch" id="setting-animations" data-action="animations" role="switch" aria-label="界面动画" aria-checked="false"></button></div><div class="setting-row"><div><h3>容量与流量单位</h3><p>IEC：KiB、MiB、GiB（1024 进制）；SI：kB、MB、GB（1000 进制）。速率均为字节每秒，不与 bit/s 混淆。</p></div><select id="setting-units" aria-label="单位"><option value="iec">IEC · 1024</option><option value="si">SI · 1000</option></select></div><div class="setting-row"><div><h3>登录时启动</h3><p>默认关闭。确认后仅在当前用户的启动文件夹中创建工具箱快捷方式，不写入 Run 注册表项。移动 EXE 后需要重新设置。</p></div><button class="switch" id="setting-startup" data-action="startup" role="switch" aria-label="登录时启动" aria-checked="false" ${boot.platform !== 'windows' ? 'disabled title="仅在 Windows 可用"' : ''}></button></div><div class="setting-row"><div><h3>界面语言</h3><p>当前版本提供简体中文。</p></div><span class="badge">简体中文</span></div></section><section class="card settings-panel"><div class="setting-row"><div><h3>本地日志</h3><p>每个日志最大 1 MiB，保留两个轮转文件。不记录完整硬件型号、IP 或查询内容。</p></div><div class="setting-control">${button('查看日志', 'view-logs', 'list')}${button('清空日志', 'clear-logs', 'trash')}</div></div><div class="setting-row"><div><h3>数据目录</h3><p style="overflow-wrap:anywhere">${esc(boot.dataDir)}</p></div>${button('复制路径', 'copy-path', 'copy')}</div></section>`; }
function settingsPage() { return existingSettingsPage() + updatesSettingsPanel(); }
function updatesSettingsPanel() { return `<section class="card settings-panel update-settings"><div class="card-heading"><div><h2>软件更新</h2><p>仅连接官方 GitHub 仓库检查版本；更新包须由你在浏览器中自行核对并安装。</p></div><span class="badge">${esc(boot.version)}</span></div><div class="setting-row"><div><h3>启动时检查更新</h3><p>默认关闭。启用后启动时仅查询版本，不下载或安装。</p></div><button class="switch" id="setting-update-startup" data-action="updates-on-startup" role="switch" aria-label="启动时检查更新" aria-checked="false"></button></div><div class="setting-row"><div><h3>允许测试版更新</h3><p>关闭时仅查询 GitHub Latest 正式版；开启后也查询预发布版本。</p></div><button class="switch" id="setting-update-beta" data-action="updates-beta" role="switch" aria-label="允许测试版更新" aria-checked="false"></button></div><div class="setting-row"><div><h3>自动下载更新</h3><p>当前禁用：本程序尚未配置可信更新签名公钥或代码签名验证，因此只提供官方页面手动更新。</p></div><button class="switch" id="setting-update-auto" role="switch" aria-label="自动下载更新" aria-checked="false" disabled title="签名验证机制配置完成后才可启用"></button></div><div class="update-status" role="status" aria-live="polite"><strong id="update-summary">尚未检查更新</strong><small id="update-last-check">上次检查：—</small><div id="update-details"></div></div><div class="setting-control">${button('检查更新', 'updates-check', 'refresh', true)}${button('打开官方 Release 页面', 'updates-open', 'external')}</div><p class="muted">安全说明：Release 附带的 SHA-256 文件只能检查传输完整性，不能单独证明发布者身份。没有可信签名时不自动下载、执行或安装更新。</p></section>`; }
let updateError = '';
function renderUpdateState() { if (page !== 'settings')
    return; const summary = document.getElementById('update-summary'), last = document.getElementById('update-last-check'), details = document.getElementById('update-details'); if (!summary || !last || !details)
    return; const s = updateInfo; summary.textContent = updateError || (!s ? '尚未检查更新' : s.status === 'no_release' ? '暂无已发布的正式版本。' : s.status === 'up_to_date' ? '已是最新版本' : s.status === 'update_available' ? `发现新版本 ${s.latestVersion}` : s.status === 'rate_limited' ? 'GitHub 请求次数已达上限，请稍后重试' : s.status === 'network_error' ? '更新检查失败' : `更新状态：${s.status}`); last.textContent = `当前版本：${boot.version} · 上次检查：${s?.checkedAt ? new Date(s.checkedAt).toLocaleString('zh-CN') : '—'} · 渠道：${s?.channel ?? (settings.allowBetaUpdates ? 'stable + beta' : 'stable')}`; if (!s?.latestVersion) {
    details.innerHTML = '';
    return;
} const date = s.publishedAt ? new Date(s.publishedAt).toLocaleDateString('zh-CN') : '未知'; const assets = (s.assets ?? []).map(a => `<li>${esc(a.name)} · ${esc(bytes(a.size, settings.units === 'si'))}</li>`).join(''); details.innerHTML = `<dl class="kv"><dt>最新版本</dt><dd>${esc(s.latestVersion)}</dd><dt>发布日期</dt><dd>${esc(date)}</dd><dt>更新渠道</dt><dd>${esc(s.channel)}</dd><dt>安装包附件</dt><dd>${assets ? `<ul>${assets}</ul>` : 'Release 暂无匹配附件'}</dd></dl><pre class="update-notes">${esc(s.notes || '此版本未提供更新日志。')}</pre>`; }
async function checkForUpdates(manual) { updateError = ''; if (page === 'settings') {
    const e = document.getElementById('update-summary');
    if (e)
        e.textContent = '正在连接官方 GitHub 更新服务…';
} try {
    updateInfo = await api('updates/check', {});
    renderUpdateState();
    if (updateInfo.hasUpdate) {
        if (manual && await confirm(`发现新版本 ${updateInfo.latestVersion}`, `当前版本：${boot.version}\n新版本：${updateInfo.latestVersion}\n发布日期：${updateInfo.publishedAt ? new Date(updateInfo.publishedAt).toLocaleDateString('zh-CN') : '未知'}\n更新日志：\n${updateInfo.notes || '无'}\n\n安装包尚无可信签名校验，本程序只打开官方 Release 页面供你手动检查和下载。`, '打开官方 Release'))
            await api('updates/open-release', { confirmed: true });
        else if (manual)
            toast(`发现新版本 ${updateInfo.latestVersion}`);
    }
    else if (manual)
        toast(updateInfo.status === 'no_release' ? '暂无已发布的正式版本。' : '已是最新版本');
}
catch (e) {
    updateError = e instanceof Error ? e.message : String(e);
    renderUpdateState();
    if (manual)
        toast(updateError, true);
} }
function renderSettingsState() { if (page !== 'settings')
    return; const set = (id, value) => { const el = document.getElementById(id); if (el)
    el.value = value; }; set('setting-theme', settings.theme); set('setting-interval', String(settings.intervalMS)); set('setting-units', settings.units); set('setting-monitor-animation', settings.monitorAnimation); document.getElementById('setting-animations')?.setAttribute('aria-checked', String(settings.animations)); document.getElementById('setting-startup')?.setAttribute('aria-checked', String(boot.startup)); document.getElementById('setting-update-startup')?.setAttribute('aria-checked', String(settings.checkUpdatesOnStartup)); document.getElementById('setting-update-beta')?.setAttribute('aria-checked', String(settings.allowBetaUpdates)); document.getElementById('setting-update-auto')?.setAttribute('aria-checked', String(settings.autoDownloadUpdates)); renderUpdateState(); }
function aboutPage() { return `<section class="card about-hero"><img src="/assets/assets/logo.svg" alt="嘉豪工具箱 Logo"><div><h1>嘉豪工具箱</h1><p>专业 · 高效 · 可靠<br>离线优先的 Windows 硬件监控与系统工具</p><span class="badge">v${esc(boot.version)} · Beta · 未通过 Windows 实机验收</span></div></section><section class="card legal"><h3>构建信息</h3><dl class="kv"><dt>版本</dt><dd>${esc(boot.version)}</dd><dt>构建时间</dt><dd>${esc(boot.buildTime)}</dd><dt>Go 版本</dt><dd>${esc(boot.goVersion)}</dd><dt>运行平台</dt><dd>${esc(boot.platform)} / ${esc(boot.architecture)}</dd><dt>界面宿主</dt><dd>${boot.native ? '原生 Windows / WebView2' : '本地浏览器兼容 / 开发模式'}</dd></dl><h3>评估构建安全说明</h3><p>当前 EXE 使用上方构建信息中报告的 Go 版本构建；Go 支持周期与补丁会变化，正式发布前需核对最新安全维护状态并进行漏洞扫描。本包用于本地评估，不视为安全审计通过版本；正式发布前必须用受支持的已修补 Go 版本重建、执行漏洞扫描并完成 Windows 实机验收。</p><h3>诚实的功能边界</h3><p>本版包含真实 CPU、内存、固定分区和网卡采集实现。GPU 使用 DXGI 与 WDDM PDH（驱动须提供有效计数）；CPU/GPU 温度、SMART 和完整驱动清单尚未接入。Windows x64 已做交叉构建；原生窗口、跨屏 DPI 与 Windows API 数值仍需实机验证。</p><h3>原生窗口与运行依赖</h3><p>使用 Microsoft Edge WebView2 Runtime。公共 WebView2Loader.dll 可按 BUILD.md 配置；单 EXE 兼容加载路径使用 Runtime 内部入口，属于需要重点验收的实验实现。不包含 Runtime，不静默下载或安装。遇到启动错误可使用“启动兼容模式.cmd”打开同一套本地监控界面。</p><h3>隐私与权限</h3><p>硬件数据不上传。后端仅监听随机本地回环端口，使用随机会话及来源校验。硬件导出不包含序列号或 UUID；导出前仍会提醒型号、设备名可能属于敏感信息。开机启动、管理工具和网络测速需要联网确认；磁盘测速会在用户选择的文件夹创建临时文件并需确认。</p><h3>许可证与素材</h3><p>本项目代码按 MIT 许可提供；Go 标准库采用 Go 的 BSD 风格许可。TypeScript 5.8.3 仅用于构建，采用 Apache-2.0 许可，不是运行依赖。WebView2 Runtime 由 Microsoft 单独授权。本项目 SVG 图标、立方体标识、雪山湖泊插画为原创，按 CC0-1.0 提供。未打包系统字体。</p><h3>参考图还原说明</h3><p>保留深蓝配色、导航结构、雪山湖泊横幅、监控卡、图表、快捷工具与诊断区域。当前横幅为可无限缩放的原创矢量插画，不是参考图中的写实摄影；未声称完成像素级还原。</p><p style="margin-top:14px">详细来源、测试记录与未验证项目见随包 LICENSES.md、TEST_REPORT.md 和 source/docs/。</p></section>`; }
let saveQueue = Promise.resolve();
function saveSettings(patch) { const job = saveQueue.then(async () => { const next = { ...settings, ...patch }; settings = await api('settings', next); applyTheme(); renderSettingsState(); updateLive(); updateHardware(true); }); saveQueue = job.catch(() => { }); return job; }
async function withBusy(button, fn) { if (button instanceof HTMLButtonElement && button.disabled)
    return; const buttons = button instanceof HTMLButtonElement ? [button] : []; buttons.forEach(b => { b.disabled = true; b.setAttribute('aria-busy', 'true'); }); try {
    await fn();
}
catch (e) {
    toast(e instanceof Error ? e.message : String(e), true);
}
finally {
    buttons.forEach(b => { b.disabled = false; b.removeAttribute('aria-busy'); });
    renderSettingsState();
    updateDiagnostics();
    updateHardware();
} }
async function runTool(id) { const tool = boot.tools.find(t => t.id === id); if (!tool)
    throw new Error('未知工具'); if (tool.admin && !await confirm('打开管理工具', `${tool.name}可能需要管理员权限。工具箱只打开 Windows 管理入口，不自动执行其中的修改。`, '继续打开'))
    return; const request = { id, elevate: false, confirmed: tool.admin }; try {
    const r = await api('tool', request);
    toast(r.message);
}
catch (e) {
    if (e instanceof ApiError && e.needsElevation && await confirm('需要管理员权限', 'Windows 要求以管理员身份打开此工具。继续后可能显示系统 UAC 对话框。', '请求权限')) {
        const r = await api('tool', { ...request, elevate: true, confirmed: true });
        toast(r.message);
    }
    else
        throw e;
} }
async function action(id) {
    switch (id) {
        case 'speed-start':
            if (!await confirm('开始网络测速', '该测试连接 Cloudflare speed.cloudflare.com，单次预计下载约 25 MB 并上传约 10 MB。不会访问或上传个人文件；结果受 Wi-Fi、VPN、后台流量及测试节点影响。现在开始？', '同意并开始'))
                return;
            speedSamples = [];
            testProgress = undefined;
            await api('speed/start', { confirmed: true });
            scheduleTestPoll();
            toast('网络测速已开始，可随时取消');
            break;
        case 'cancel-benchmark': {
            const r = await api('tests/cancel', {});
            toast(r.cancelRequested ? '已发送取消请求，正在停止并清理' : '当前没有正在运行的测速');
            break;
        }
        case 'disk-browse': {
            const r = await api('disk/pick-folder', {});
            selectedTestDirectory = r.directory;
            text('diskbench-directory', r.directory);
            break;
        }
        case 'disk-start': {
            if (!selectedTestDirectory)
                throw new Error('请先使用文件夹选择器选取测试位置');
            const volume = document.getElementById('diskbench-volume').value;
            if (!volume)
                throw new Error('当前没有可用固定分区');
            const mode = (document.querySelector('input[name="diskbench-mode"]:checked')?.value || 'quick');
            const size = Number(document.getElementById('diskbench-size')?.value || 256), queue = Number(document.getElementById('diskbench-queue')?.value || 1), duration = Number(document.getElementById('diskbench-duration')?.value || 4);
            const shown = mode === 'quick' ? 64 : mode === 'standard' ? 256 : size;
            if (!await confirm('确认磁盘测速', `会在 ${selectedTestDirectory} 下创建仅供本次测试使用的临时文件。预计文件大小 ${shown} MiB，包含顺序写入与 4K 随机写入，会产生少量 SSD 写入损耗。不会修改原始磁盘或覆盖现有文件；Windows 文件缓存会影响读取成绩。确认继续？`, '确认并开始'))
                return;
            testProgress = undefined;
            await api('disk/start', { directory: selectedTestDirectory, volumeId: volume, mode, fileSizeMiB: size, queueDepth: queue, durationSeconds: duration, confirmed: true });
            scheduleTestPoll();
            toast('磁盘测速已开始，可随时取消');
            break;
        }
        case 'disk-recover': {
            if (!selectedTestDirectory)
                throw new Error('请先选择曾进行磁盘测速的文件夹');
            const volume = document.getElementById('diskbench-volume').value;
            if (!await confirm('检查遗留测试文件', `仅检查 ${selectedTestDirectory} 下带有嘉豪工具箱专属标识的临时测速目录；其他文件和目录不会处理。`, '检查并清理'))
                return;
            const r = await api('disk/recover', { directory: selectedTestDirectory, volumeId: volume, confirmed: true });
            toast(`已清理 ${r.removed} 个可识别的遗留测速目录`);
            break;
        }
        case 'cmd-open': {
            await api('cmd/open', {});
            toast('已打开绿色 CMD 窗口');
            break;
        }
        case 'cmd-scan': {
            const pick = await api('cmd/pick-folder', {});
            if (!await confirm('运行经典 dir /s', `将在单独的绿色 CMD 窗口递归扫描你选择的目录：\n${pick.directory}\n扫描会遍历其子目录；可按 Ctrl+C 或关闭 CMD 窗口终止。`, '打开 CMD 并扫描'))
                return;
            await api('cmd/scan', { directory: pick.directory, confirmed: true });
            toast('扫描已在独立 CMD 窗口启动');
            break;
        }
        case 'cmd-copy':
            await copyText(classicBAT);
            break;
        case 'cmd-save-bat':
            download('Jiahao_Classic_CMD.bat', classicBAT);
            break;
        case 'cmd-save-legacy':
            download('Jiahao_Classic_Toolbox.bat', legacyBAT);
            toast('已提交 BAT 保存请求；文件不会自动运行');
            break;
        case 'pause':
            await api('monitor', { paused: !state.paused, background: document.hidden });
            state.paused = !state.paused;
            toast(state.paused ? '已暂停采样' : '已恢复采样');
            updateLive();
            break;
        case 'diagnose':
            report = await api('diagnose', {});
            updateDiagnostics();
            toast(report.score == null ? '检查完成：数据不足，暂无评分' : `检查完成，状态评分 ${report.score}，请阅读各项说明`);
            break;
        case 'refresh-hardware': {
            const r = await api('hardware/refresh', {});
            hardware = await api('hardware');
            updateHardware(true);
            toast(r.started ? '已开始异步查询，可在详情页取消' : '已有硬件查询在执行');
            break;
        }
        case 'cancel-hardware': {
            const r = await api('hardware/cancel', {});
            toast(r.cancelled ? '已发送取消请求，正在回收查询进程' : '当前没有正在进行的查询');
            break;
        }
        case 'copy-hardware':
            if (await confirm('复制硬件信息', '数据不包含序列号或 UUID，但包含设备名称、硬件型号、驱动与其他设备信息。请确认剪贴板中允许保存这些信息。', '复制'))
                await copyText(JSON.stringify(hardware, null, 2));
            break;
        case 'export-hardware':
            if (await confirm('导出硬件信息', '导出为本地 JSON。不包含序列号与 UUID，但可能包含设备名、型号和驱动信息，请勿随意公开。', '导出'))
                download('Jiahao-Hardware-' + dateName() + '.json', hardware);
            break;
        case 'export-report':
            if (!report)
                throw new Error('请先运行一次诊断');
            if (await confirm('导出诊断报告', '报告包含检测时间、项目解释与评分，可能包含本地分区名称。仅保存到你选择的本地位置。', '导出'))
                download('Jiahao-Diagnostics-' + dateName() + '.json', report);
            break;
        case 'animations':
            await saveSettings({ animations: !settings.animations });
            toast('动画偏好已保存');
            break;
        case 'startup': {
            const enabled = !boot.startup;
            if (!await confirm(enabled ? '启用登录启动' : '关闭登录启动', enabled ? '将在当前用户的启动文件夹创建指向此 EXE 的快捷方式。不会修改注册表。' : '只删除由嘉豪工具箱创建并标识的启动快捷方式。', '确认修改'))
                return;
            const r = await api('startup', { enabled, confirmed: true });
            boot.startup = r.enabled;
            renderSettingsState();
            toast(r.enabled ? '已启用登录启动' : '已关闭登录启动');
            break;
        }
        case 'updates-on-startup':
            await saveSettings({ checkUpdatesOnStartup: !settings.checkUpdatesOnStartup });
            toast(settings.checkUpdatesOnStartup ? '已启用启动时检查更新' : '已关闭启动时检查更新');
            break;
        case 'updates-beta':
            await saveSettings({ allowBetaUpdates: !settings.allowBetaUpdates });
            toast(settings.allowBetaUpdates ? '已允许检查测试版' : '仅检查正式版');
            break;
        case 'updates-check':
            await checkForUpdates(true);
            break;
        case 'updates-open':
            if (await confirm('打开官方更新页面', '即将打开固定官方仓库 stone2013/JiahaoToolbox 的 Releases 页面。请在浏览器中自行核对来源和版本。', '继续打开'))
                await api('updates/open-release', { confirmed: true });
            break;
        case 'view-logs': {
            const r = await api('logs');
            showText('本地运行日志', r.text || '日志为空');
            break;
        }
        case 'clear-logs':
            if (await confirm('清空工具箱日志', '只清空本应用的 app.log 与两个轮转文件。不会删除 Windows 事件日志或其他文件。', '清空')) {
                await api('logs/clear', { confirmed: true });
                toast('工具箱日志已清空');
            }
            break;
        case 'copy-path':
            await copyText(boot.dataDir);
            break;
        case 'cancel-network':
            netTask?.abort();
            break;
        case 'exit':
            if (boot.native) {
                await api('window', { command: 'close' });
            }
            else if (await confirm('退出工具箱', '将停止本地采样和服务。此浏览器标签页可随后手动关闭。', '退出')) {
                await api('exit', {});
                stopped = true;
                if (timer)
                    clearTimeout(timer);
                document.getElementById('app').innerHTML = `<div class="boot"><img src="/assets/assets/logo.svg" width="48" alt=""><h1>工具箱已退出</h1><p>本地服务已收到退出请求。你可以关闭此标签页。</p></div>`;
            }
            break;
        default: throw new Error('未知操作');
    }
}
function dateName() { return new Date().toISOString().replace(/[:.]/g, '-'); }
async function checkNetwork(e) { e.preventDefault(); if (netTask)
    return; const target = document.getElementById('network-target').value.trim(); const kind = e.submitter?.value || 'dns'; if (!await confirm('允许这次联网检查？', `${kind === 'dns' ? 'DNS 将使用系统配置的解析器查询' : 'Ping 将发送 ICMP 请求到'}：${target}\n该操作需要网络；不会上传硬件清单。`, '开始检查'))
    return; const task = netTask = new AbortController(); document.querySelectorAll('#network-form button[type="submit"]').forEach(b => b.disabled = true); const cancel = document.getElementById('cancel-network'); cancel.disabled = false; html('network-result', '<p class="muted" style="margin-top:15px">正在检查…</p>'); try {
    const r = await api('network/check', { kind, target, confirmed: true }, task.signal);
    html('network-result', `<pre class="modal-code">${esc(JSON.stringify(r, null, 2))}</pre>`);
}
catch (e) {
    html('network-result', `<div class="notice" style="margin-top:12px">${esc(e instanceof Error ? e.message : String(e))}</div>`);
}
finally {
    netTask = null;
    document.querySelectorAll('#network-form button[type="submit"]').forEach(b => b.disabled = false);
    cancel.disabled = true;
} }
function closeSearch() { document.getElementById('global-search')?.removeAttribute('aria-activedescendant'); const box = document.getElementById('search-results'); if (box)
    box.hidden = true; document.getElementById('global-search')?.setAttribute('aria-expanded', 'false'); }
function search(query) { searchQuery = query; searchMatches = searchService.search(query); searchIndex = 0; renderSearch(); }
function renderSearch() { const box = document.getElementById('search-results'); box.hidden = false; const input = document.getElementById('global-search'); input.setAttribute('aria-expanded', 'true'); if (searchMatches.length)
    input.setAttribute('aria-activedescendant', 'search-option-' + searchIndex);
else
    input.removeAttribute('aria-activedescendant'); box.innerHTML = searchMatches.length ? searchMatches.map((r, i) => `<button id="search-option-${i}" type="button" role="option" aria-selected="${searchIndex === i}" data-search-index="${i}" class="search-result${searchIndex === i ? ' selected' : ''}">${icon(r.entry.icon)}<span>${highlightTitle(r.entry, searchQuery)}<span class="search-detail">${highlight(r.matched === r.entry.title ? r.entry.pinyin : r.matched, searchQuery)}</span></span><small>${r.entry.type === 'page' ? '页面' : '系统工具'}</small></button>`).join('') : '<div class="search-empty" role="status">没有找到相关功能，请试试中文、完整拼音或首字母，例如 xitong、xtgj。</div>'; document.getElementById('search-option-' + searchIndex)?.scrollIntoView({ block: 'nearest' }); }
function activateSearch(i) { const r = searchMatches[i]; if (!r)
    return; closeSearch(); dispatchSearch(r.entry, navigate, id => { if (boot.platform !== 'windows')
    toast('此系统工具仅在 Windows 中可用', true);
else
    withBusy(null, () => runTool(id)); }); }
function bindEvents() {
    document.addEventListener('click', e => { const target = e.target; if (!(target instanceof Element))
        return; const b = target.closest('button'); if (b && !b.disabled) {
        if (b.dataset.route)
            navigate(b.dataset.route);
        else if (b.dataset.metric) {
            metric = b.dataset.metric;
            drawPlot();
        }
        else if (b.dataset.diskmetric) {
            metric = b.dataset.diskmetric;
            document.querySelectorAll('[data-diskmetric]').forEach(el => el.classList.toggle('active', el === b));
            drawPlot();
        }
        else if (b.dataset.theme)
            withBusy(b, async () => { await saveSettings({ theme: b.dataset.theme }); toast('主题已保存'); });
        else if (b.dataset.tool)
            withBusy(b, () => runTool(b.dataset.tool));
        else if (b.dataset.action)
            withBusy(b, () => action(b.dataset.action));
        else if (b.dataset.window)
            withBusy(b, async () => { await api('window', { command: b.dataset.window }); });
        else if (b.dataset.searchIndex)
            activateSearch(Number(b.dataset.searchIndex));
    } if (!target.closest('.search-wrap'))
        closeSearch(); });
    document.addEventListener('change', e => { const el = e.target; if (el.id === 'diskbench-volume')
        return; const patches = { 'setting-theme': { theme: el.value }, 'setting-interval': { intervalMS: Number(el.value) }, 'setting-units': { units: el.value }, 'setting-monitor-animation': { monitorAnimation: el.value }, 'disk-select': { disk: el.value }, 'network-select': { interface: el.value } }; if (patches[el.id])
        withBusy(null, async () => { try {
            await saveSettings(patches[el.id]);
            toast('设置已保存');
        }
        catch (err) {
            renderSettingsState();
            updateSelectors();
            throw err;
        } }); });
    document.addEventListener('compositionend', e => { const el = e.target; if (el.id === 'global-search')
        search(el.value); });
    document.addEventListener('input', e => { const el = e.target; if (el.id === 'global-search' && !e.isComposing)
        search(el.value); if (el.id === 'hardware-filter')
        updateHardware(true); });
    document.addEventListener('focusin', e => { const el = e.target; if (el.id === 'global-search')
        search(el.value); });
    document.addEventListener('keydown', e => { if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        document.getElementById('global-search').focus();
        search(document.getElementById('global-search').value);
    } if (e.isComposing)
        return; if (e.key === 'Escape') {
        if (e.target.id === 'global-search')
            e.preventDefault();
        closeSearch();
        return;
    } if (e.target.id === 'global-search' && !document.getElementById('search-results')?.hidden) {
        if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
            e.preventDefault();
            searchIndex = (searchIndex + (e.key === 'ArrowDown' ? 1 : -1) + searchMatches.length) % Math.max(1, searchMatches.length);
            renderSearch();
        }
        if (e.key === 'Enter') {
            e.preventDefault();
            activateSearch(searchIndex);
        }
    } });
    document.addEventListener('submit', e => { if (e.target.id === 'network-form')
        void checkNetwork(e).catch(err => toast(String(err), true)); });
    document.addEventListener('pointerdown', e => { const t = e.target; if (boot.native && e.button === 0 && t.closest('.topbar') && !t.closest('button,input,select,.search-wrap')) {
        void api('window', { command: 'drag' }).catch(err => toast(String(err), true));
    } });
    document.addEventListener('dblclick', e => { const t = e.target; if (boot.native && t.closest('.topbar') && !t.closest('button,input,.search-wrap'))
        void api('window', { command: 'maximize' }).catch(err => toast(String(err), true)); });
    document.addEventListener('visibilitychange', () => { if (stopped)
        return; motion.setVisible(!document.hidden); if (document.hidden)
        stopChart();
    else
        drawPlot(); void api('monitor', { paused: state.paused, background: document.hidden }).catch(() => { }); if (timer)
        clearTimeout(timer); timer = setTimeout(poll, document.hidden ? 5000 : 0); });
    matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => { if (settings.theme === 'system')
        applyTheme(); });
    matchMedia('(prefers-reduced-motion: reduce)').addEventListener('change', () => { if (!document.hidden)
        updateLive(); });
    window.addEventListener('pagehide', () => { stopped = true; if (timer)
        clearTimeout(timer); if (benchmarkTimer)
        clearTimeout(benchmarkTimer); if (testProgress?.status === 'running' || testProgress?.status === 'canceling')
        void api('tests/cancel', {}).catch(() => { }); stopChart(); motion.dispose(); resizeObserver?.disconnect(); netTask?.abort(); });
}
function takeState(next) { state = next; if (next.history?.length)
    history = next.history;
else if (next.latest.time && next.latest.time !== history[history.length - 1]?.time)
    history.push(next.latest); const end = history[history.length - 1]?.time ?? Date.now(); history = history.filter(s => s.time >= end - 60000).slice(-121); }
async function poll() { if (stopped || polling)
    return; polling = true; try {
    const next = await api('snapshot?history=1');
    const wasOffline = !online;
    online = true;
    takeState(next);
    if (hardware.status === 'loading' || ++pollCount % 15 === 0) {
        hardware = await api('hardware');
        updateHardware();
    }
    updateLive();
    if (wasOffline)
        toast('本地连接已恢复');
}
catch (e) {
    if (online)
        toast(e instanceof Error ? e.message : String(e), true);
    online = false;
    updateLive();
}
finally {
    polling = false;
    if (!stopped)
        timer = setTimeout(poll, document.hidden ? 5000 : Math.max(500, settings.intervalMS));
} }
async function init() { try {
    const stylesheet = document.querySelector('link[href="/assets/styles/app.css"]');
    if (stylesheet) {
        let valid = false;
        try {
            valid = !!stylesheet.sheet?.cssRules.length;
        }
        catch { }
        if (!valid)
            throw new Error('界面样式资源加载失败，请重新启动或使用兼容模式。');
    }
    window.jiahaoStartup?.stage('正在连接核心接口…');
    boot = await api('bootstrap');
    window.jiahaoStartup?.stage('正在挂载仪表盘…');
    searchService = new SearchService();
    settings = boot.settings;
    settings.monitorAnimation ?? (settings.monitorAnimation = 'smooth');
    settings.checkUpdatesOnStartup ?? (settings.checkUpdatesOnStartup = false);
    settings.allowBetaUpdates ?? (settings.allowBetaUpdates = false);
    settings.autoDownloadUpdates ?? (settings.autoDownloadUpdates = false);
    hardware = boot.hardware;
    takeState(boot.state);
    shell();
    applyTheme();
    mount();
    await api('ui-ready', {});
    void window.jiahaoStartup?.finish();
    if (boot.configWarning)
        toast(boot.configWarning, true);
    void api('updates/status').then(i => { updateInfo = i; renderUpdateState(); }).catch(() => { });
    if (settings.checkUpdatesOnStartup)
        setTimeout(() => void checkForUpdates(false), 500);
    timer = setTimeout(poll, Math.max(500, settings.intervalMS));
}
catch (e) {
    window.jiahaoStartup?.fail(e instanceof Error ? e.message : String(e));
    document.getElementById('app').innerHTML = `<div class="boot"><img src="/assets/assets/logo.svg" width="48" alt=""><h1>无法连接本地服务</h1><p>${esc(e instanceof Error ? e.message : String(e))}</p><p>请重新启动嘉豪工具箱，并通过 EXE 打开的窗口访问。</p><button class="button primary" id="retry-start">重新连接</button></div>`;
    document.getElementById('retry-start').onclick = () => void init();
} }
void init();
