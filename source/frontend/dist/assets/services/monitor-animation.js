export function easeOut(t) { const x = Math.max(0, Math.min(1, t)); return 1 - (1 - x) * (1 - x); }
/** Presentation-only interpolation. It never owns or mutates sampler snapshots. */
export class MonitorAnimator {
    constructor(mode, reduced, raf = cb => typeof requestAnimationFrame === 'function' ? requestAnimationFrame(cb) : setTimeout(() => cb(performance.now()), 16), cancel = id => { if (typeof cancelAnimationFrame === 'function')
        cancelAnimationFrame(id);
    else
        clearTimeout(id); }) {
        this.mode = mode;
        this.reduced = reduced;
        this.raf = raf;
        this.cancel = cancel;
        this.motions = new Map();
        this.displayed = new Map();
        this.frame = 0;
        this.visible = true;
        this.disposed = false;
    }
    setVisible(visible) { this.visible = visible; if (!visible) {
        if (this.frame)
            this.cancel(this.frame);
        this.frame = 0;
    } }
    reset() { if (this.frame)
        this.cancel(this.frame); this.frame = 0; this.motions.clear(); this.displayed.clear(); }
    animate(key, target, duration, render) { if (this.disposed)
        return; if (target == null || !Number.isFinite(target)) {
        this.motions.delete(key);
        this.displayed.delete(key);
        render(Number.NaN);
        this.schedule();
        return;
    } const old = this.motions.get(key); const from = old ? this.valueAt(old, performance.now()) : this.displayed.get(key) ?? target; if (this.mode() === 'off' || this.reduced() || duration <= 0) {
        this.motions.delete(key);
        this.displayed.set(key, target);
        render(target);
        this.schedule();
        return;
    } this.motions.set(key, { from, to: target, start: performance.now(), duration, render }); this.schedule(); }
    valueAt(m, now) { const p = (now - m.start) / m.duration; return m.from + (m.to - m.from) * easeOut(p); }
    schedule() { if (this.frame || !this.visible || this.disposed || !this.motions.size)
        return; this.frame = this.raf(now => { this.frame = 0; if (this.disposed || !this.visible)
        return; for (const [key, m] of this.motions) {
        const p = Math.min(1, (now - m.start) / m.duration), v = m.from + (m.to - m.from) * easeOut(p);
        this.displayed.set(key, v);
        m.render(v);
        if (p >= 1)
            this.motions.delete(key);
    } this.schedule(); }); }
    dispose() { this.disposed = true; if (this.frame)
        this.cancel(this.frame); this.frame = 0; this.motions.clear(); this.displayed.clear(); }
    get activeCount() { return this.motions.size; }
}
