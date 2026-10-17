"""Offline renderer + real Go HTTP tests. Never modifies browser management policy.
Run with JIAHAO_TEST_URL, optionally JIAHAO_CHROMIUM and JIAHAO_TEST_OUTPUT.
UI routing/geometry checks do NOT establish Windows native WebView2/DPI correctness.
"""
from pathlib import Path
import os,json,time,math,platform,sys
from ui_renderer import mount,request
from playwright.sync_api import sync_playwright
OUT=Path(os.environ.get('JIAHAO_TEST_OUTPUT','test-output/ui'));OUT.mkdir(parents=True,exist_ok=True)
report={'scope':'Linux Chromium offline renderer, actual Go HTTP transport bridge; NOT Windows/WebView2 testing','platform':platform.platform(),'matrix':[],'interactions':[],'errors':[]}
def check(name,ok,detail=''):
 report['interactions'].append({'test':name,'passed':bool(ok),'detail':detail})
 if not ok:raise AssertionError(name+': '+str(detail))
def settings_patch(patch):
 boot=json.loads(request('/api/bootstrap')['body']);s=boot['settings'];s.update(patch)
 r=request('/api/settings',{'method':'POST','body':json.dumps(s),'headers':{'Content-Type':'application/json','X-Jiahao-Request':'1'}})
 if r['status']!=200:raise RuntimeError(r)
def clickroute(page,route):
 page.locator('.nav-item[data-route="'+route+'"]').click()
 page.wait_for_timeout(60)
def geometry(page):return page.evaluate('''()=>{
 const main=document.querySelector('#main'),h=document.querySelector('.topbar');
 const elements=[...document.querySelectorAll('button:not([disabled]),input,select')];
 return {bodyOverflow:document.body.scrollWidth>innerWidth+1, mainOverflow:main.scrollWidth>main.clientWidth+1, topOverflow:h.scrollWidth>h.clientWidth+1,
 tooWide:elements.filter(e=>{const r=e.getBoundingClientRect();return r.width>0&&(r.right>innerWidth+2||r.left< -2)}).slice(0,12).map(e=>e.id||e.className),
 viewport:[innerWidth,innerHeight],mainWidth:main.clientWidth,mainScrollWidth:main.scrollWidth};}''')
try:
 with sync_playwright() as p:
  browser=p.chromium.launch(executable_path=os.environ.get('JIAHAO_CHROMIUM','/usr/bin/chromium'),headless=True,args=['--no-sandbox'])
  settings_patch({'theme':'dark','intervalMS':1000,'units':'iec'})
  page=browser.new_page(viewport={'width':1536,'height':1024})
  page.on('pageerror',lambda e:report['errors'].append(str(e)[:500]));mount(page);page.wait_for_timeout(700)
  timings=[]
  for route in ['cpu','gpu','memory','disk','network','hardware','tools','diagnostics','settings','about','overview']:
   # Evaluate only local DOM routing latency, not Python transport/IPC latency.
   elapsed=page.evaluate('''async(route)=>{const t=performance.now();document.querySelector('.nav-item[data-route="'+route+'"]').click();await new Promise(requestAnimationFrame);return performance.now()-t;}''',route)
   timings.append({'route':route,'milliseconds':round(elapsed,2)})
   check('route '+route,page.locator('.nav-item[data-route="'+route+'"]').get_attribute('aria-current')=='page')
  report['route_latency_renderer_only']=timings
  page.keyboard.press('Control+k');check('Ctrl+K search focus',page.locator('#global-search').evaluate('(e)=>e===document.activeElement'))
  page.locator('#global-search').fill('内存');page.wait_for_timeout(120)
  check('search page result',page.locator('.search-result').count()>0)
  page.locator('.search-result').first.click();check('search navigation',page.locator('.nav-item[data-route="memory"]').get_attribute('aria-current')=='page')
  clickroute(page,'settings');page.locator('#setting-theme').select_option('light');page.wait_for_timeout(150)
  check('light theme applied',page.locator('html').get_attribute('data-theme')=='light')
  check('theme persisted',json.loads(request('/api/bootstrap')['body'])['settings']['theme']=='light')
  page.locator('#setting-units').select_option('si');page.wait_for_timeout(100)
  check('SI units persisted',json.loads(request('/api/bootstrap')['body'])['settings']['units']=='si')
  page.locator('#setting-interval').select_option('500');page.wait_for_timeout(200)
  clickroute(page,'overview');page.locator('[data-action="pause"]').click();page.wait_for_timeout(220)
  n=json.loads(request('/api/snapshot')['body'])['samples'];page.wait_for_timeout(1300)
  check('pause really stops sampler',json.loads(request('/api/snapshot')['body'])['samples']==n)
  page.locator('[data-action="pause"]').click();page.wait_for_timeout(1100)
  check('resume really samples',json.loads(request('/api/snapshot')['body'])['samples']>n)
  clickroute(page,'diagnostics');page.locator('[data-action="diagnose"]').click();page.wait_for_timeout(250)
  d=json.loads(request('/api/report')['body']);check('real diagnostic report',len(d.get('checks',[]))==5 and 'coverage' in d)
  check('missing hardware never forced 100',d['score'] is None or d['coverage']>=80)
  page.locator('[data-action="export-report"]').click();page.wait_for_timeout(80)
  check('export privacy confirmation',page.locator('dialog[open]').count()>0)
  # Escape cancels: no file export or system modification performed by this test.
  page.keyboard.press('Escape');page.wait_for_timeout(80)
  check('export cancelled by Escape',page.locator('dialog[open]').count()==0)
  clickroute(page,'hardware');page.locator('#hardware-filter').fill('not-present-xyz');page.wait_for_timeout(150)
  check('hardware empty search feedback','没有匹配' in page.locator('#hardware-details').inner_text())
  clickroute(page,'tools');check('non-Windows tools disabled',page.locator('[data-tool]:disabled').count()==10)
  # No automatic request to any external destination is performed.
  clickroute(page,'network');page.locator('#network-target').fill('bad host / path');page.locator('button[value="dns"]').click();page.wait_for_timeout(80)
  if page.locator('dialog[open]').count():page.keyboard.press('Escape')
  clickroute(page,'settings');page.locator('#setting-theme').select_option('dark');page.locator('#setting-units').select_option('iec');page.locator('#setting-interval').select_option('1000');page.wait_for_timeout(250)
  clickroute(page,'overview');page.screenshot(path=str(OUT/'overview-1536x1024-dark.png'))
  page.close()
  for physical in [(1366,768),(1920,1080),(2560,1440),(3840,2160)]:
   for dpr in [1,1.25,1.5,1.75,2,2.5,3]:
    css={'width':math.floor(physical[0]/dpr),'height':math.floor(physical[1]/dpr)}
    page=browser.new_page(viewport=css,device_scale_factor=dpr)
    page.on('pageerror',lambda e:report['errors'].append(str(e)[:500]));mount(page);page.wait_for_timeout(80)
    g=geometry(page);row={'physical':physical,'scale':dpr,'css':css,'geometry':g,'passed':not(g['bodyOverflow'] or g['mainOverflow'] or g['topOverflow'] or g['tooWide'])};report['matrix'].append(row)
    if physical==(3840,2160) and dpr==2:page.screenshot(path=str(OUT/'overview-3840x2160-200pct.png'))
    if physical==(1366,768) and dpr==3:page.screenshot(path=str(OUT/'overview-1366x768-300pct.png'))
    # All page layouts at smallest CSS viewport (key high-DPI stress case).
    if physical==(1366,768) and dpr==3:
     for route in ['cpu','gpu','memory','disk','network','hardware','tools','diagnostics','settings','about']:
      clickroute(page,route);r=geometry(page);report['matrix'].append({'page':route,'physical':physical,'scale':dpr,'geometry':r,'passed':not(r['bodyOverflow'] or r['mainOverflow'] or r['topOverflow'] or r['tooWide'])})
    page.close()
  settings_patch({'theme':'light'})
  page=browser.new_page(viewport={'width':1920,'height':1080},device_scale_factor=1);mount(page);page.wait_for_timeout(200)
  g=geometry(page);check('light 1080P layout',not(g['bodyOverflow'] or g['mainOverflow'] or g['topOverflow']))
  page.screenshot(path=str(OUT/'overview-1920x1080-light.png'));page.close();settings_patch({'theme':'dark'})
  browser.close()
except Exception as e:
 report['fatal']=str(e)
finally:
 report['passed']=not report.get('fatal') and not report['errors'] and all(x['passed'] for x in report['matrix']) and all(x['passed'] for x in report['interactions'])
 (OUT/'ui-results.json').write_text(json.dumps(report,ensure_ascii=False,indent=2))
 print(json.dumps(report,ensure_ascii=False,indent=2))
 if not report['passed']:sys.exit(1)
