import {chromium} from '../frontend/node_modules/playwright-core/index.mjs';
import {spawn} from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import assert from 'node:assert/strict';
const root=path.resolve(import.meta.dirname,'..');
const out=path.join(root,'test-output','v4.1');fs.mkdirSync(out,{recursive:true});
const exe=process.env.JIAHAO_TEST_EXE||[path.resolve(root,'../JiahaoToolbox_v4.2.exe'),path.resolve(root,'../release-v4.2/JiahaoToolbox_v4.2.exe'),path.resolve(root,'build/JiahaoToolbox_v4.2.exe')].find(p=>fs.existsSync(p));
const child=spawn(exe,['--serve','--data-dir',path.join(out,'browser-profile')],{windowsHide:true});
let browser;const evidence=[];
try{
 const url=await new Promise((resolve,reject)=>{let output='';const timer=setTimeout(()=>reject(new Error('server timeout')),20000);child.stdout.on('data',b=>{output+=b;const match=output.match(/http:\/\/127\.0\.0\.1:\d+\/start\/\S+/);if(match){clearTimeout(timer);resolve(match[0]);}});child.on('exit',code=>reject(new Error('server exit '+code)));});
 browser=await chromium.launch({channel:'msedge',headless:true});
 for(const [width,height,dpi] of [[1536,1000,1],[1024,768,1.5],[640,600,2]]){
 const context=await browser.newContext({viewport:{width,height},deviceScaleFactor:dpi});
 await context.route('**/*',route=>route.request().url().startsWith(new URL(url).origin)?route.continue():route.abort());
 const page=await context.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message));await page.goto(url);await page.locator('.dashboard').waitFor();
 const input=page.locator('#global-search');await input.fill('xitong');await page.locator('#search-results button').first().waitFor();assert.ok(await page.locator('#search-results').innerText().then(s=>s.includes('系统工具')));
 const panel=await page.locator('#search-results').boundingBox();const controls=await page.locator('.window-actions').boundingBox();assert.ok(panel.y>=controls.y+controls.height,JSON.stringify({panel,controls}));assert.ok(panel.x>=0&&panel.x+panel.width<=width);
 await page.screenshot({path:path.join(out,`search-${width}-${dpi}x.png`)});
 await input.fill('xtgj');await input.press('Enter');await page.waitForFunction(()=>document.querySelector('.nav-item.active')?.getAttribute('data-route')==='tools');assert.ok(await page.locator('#search-results').isHidden());
 await input.fill('cpu');await page.locator('#search-results button').first().click();await page.locator('h1').filter({hasText:'CPU 监控'}).waitFor();
 await input.fill('rwglq');assert.ok((await page.locator('#search-results').innerText()).includes('任务管理器'));
 await input.press('Escape');assert.ok(await page.locator('#search-results').isHidden());await page.keyboard.press('Control+k');assert.ok(await input.evaluate(e=>document.activeElement===e));assert.ok(await page.locator('#search-results').isVisible());
 await input.fill('xitong');await input.press('ArrowDown');assert.equal(await page.locator('#search-results [aria-selected=true]').getAttribute('data-search-index'),'1');
 for(const route of ['overview','cpu','gpu','memory','disk','network','hardware','tools','diagnostics','settings','about']){await page.locator(`.nav-item[data-route=${route}]`).click();assert.equal(await page.locator('.nav-item.active').getAttribute('data-route'),route);}
 assert.deepEqual(errors,[]);evidence.push({width,height,deviceScaleFactor:dpi,search:true,navigationPages:11,windowControlsClear:true,consoleErrors:errors});await context.close();
 }
 fs.writeFileSync(path.join(out,'browser-evidence.json'),JSON.stringify({environment:'Windows / headless Microsoft Edge, actual EXE local server',tests:evidence,externalTools:'Not launched in browser tests; existing API dispatch verified separately'},null,2));console.log('PASS: real EXE backend + Edge search, keyboard, 11 pages, 3 DPI/layout configurations');
}finally{await browser?.close();child.kill();}
