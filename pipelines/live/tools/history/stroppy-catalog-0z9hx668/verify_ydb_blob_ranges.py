import urllib.request,hashlib,json,time,datetime
from pathlib import Path
expected='600d3ddf7ce26a191d1724cf6855f19ed133c4060e76515597495f1b06c2df50';url='https://docker.stroppy.io/v2/stroppy-io/ydb-local/blobs/sha256:'+expected
size=272307622;chunk_size=8*1024*1024;h=hashlib.sha256()
for begin in range(0,size,chunk_size):
 end=min(size-1,begin+chunk_size-1)
 for attempt in range(4):
  try:
   with urllib.request.urlopen(urllib.request.Request(url,headers={'Range':f'bytes={begin}-{end}','Accept-Encoding':'identity'}),timeout=30) as r:
    assert r.status==206 and r.headers.get('Content-Range')==f'bytes {begin}-{end}/{size}'
    b=r.read();assert len(b)==end-begin+1,(begin,len(b),end-begin+1)
   break
  except Exception as e:
   print('range retry',begin,attempt,type(e).__name__,flush=True)
   if attempt==3:raise
   time.sleep(1)
 h.update(b)
 if begin//chunk_size%4==0:print('verified download bytes',end+1,'/',size,flush=True)
assert h.hexdigest()==expected,(h.hexdigest(),expected)
p=Path('/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live/ydb-26.1-mirror-check.json');d=json.loads(p.read_text());d['status']='passed';d['image_id']=d.pop('image_config_digest',d.get('image_id'));d['same_manifest_digest']=d['pinned'].endswith('@'+d['source_digest']);d['required_blob_download']={'bytes':size,'sha256':h.hexdigest(),'status':'passed','method':'complete byte coverage via checked HTTP ranges; each response length and Content-Range validated'};d.pop('download_check',None);d.pop('actual_download_sha256',None);d['checked_at']=datetime.datetime.now(datetime.timezone.utc).isoformat();p.write_text(json.dumps(d,indent=2)+'\n');print('Blob SHA-256 verified',h.hexdigest(),flush=True)
