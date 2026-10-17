import {searchIndex} from './search-index.js';
import {esc} from './format.js';
export interface SearchEntry {id:string;title:string;english:string;icon:string;type:'page'|'tool';route?:string;action?:string;pinyin:string;initials:string;keywords:readonly string[];aliases:readonly string[];syllables:readonly string[];terms:readonly {title:string;pinyin:string;initials:string}[]}
export interface SearchResult {entry:SearchEntry;score:number;matched:string}
export const normalize=(s:string)=>s.normalize('NFKC').toLowerCase().replace(/[\s'’_-]+/g,'');
// Only one edit on sufficiently long words. Short initials never use fuzzy matching.
function near(a:string,b:string):boolean {if(Math.abs(a.length-b.length)>1)return false;let i=0,j=0,edits=0;while(i<a.length&&j<b.length){if(a[i]===b[j]){i++;j++;continue}if(++edits>1)return false;if(a.length>=b.length)i++;if(b.length>=a.length)j++;}return edits+(i<a.length||j<b.length?1:0)<=1;}
export class SearchService {
 private cache=new Map<string,SearchResult[]>();
 private entries:SearchEntry[];
 constructor(entries:readonly SearchEntry[]=searchIndex){this.entries=[...new Map(entries.map(e=>[e.type+':'+e.id,e])).values()];}
 search(query:string):SearchResult[]{const q=normalize(query).slice(0,120);if(!q)return this.entries.filter(e=>e.type==='page').slice(0,7).map(entry=>({entry,score:0,matched:''}));if(!/^[\p{L}\p{N}]+$/u.test(q))return [];const cached=this.cache.get(q);if(cached)return cached;
 const results:SearchResult[]=[];
 for(const e of this.entries){const title=normalize(e.title),en=normalize(e.english);let score=Infinity,matched=e.title;
 const accept=(s:number,label:string)=>{if(s<score){score=s;matched=label}};
 if(title===q)accept(0,e.title);if(en===q)accept(1,e.english);if(e.pinyin===q)accept(2,e.title);if(e.initials===q)accept(3,e.title);
 if(title.startsWith(q)||en.startsWith(q))accept(4,title.startsWith(q)?e.title:e.english);
 if(e.pinyin.startsWith(q)||e.initials.startsWith(q))accept(5,e.title);
 for(const term of e.terms){if([normalize(term.title),term.pinyin,term.initials].some(t=>t===q||t.startsWith(q)))accept(6,term.title);}
 if(e.keywords.some(t=>normalize(t).startsWith(q)))accept(6,e.english);
 // Mixed Chinese and pinyin can be typed per syllable, e.g. 系统gongju.
 if(/[\u3400-\u9fff]/.test(q)&&/[a-z]/.test(q)){const chars=Array.from(e.title);let positions=new Set([0]);chars.forEach((char,i)=>{const next=new Set<number>();for(const pos of positions)for(const token of [normalize(char),normalize(e.syllables[i]??char)])if(q.startsWith(token,pos))next.add(pos+token.length);positions=next;});if(positions.has(q.length))accept(6,e.title);}
 if(title.includes(q)||en.includes(q))accept(7,title.includes(q)?e.title:e.english);
 if(q.length>=5&&/^[a-z]+$/.test(q)){for(const [token,label] of [[e.pinyin,e.title],[en,e.english],...e.terms.map(t=>[t.pinyin,t.title])])if(near(q,token))accept(8,label);}
 if(Number.isFinite(score))results.push({entry:e,score,matched});}
 results.sort((a,b)=>a.score-b.score);if(this.cache.size>=100)this.cache.delete(this.cache.keys().next().value!);this.cache.set(q,results);return results;}
}
export function highlight(value:string,query:string):string {const q=normalize(query);if(!q)return esc(value);const chars=Array.from(value);const normalized=chars.map(normalize);const flat=normalized.join('');const start=flat.indexOf(q);if(start<0)return esc(value);let offset=0;return chars.map((c,i)=>{const hit=offset<start+q.length&&offset+normalized[i].length>start;offset+=normalized[i].length;return hit?`<mark>${esc(c)}</mark>`:esc(c)}).join('');}
export function dispatchSearch(entry:SearchEntry,navigate:(id:string)=>void,tool:(id:string)=>void){if(entry.type==='page'&&entry.route)navigate(entry.route);else if(entry.type==='tool'&&entry.action)tool(entry.action);}

export function highlightTitle(entry:SearchEntry,query:string):string {const q=normalize(query);if(!q||normalize(entry.title).includes(q)||!/^[\u3400-\u9fff]+$/.test(entry.title))return highlight(entry.title,query);const initial=entry.initials.startsWith(q),full=entry.pinyin.startsWith(q);if(!initial&&!full)return esc(entry.title);let length=0;return Array.from(entry.title).map((char,i)=>{const hit=initial?i<q.length:length<q.length;length+=entry.syllables[i]?.length??0;return hit?'<mark>'+esc(char)+'</mark>':esc(char)}).join('');}
