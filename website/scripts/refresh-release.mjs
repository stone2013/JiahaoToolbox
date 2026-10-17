import fs from 'node:fs';
import { fetchRelease } from './release-data.mjs';
const target = new URL('../src/release.json', import.meta.url);
try {
  const release = await fetchRelease();
  fs.writeFileSync(target, JSON.stringify(release, null, 2) + '\n');
  console.log(release.available ? `Verified official release v${release.version}` : 'No published stable release (404)');
} catch (error) {
  if (!fs.existsSync(target)) throw error;
  console.warn(`${error.message}; keeping the last verified snapshot. The page displays its verification time.`);
}
