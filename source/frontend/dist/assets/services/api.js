export class ApiError extends Error {
    constructor(message, status, needsElevation = false) {
        super(message);
        this.status = status;
        this.needsElevation = needsElevation;
    }
}
export async function api(path, body, signal) {
    const abort = new AbortController();
    const timeout = setTimeout(() => abort.abort(), 12000);
    const forward = () => abort.abort();
    signal?.addEventListener('abort', forward, { once: true });
    if (signal?.aborted)
        abort.abort();
    try {
        const r = await fetch('/api/' + path, { method: body === undefined ? 'GET' : 'POST', credentials: 'same-origin', headers: body === undefined ? {} : { 'Content-Type': 'application/json', 'X-Jiahao-Request': '1' }, body: body === undefined ? undefined : JSON.stringify(body), signal: abort.signal });
        const text = await r.text();
        let v;
        try {
            v = JSON.parse(text);
        }
        catch {
            throw new ApiError(r.status === 401 ? '本地会话已结束，请重新启动工具箱' : '本地服务返回了无效响应', r.status);
        }
        if (!r.ok) {
            const e = v;
            throw new ApiError(e.error || '请求失败', r.status, !!e.needsElevation);
        }
        ;
        return v;
    }
    catch (e) {
        if (e instanceof DOMException && e.name === 'AbortError')
            throw new Error(signal?.aborted ? '操作已取消' : '本地请求超时，请查看日志');
        throw e;
    }
    finally {
        clearTimeout(timeout);
        signal?.removeEventListener('abort', forward);
    }
}
