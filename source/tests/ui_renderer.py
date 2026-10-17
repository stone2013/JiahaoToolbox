"""Offline browser renderer with a real HTTP transport bridge.
The renderer does not navigate to URLs or alter Chromium management policies.
Only the test transport is replaced. All data comes from the running Go process.
"""
import os
from pathlib import Path
import re,base64,json,urllib.request,urllib.error,http.cookiejar
from playwright.sync_api import sync_playwright
ROOT=Path(__file__).resolve().parents[1]/'frontend/dist'
url=os.environ.get('JIAHAO_TEST_URL','')
if not url: raise RuntimeError('Set JIAHAO_TEST_URL to the localhost /start/... address from --serve; use an isolated test profile.')
origin=url.split('/start/')[0]
opener=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
opener.open(url).read()

def request(path,init=None):
    init=init or {}
    if not path.startswith('/api/'): raise ValueError('Test harness allows local API only')
    headers=dict(init.get('headers',{}))
    if init.get('method')=='POST':headers['Origin']=origin
    body=init.get('body')
    req=urllib.request.Request(origin+path,data=body.encode() if body is not None else None,headers=headers,method=init.get('method','GET'))
    try:
        r=opener.open(req,timeout=15)
    except urllib.error.HTTPError as err:r=err
    return dict(status=r.status,body=r.read().decode('utf-8'),headers=dict(r.headers))

def data_uri(path,kind):return 'data:'+kind+';base64,'+base64.b64encode(path.read_bytes()).decode()
logo=data_uri(ROOT/'assets/assets/logo.svg','image/svg+xml')
landscape=data_uri(ROOT/'assets/assets/mountains.svg','image/svg+xml')
cache={}
def js_module(path):
    path=path.resolve()
    if path in cache:return cache[path]
    code=path.read_text().replace('/assets/assets/logo.svg',logo)
    code=re.sub(r"from\s+(['\"])([^'\"]+)\1",lambda m:'from '+json.dumps(js_module(path.parent/m[2])),code)
    uri='data:text/javascript;base64,'+base64.b64encode(code.encode()).decode();cache[path]=uri;return uri

def mount(page):
    page.expose_function('__localRequest',request)
    document=(ROOT/'index.html').read_text()
    document=re.sub(r'<script[^>]*>.*?</script>','',document,flags=re.S)
    document=re.sub(r'<link[^>]*>','',document)
    document=document.replace('/assets/assets/logo.svg',logo)
    css=(ROOT/'assets/styles/app.css').read_text().replace('../assets/mountains.svg',landscape)
    document=document.replace('</head>','<style>'+css+'</style></head>')
    page.set_content(document)
    page.add_script_tag(content="""window.fetch=async function(path,init={}) {
      if(init.signal?.aborted)throw new DOMException('Aborted','AbortError');
      const response=await window.__localRequest(path,{method:init.method||'GET',headers:init.headers||{},body:init.body});
      if(init.signal?.aborted)throw new DOMException('Aborted','AbortError');
      return new Response(response.body,{status:response.status,headers:response.headers});
    };""")
    page.add_script_tag(type='module',content='import '+json.dumps(js_module(ROOT/'assets/main.js'))+';')
    page.wait_for_selector('.dashboard',timeout=15000)

