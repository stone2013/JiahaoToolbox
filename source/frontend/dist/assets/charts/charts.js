import { bytes, esc, time } from '../services/format.js';
const value = (v) => v?.value ?? null;
export function seriesFor(metric, o, latest) {
    if (metric === 'gpu')
        return (latest?.gpus ?? []).slice(0, 4).map((g, i) => ({ name: g.name, color: ['#168bff', '#39cce8', '#21d67b', '#9271f5'][i], get: (s) => value(s.gpus.find(v => v.id === g.id)?.usage) }));
    if (metric === 'cpu')
        return [{ name: '总使用率', color: '#168bff', get: s => value(s.cpu.usage) }, ...[0, 1, 2, 3].filter(i => !!latest?.cpu.logical[i]).map((i) => ({ name: `逻辑处理器 ${latest?.cpu.names[i] ?? i}`, color: ['#39cce8', '#21d67b', '#9271f5', '#abb2ff'][i], get: s => value(s.cpu.logical[i]) }))];
    if (metric === 'memory')
        return [{ name: '物理内存', color: '#21d67b', get: s => value(s.memory.usage) }];
    if (metric === 'disk')
        return [{ name: '磁盘活动率', color: '#9255e8', get: s => value(s.disks.find(d => d.id === o.diskID)?.activity) }];
    if (metric === 'diskio')
        return [{ name: '读取', color: '#9255e8', get: s => value(s.disks.find(d => d.id === o.diskID)?.read) }, { name: '写入', color: '#d2a6ff', get: s => value(s.disks.find(d => d.id === o.diskID)?.write) }];
    return [{ name: '下载', color: '#168bff', get: s => value(s.networks.find(n => n.id === o.networkID)?.download) }, { name: '上传', color: '#39cce8', get: s => value(s.networks.find(n => n.id === o.networkID)?.upload) }];
}
function segments(data, get, x, y, maxGap) {
    const paths = [];
    let path = '', firstX = 0, lastX = 0, lastTime = 0;
    const flush = () => { if (path)
        paths.push({ path, firstX, lastX }); path = ''; };
    for (const s of data) {
        const v = get(s);
        if (v == null || !Number.isFinite(v)) {
            flush();
            lastTime = 0;
            continue;
        }
        ;
        if (lastTime && s.time - lastTime > maxGap)
            flush();
        const xx = x(s.time);
        if (!path)
            firstX = xx;
        path += `${path ? 'L' : 'M'}${xx.toFixed(2)} ${y(v).toFixed(2)} `;
        lastX = xx;
        lastTime = s.time;
    }
    ;
    flush();
    return paths;
}
export function mini(data, metric, o, id) {
    if (!data.length)
        return '<div class="spark-empty">等待真实采样</div>';
    const latest = data[data.length - 1], cut = latest.time - 60000, d = data.filter(s => s.time >= cut), ser = seriesFor(metric, o, latest)[0];
    if (!ser)
        return '<div class="spark-empty">暂无适配器数据</div>';
    const vals = d.map(ser.get).filter((n) => n != null && Number.isFinite(n));
    if (vals.length < 2)
        return `<div class="spark-empty">${vals.length ? '等待第二个采样点' : '暂不可用'}</div>`;
    const max = metric === 'network' ? Math.max(1024, ...vals) * 1.1 : 100, w = 240, h = 40, x = (t) => (t - cut) / 60000 * w, y = (v) => h - 2 - Math.max(0, Math.min(max, v)) / max * (h - 4);
    const p = segments(d, ser.get, x, y, Math.max(6500, o.interval * 3));
    return `<svg viewBox="0 0 ${w} ${h}" preserveAspectRatio="none" aria-hidden="true"><defs><linearGradient id="${id}-fill" x1="0" y1="0" x2="0" y2="1"><stop stop-color="${ser.color}" stop-opacity=".3"/><stop offset="1" stop-color="${ser.color}" stop-opacity=".015"/></linearGradient></defs>${p.map(p => `<path d="${p.path}L${p.lastX} ${h}L${p.firstX} ${h}Z" fill="url(#${id}-fill)"/><path d="${p.path}" fill="none" stroke="${ser.color}" stroke-width="1.4" vector-effect="non-scaling-stroke"/>`).join('')}</svg>`;
}
export function plot(data, metric, o, id, width = 600, height = 210, endTime) {
    const latest = data[data.length - 1];
    const end = endTime ?? latest?.time ?? Date.now(), start = end - 60000, d = data.filter(s => s.time >= start), series = seriesFor(metric, o, latest);
    const rate = metric === 'network' || metric === 'diskio';
    let max = 100;
    if (rate) {
        const values = series.flatMap(s => d.map(s.get)).filter((n) => n != null && Number.isFinite(n));
        const raw = Math.max(1024, ...values) * 1.12;
        const pow = Math.pow(10, Math.floor(Math.log10(raw)));
        max = Math.ceil(raw / pow) * pow;
    }
    const W = Math.max(220, width), H = Math.max(135, height), left = rate ? 61 : 42, right = 6, top = 15, bottom = 29, pw = W - left - right, ph = H - top - bottom, x = (t) => left + (t - start) / 60000 * pw, y = (v) => top + ph - Math.max(0, Math.min(max, v)) / max * ph;
    let grid = '';
    for (let i = 0; i <= 4; i++) {
        const yy = top + i * ph / 4, val = max * (1 - i / 4), label = rate ? bytes(val, o.si, true) : `${val.toFixed(0)}%`;
        grid += `<line x1="${left}" y1="${yy}" x2="${W - right}" y2="${yy}" stroke="var(--line)" stroke-opacity=".62"/><text x="${left - 10}" y="${yy + 4}" text-anchor="end" fill="var(--muted)" font-size="10">${esc(label)}</text>`;
    }
    const ticks = pw < 350 ? 3 : 6;
    for (let i = 0; i <= ticks; i++) {
        const xx = left + i * pw / ticks;
        grid += `<line x1="${xx}" y1="${top}" x2="${xx}" y2="${top + ph}" stroke="var(--line)" stroke-opacity=".45"/><text x="${xx}" y="${H - 7}" text-anchor="${i === 0 ? 'start' : i === ticks ? 'end' : 'middle'}" fill="var(--muted)" font-size="9">${time(start + i * 60000 / ticks)}</text>`;
    }
    let points = 0;
    let lines = '';
    series.forEach((s, i) => { const p = segments(d, s.get, x, y, Math.max(6500, o.interval * 3)); points += p.length; for (const seg of p) {
        if (i === 0)
            lines += `<path data-series-path="1" d="${seg.path}L${seg.lastX} ${top + ph}L${seg.firstX} ${top + ph}Z" fill="url(#${id}-area)"/>`;
        lines += `<path data-series-path="1" d="${seg.path}" fill="none" stroke="${s.color}" stroke-width="${i === 0 ? 1.7 : 1.15}" vector-effect="non-scaling-stroke"/>`;
    } });
    const svg = `<svg viewBox="0 0 ${W} ${H}" preserveAspectRatio="none" role="img" aria-label="最近六十秒${rate ? '速率' : '使用率'}曲线，来自本机真实采样"><defs><linearGradient id="${id}-area" x1="0" y1="0" x2="0" y2="1"><stop stop-color="${(series[0]?.color ?? '#168bff')}" stop-opacity=".28"/><stop offset="1" stop-color="${(series[0]?.color ?? '#168bff')}" stop-opacity=".02"/></linearGradient></defs>${grid}${lines}</svg>${points === 0 ? '<div class="chart-empty">此指标暂不可用<br>未使用模拟数据填充</div>' : ''}`;
    const legend = series.map(s => { const v = latest ? s.get(latest) : null; return `<div class="legend-row"><span class="dot" style="color:${s.color}"></span><span class="legend-name" title="${esc(s.name)}">${esc(s.name)}</span><span class="legend-value">${v == null ? '—' : rate ? bytes(v, o.si, true) : v.toFixed(0) + '%'}</span></div>`; }).join('');
    return { svg, legend };
}
