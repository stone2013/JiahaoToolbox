export const repository = 'stone2013/JiahaoToolbox';
export const releasesUrl = `https://github.com/${repository}/releases`;
export const apiUrl = `https://api.github.com/repos/${repository}/releases/latest`;

export function normalizeRelease(data, checkedAt = new Date().toISOString()) {
  if (!data) return { available: false, checkedAt, version: null, publishedAt: null, url: releasesUrl, changelog: [], assets: [] };
  if (data.draft || data.prerelease || !/^v?\d+\.\d+\.\d+$/.test(data.tag_name ?? '')) throw new Error('Not a stable semantic release');
  const version = data.tag_name.replace(/^v/, '');
  const expected = `${releasesUrl}/tag/${data.tag_name}`;
  if (data.html_url !== expected || !Number.isFinite(Date.parse(data.published_at))) throw new Error('Invalid official release metadata');
  const section = (data.body ?? '').split('## 本版内容')[1]?.split('\n## ')[0] ?? '';
  const changelog = section.split('\n').filter(x => x.startsWith('- ')).slice(0, 4).map(x => x.slice(2).replace(/`/g, ''));
  const names = { installer: `JiahaoToolbox_v${version}_Setup_x64.exe`, portable: `JiahaoToolbox_v${version}_Portable.zip` };
  const assets = Object.entries(names).flatMap(([type, name]) => {
    const asset = data.assets?.find(x => x.name === name);
    const url = `https://github.com/${repository}/releases/download/${data.tag_name}/${name}`;
    return asset && asset.browser_download_url === url && asset.state === 'uploaded' && Number.isSafeInteger(asset.size) && asset.size > 0
      ? [{ type, name, url, bytes: asset.size }] : [];
  });
  return { available: true, checkedAt, version, publishedAt: data.published_at, url: expected, changelog, assets };
}

export async function fetchRelease(fetcher = fetch) {
  const response = await fetcher(apiUrl, { headers: { Accept: 'application/vnd.github+json', 'User-Agent': 'JiahaoToolbox-Official-Website' }, signal: AbortSignal.timeout(10000) });
  if (response.status === 404) return normalizeRelease(null);
  if (!response.ok) throw new Error(`GitHub API HTTP ${response.status}`);
  return normalizeRelease(await response.json());
}
