import fs from 'node:fs';
import path from 'node:path';
const root = path.resolve('dist'), base = '/JiahaoToolbox/';
let checked = 0;
for (const file of ['index.html', '404.html']) {
  const html = fs.readFileSync(path.join(root, file), 'utf8');
  if (/__(?:RELEASE_)[A-Z_]+__/.test(html)) throw new Error('Unresolved release template');
  const ids = new Set([...html.matchAll(/\bid="([^"]+)"/g)].map(x => x[1]));
  const values = [...html.matchAll(/\b(?:src|href|poster)="([^"]+)"/g)].map(x => x[1]);
  for (const [, srcset] of html.matchAll(/\bsrcset="([^"]+)"/g)) values.push(...srcset.split(',').map(x => x.trim().split(/\s/)[0]));
  for (const value of values) {
    if (/^https?:/.test(value)) continue;
    if (value.startsWith('#')) { if (!ids.has(value.slice(1))) throw new Error(`Missing anchor ${value} in ${file}`); continue; }
    if (!value.startsWith(base)) throw new Error(`Wrong Pages prefix: ${value}`);
    const rel = value.slice(base.length).split(/[?#]/)[0] || 'index.html';
    if (!fs.existsSync(path.join(root, rel))) throw new Error(`Missing static file: ${value}`);
    checked++;
  }
}
for (const required of ['robots.txt', 'sitemap.xml', '.nojekyll', 'images/og-cover.png', 'media/jiahao-toolbox-promo.mp4']) if (!fs.existsSync(path.join(root, required))) throw new Error(`Missing ${required}`);
console.log(`Verified ${checked} local references, HTML anchors, Pages prefix, 404, SEO files and video.`);
