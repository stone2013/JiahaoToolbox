export function esc(v) { return String(v ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c])); }
export function bytes(n, si = false, rate = false) { if (n == null || !Number.isFinite(n) || n < 0)
    return '—'; const base = si ? 1000 : 1024, units = si ? ['B', 'kB', 'MB', 'GB', 'TB', 'PB'] : ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB']; let i = 0; while (n >= base && i < units.length - 1) {
    n /= base;
    i++;
} return `${n.toFixed(i === 0 ? 0 : n >= 100 ? 0 : 1)} ${units[i]}${rate ? '/s' : ''}`; }
export function pct(v, d = 0) { return v?.value == null ? '—' : `${v.value.toFixed(d)}%`; }
export function reason(v) { return v?.reason || (v?.status === 'ok' ? '来自本机实时采样' : '暂不可用'); }
export function uptime(s) { const d = Math.floor(s / 86400), h = Math.floor(s % 86400 / 3600), m = Math.floor(s % 3600 / 60); return `${d ? d + ' 天 ' : ''}${h} 小时 ${m} 分钟`; }
export function time(t) { return new Date(t).toLocaleTimeString('zh-CN', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' }); }
export function stringify(v) { if (v == null || v === '')
    return '暂不可用'; if (Array.isArray(v))
    return v.map(stringify).join('、'); if (typeof v === 'object')
    return JSON.stringify(v); return String(v).trim() || '暂不可用'; }
export function text(id, value) { const e = document.getElementById(id); if (e && e.textContent !== String(value ?? ''))
    e.textContent = String(value ?? ''); }
export function html(id, value) { const e = document.getElementById(id); if (e && e.innerHTML !== value)
    e.innerHTML = value; }
