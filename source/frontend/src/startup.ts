// Standalone watchdog: runs before the application module and survives its load failure.
interface Window {jiahaoStartup?:{stage:(message:string)=>void;finish:()=>Promise<void>;fail:(message:string)=>void;dispose:()=>void}}
(()=>{
 const root=document.documentElement,layer=document.getElementById('startup'),status=document.getElementById('startup-status');
 if(!layer)return;
 const start=performance.now();let done=false,disposed=false,deadline:ReturnType<typeof setTimeout>,fallback:ReturnType<typeof setTimeout>|undefined;
 const reduced=()=>root.dataset.motion==='off'||root.classList.contains('no-animation')||matchMedia('(prefers-reduced-motion: reduce)').matches;
 if((navigator.hardwareConcurrency&&navigator.hardwareConcurrency<=2)||matchMedia('(prefers-reduced-motion: reduce)').matches)root.dataset.motion='off';
 if(root.dataset.theme==='system')root.dataset.theme=matchMedia('(prefers-color-scheme: light)').matches?'light':'dark';
 const stage=(message:string)=>{if(status&&!done&&!disposed)status.textContent=message};
 const cleanup=()=>{clearTimeout(deadline);clearTimeout(fallback);window.removeEventListener('error',onError,true);window.removeEventListener('unhandledrejection',onRejection)};
 const fail=(message:string)=>{if(done||disposed)return;cleanup();layer.classList.add('startup-error');layer.setAttribute('aria-busy','false');if(status)status.textContent=message;document.getElementById('startup-retry')?.removeAttribute('hidden');document.getElementById('startup-retry')?.addEventListener('click',()=>location.reload(),{once:true});root.dataset.startup='error';performance.mark('startup-error')};
 function onError(event:Event){const target=event.target;if(target instanceof HTMLScriptElement||target instanceof HTMLLinkElement)fail('界面资源加载失败，请重新启动或使用兼容模式。');else if(event instanceof ErrorEvent)fail('界面初始化失败：'+(event.message||'JavaScript 错误'));}
 function onRejection(){fail('界面初始化发生错误，请重新启动或查看应用日志。');}
 const dispose=()=>{disposed=true;cleanup()};
 const finish=async()=>{if(done||disposed||root.dataset.startup==='error')return;done=true;cleanup();layer.setAttribute('aria-busy','false');if(status)status.textContent='正在进入主界面…';performance.mark('startup-ready');root.dataset.startupReadyMS=performance.now().toFixed(2);
  // Initialization runs in parallel; keep the brand entrance visible for its designed timeline.
  if(!reduced())await new Promise<void>(resolve=>{fallback=setTimeout(resolve,Math.max(0,950-(performance.now()-start)))});
  if(disposed)return;
  // Two paint opportunities expose the mounted dashboard before the crossfade.
  await new Promise<void>(resolve=>{let settled=false;const next=()=>{if(!settled){settled=true;resolve()}};fallback=setTimeout(next,80);requestAnimationFrame(()=>requestAnimationFrame(next))});clearTimeout(fallback);if(disposed)return;
  root.dataset.startup='ready';layer.classList.add('startup-leaving');
  await new Promise<void>(resolve=>{const end=(event?:TransitionEvent)=>{if(event&&event.target!==layer)return;layer.removeEventListener('transitionend',end);clearTimeout(fallback);resolve()};layer.addEventListener('transitionend',end);fallback=setTimeout(()=>end(),reduced()?0:360)});
  if(disposed)return;layer.remove();root.dataset.startup='complete';performance.mark('startup-complete');root.dataset.startupCompleteMS=performance.now().toFixed(2);
 };
 // Measurements report readiness and fade separately; no invented loading percentages.
 window.jiahaoStartup={stage,fail,finish:async()=>{await finish();if(!disposed&&root.dataset.startup==='complete'){performance.measure('startup-visible',{start,end:performance.now()})}},dispose};
 deadline=setTimeout(()=>fail('启动通信或界面初始化超过 15 秒，请重试或使用兼容模式。'),15000);
 window.addEventListener('error',onError,true);window.addEventListener('unhandledrejection',onRejection);window.addEventListener('pagehide',dispose,{once:true});
 stage('正在连接本地服务…');
 // Install error handling before initiating the one and only application module load.
 const module=document.createElement('script');module.type='module';module.src='/assets/main.js';document.head.append(module);
})();
