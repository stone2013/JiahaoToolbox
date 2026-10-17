export class ApiError extends Error {constructor(message:string, public status:number,public needsElevation=false){super(message)}}
export async function api<T>(path:string,body?:unknown,signal?:AbortSignal):Promise<T>{
 const abort=new AbortController();const timeout=setTimeout(()=>abort.abort(),12000);
 const forward=()=>abort.abort();signal?.addEventListener('abort',forward,{once:true});if(signal?.aborted)abort.abort();
 try{const r=await fetch('/api/'+path,{method:body===undefined?'GET':'POST',credentials:'same-origin',headers:body===undefined?{}:{'Content-Type':'application/json','X-Jiahao-Request':'1'},body:body===undefined?undefined:JSON.stringify(body),signal:abort.signal});
 const text=await r.text();let v:unknown;try{v=JSON.parse(text)}catch{throw new ApiError(r.status===401?'本地会话已结束，请重新启动工具箱':'本地服务返回了无效响应',r.status)}
 if(!r.ok){const e=v as {error?:string;needsElevation?:boolean};throw new ApiError(e.error||'请求失败',r.status,!!e.needsElevation)};return v as T;
 }catch(e){if(e instanceof DOMException&&e.name==='AbortError')throw new Error(signal?.aborted?'操作已取消':'本地请求超时，请查看日志');throw e}finally{clearTimeout(timeout);signal?.removeEventListener('abort',forward)}
}
