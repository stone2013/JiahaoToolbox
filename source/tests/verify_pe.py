"""Inspect the actual PE file without executing it. Not Windows runtime testing."""
import struct,json,sys,hashlib
from pathlib import Path
path=Path(sys.argv[1]);b=path.read_bytes()
u16=lambda off:struct.unpack_from('<H',b,off)[0]
u32=lambda off:struct.unpack_from('<I',b,off)[0]
assert b[:2]==b'MZ','Missing MZ header'
pe=u32(0x3c);assert b[pe:pe+4]==b'PE\0\0','Missing PE header'
coff=pe+4;machine=u16(coff);n=u16(coff+2);opt=coff+20;optsize=u16(coff+16)
assert machine==0x8664,'Expected AMD64'
assert u16(opt)==0x20b,'Expected PE32+'
assert u16(opt+68)==2,'Expected Windows GUI subsystem'
sections=[]
for i in range(n):
 o=opt+optsize+40*i
 sections.append({'name':b[o:o+8].split(b'\0')[0].decode(),'rva':u32(o+12),'vsize':u32(o+8),'size':u32(o+16),'offset':u32(o+20)})
def fileoff(rva):
 for s in sections:
  if s['rva']<=rva<s['rva']+max(s['size'],s['vsize']):return s['offset']+rva-s['rva']
 raise ValueError('RVA outside sections')
rsrcRVA=u32(opt+112+2*8);assert rsrcRVA>0,'Resources missing'
base=fileoff(rsrcRVA);resources=[]
def walk(off,path):
 n=u16(base+off+12)+u16(base+off+14)
 for i in range(n):
  q=base+off+16+8*i;name,target=u32(q),u32(q+4);assert not name&0x80000000,'Only numeric IDs expected'
  if target&0x80000000:walk(target&0x7fffffff,path+[name])
  else:
   entry=base+target;rva,length=u32(entry),u32(entry+4);payload=b[fileoff(rva):fileoff(rva)+length]
   assert len(payload)==length
   resources.append({'ids':path+[name],'bytes':length,'sha256':hashlib.sha256(payload).hexdigest()})
   if path[0]==24:
    text=payload.decode('utf-8');assert 'PerMonitorV2' in text and 'asInvoker' in text
walk(0,[])
assert any(x['ids'][0]==24 for x in resources),'Manifest absent'
assert any(x['ids'][0]==14 for x in resources),'Group icon absent'
assert sum(x['ids'][0]==3 for x in resources)>=5,'Icon sizes absent'
print(json.dumps({'file':path.name,'architecture':'AMD64','format':'PE32+','subsystem':'Windows GUI','size':len(b),'sha256':hashlib.sha256(b).hexdigest(),'resources':resources,'passed':True,'scope':'Static file inspection only; not executed on Windows'},indent=2))
