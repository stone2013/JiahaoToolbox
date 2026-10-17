import { test, expect } from '@playwright/test';
import fs from 'node:fs';
const release = JSON.parse(fs.readFileSync(new URL('../src/release.json', import.meta.url), 'utf8'));
test.beforeEach(async ({ page }) => {
  const errors: string[] = [];
  page.on('pageerror',error=>errors.push(error.message));
  page.on('console',message=>{if(message.type()==='error')errors.push(message.text());});
  (page as unknown as { errors:string[] }).errors=errors;
});
test.afterEach(async ({ page }) => expect((page as unknown as {errors:string[]}).errors).toEqual([]));

test('首页、真实图像、SEO与响应式布局', async ({ page }, info) => {
  await page.goto('./');
  await expect(page).toHaveTitle('嘉豪工具箱 — 专业硬件监控与系统工具');
  await expect(page.getByRole('heading',{name:'嘉豪工具箱',exact:true,level:1})).toBeVisible();
  await expect(page.locator('.hero-screen img')).toHaveJSProperty('complete',true);
  expect(await page.locator('.hero-screen img').evaluate((im:HTMLImageElement)=>im.naturalWidth)).toBeGreaterThan(0);
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth)).toBe(true);
  expect(await page.locator('link[rel=canonical]').getAttribute('href')).toBe('https://stone2013.github.io/JiahaoToolbox/');
  await page.screenshot({path:`docs/qa/${info.project.name}-home.png`});
});

test('导航和功能条目可用，移动菜单支持Escape', async ({ page }) => {
  await page.goto('./');
  const mobile=(page.viewportSize()?.width??1440)<=800;
  if(mobile){await page.getByRole('button',{name:'打开导航菜单'}).click();await expect(page.locator('.menu-toggle')).toHaveAttribute('aria-expanded','true');await page.keyboard.press('Escape');await expect(page.locator('.menu-toggle')).toHaveAttribute('aria-expanded','false');await page.getByRole('button',{name:'打开导航菜单'}).click();}
  await page.locator('#site-nav a[href="#features"]').click();
  await expect(page).toHaveURL(/#features$/);
  await expect(page.locator('.feature-card')).toHaveCount(10);
  await expect(page.locator('.feature-card').first()).toBeInViewport();
  if(mobile)await expect(page.locator('.menu-toggle')).toHaveAttribute('aria-expanded','false');
  for(const id of ['screenshots','download','home']){
    if(mobile)await page.getByRole('button',{name:'打开导航菜单'}).click();
    await page.locator(`#site-nav a[href="#${id}"]`).click();
    await expect(page).toHaveURL(new RegExp(`#${id}$`));
  }
});

test('所有下载来源和GitHub按钮正确',async({page})=>{
  await page.goto('./');
  await expect(page.getByRole('link',{name:'免费下载',exact:true})).toHaveAttribute('href','#download');
  await expect(page.getByRole('link',{name:'查看 GitHub',exact:true})).toHaveCount(0);
  await page.getByRole('link',{name:'免费下载',exact:true}).click();
  await expect(page).toHaveURL(/#download$/);
  await expect(page.locator('.runtime-notice')).toBeInViewport();
  if(release.available){await expect(page.locator('.version-badge')).toHaveText('v'+release.version);for(const a of release.assets){await expect(page.locator(`.download-buttons a[href="${a.url}"]`)).toHaveCount(1);}}
  for(const a of await page.locator('.download-buttons a').all())expect(await a.getAttribute('href')).toMatch(/^https:\/\/github\.com\/stone2013\/JiahaoToolbox\/releases/);
});

test('五张真实截图可放大、Esc关闭并恢复焦点',async({page})=>{
  await page.goto('./');
  const buttons=page.locator('[data-lightbox]');
  for(let i=0;i<await buttons.count();i++){
    const button=buttons.nth(i);await button.scrollIntoViewIfNeeded();await button.click();
    await expect(page.locator('#screenshot-dialog')).toBeVisible();
    await expect(page.locator('.lightbox-image')).toHaveJSProperty('complete',true);
    expect(await page.locator('.lightbox-image').evaluate((im:HTMLImageElement)=>im.naturalWidth)).toBeGreaterThan(1000);
    await page.keyboard.press('Escape');await expect(page.locator('#screenshot-dialog')).not.toBeVisible();await expect(button).toBeFocused();
  }
});

test('视频按需加载并实际播放1080p片源',async({page})=>{
  let loaded=false;page.on('request',r=>{if(r.url().endsWith('.mp4'))loaded=true;});
  await page.goto('./');await page.waitForTimeout(300);expect(loaded).toBe(false);
  const v=page.locator('#promo-video');await v.scrollIntoViewIfNeeded();await v.evaluate((el:HTMLVideoElement)=>{el.muted=true;});
  await expect(page.getByRole('heading',{name:'官方宣传片',exact:true})).toBeVisible();
  await expect(page.getByText('SMV6 出品 · 嘉豪工具箱功能展示与实际演示',{exact:true})).toBeVisible();
  await expect(page.getByRole('link',{name:'下载嘉豪工具箱',exact:true})).toHaveAttribute('href','https://github.com/stone2013/JiahaoToolbox/releases');
  await expect(v).toHaveAttribute('controls','');
  expect(await v.getAttribute('autoplay')).toBeNull();
  const box=await v.boundingBox();expect(box).not.toBeNull();
  expect(box!.width / box!.height).toBeCloseTo(16 / 9, 2);
  await page.getByRole('button',{name:'播放宣传片',exact:true}).click();
  await expect.poll(()=>v.evaluate((el:HTMLVideoElement)=>el.currentTime)).toBeGreaterThan(0);
  const metadata=await v.evaluate((el:HTMLVideoElement)=>({width:el.videoWidth,height:el.videoHeight,duration:el.duration,error:el.error?.code??null}));
  expect(metadata.width).toBe(1920);expect(metadata.height).toBe(1080);expect(metadata.duration).toBeCloseTo(90,1);expect(metadata.error).toBeNull();expect(loaded).toBe(true);
  await v.evaluate((el:HTMLVideoElement)=>el.pause());
});

test('损坏视频源显示加载失败提示和下载链接',async({page})=>{
  await page.route('**/media/jiahao-toolbox-promo.mp4',route=>route.fulfill({status:200,contentType:'video/mp4',body:'Controlled invalid MP4 test fixture'}));
  await page.goto('./');
  await page.getByRole('button',{name:'播放宣传片',exact:true}).click();
  await expect(page.locator('#video-error')).toBeVisible();
  await expect(page.locator('#video-error a')).toHaveAttribute('href','/JiahaoToolbox/media/jiahao-toolbox-promo.mp4');
  await expect(page.getByRole('link',{name:'下载嘉豪工具箱',exact:true})).toHaveAttribute('href','https://github.com/stone2013/JiahaoToolbox/releases');
});

test('减少动画偏好与404返回链接',async({page})=>{
  await page.emulateMedia({reducedMotion:'reduce'});await page.goto('./');
  expect(await page.evaluate(()=>getComputedStyle(document.documentElement).scrollBehavior)).toBe('auto');
  await page.locator('#features').scrollIntoViewIfNeeded();
  expect(await page.locator('.feature-card').first().evaluate(el=>getComputedStyle(el).opacity)).toBe('1');
  await page.goto('./404.html');await expect(page.getByRole('heading',{name:'这个页面暂时找不到。'})).toBeVisible();
  await expect(page.getByRole('link',{name:'返回首页'})).toHaveAttribute('href','/JiahaoToolbox/');await page.getByRole('link',{name:'返回首页'}).click();await expect(page).toHaveTitle('嘉豪工具箱 — 专业硬件监控与系统工具');
});

test('子路径资源无404，所有图片可解码',async({page})=>{
  const failures:string[]=[];page.on('response',r=>{if(r.status()>=400)failures.push(r.status()+':'+r.url());});
  await page.goto('./');
  for(const im of await page.locator('img:not(.lightbox-image)').all()) {await im.scrollIntoViewIfNeeded();await expect(im).toHaveJSProperty('complete',true);expect(await im.evaluate((x:HTMLImageElement)=>x.naturalWidth)).toBeGreaterThan(0);}
  expect(failures).toEqual([]);expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
});
