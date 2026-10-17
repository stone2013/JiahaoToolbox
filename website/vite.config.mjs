import { defineConfig } from 'vite';
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';
import { releasesUrl } from './scripts/release-data.mjs';
const escape = value => String(value).replace(/[&<>"']/g, x => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[x]));
export default defineConfig({
  base: '/JiahaoToolbox/',
  plugins: [{ name: 'verified-release-content', transformIndexHtml(html) {
    const r = JSON.parse(fs.readFileSync(new URL('./src/release.json', import.meta.url), 'utf8'));
    const date = r.publishedAt ? new Intl.DateTimeFormat('zh-CN', { timeZone:'Asia/Shanghai', year:'numeric', month:'long', day:'numeric' }).format(new Date(r.publishedAt)) : '敬请期待';
    const checked = new Intl.DateTimeFormat('zh-CN', { timeZone:'Asia/Shanghai', year:'numeric', month:'2-digit', day:'2-digit' }).format(new Date(r.checkedAt));
    const links = r.assets.map(a => `<a class="button ${a.type === 'installer' ? 'primary' : 'secondary'}" href="${escape(a.url)}"><svg class="icon" aria-hidden="true"><use href="#i-${a.type === 'installer' ? 'download' : 'box'}"/></svg>${a.type === 'installer' ? '下载安装版' : '下载便携版'}<span class="file-size">${(a.bytes / 1e6).toFixed(1)} MB</span></a>`).join('');
    const values = {
      RELEASE_BADGE: r.available ? `v${escape(r.version)} 已发布` : '正式版本即将发布',
      RELEASE_VERSION: r.available ? `v${escape(r.version)}` : '正式版本即将发布',
      RELEASE_DATE: escape(date), RELEASE_CHECKED: escape(checked),
      RELEASE_LINK: escape(r.available ? r.url : releasesUrl),
      RELEASE_DOWNLOADS: links || `<a class="button primary" href="${releasesUrl}">前往 GitHub Releases<svg class="icon" aria-hidden="true"><use href="#i-arrow"/></svg></a>`,
      RELEASE_CHANGES: r.changelog.length ? r.changelog.map(x => `<li>${escape(x)}</li>`).join('') : '<li>正式版本发布后，将在这里显示更新内容。</li>',
      RELEASE_DATE_LABEL: r.available ? '发布于' : '发布状态',
    };
    for (const [key, value] of Object.entries(values)) html = html.replaceAll(`__${key}__`, value);
    return html;
  }}],
  build: { rolldownOptions: { input: { index: fileURLToPath(new URL('./index.html', import.meta.url)), notFound: fileURLToPath(new URL('./404.html', import.meta.url)) } } },
});
