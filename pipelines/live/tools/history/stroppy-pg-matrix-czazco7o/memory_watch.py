import json,time,datetime,urllib.request,urllib.parse
from pathlib import Path
p=Path('/tmp/stroppy-pg-matrix-czazco7o/memory-observations.jsonl')
for _ in range(240):
 try:
  q='container_memory_working_set_bytes{namespace="graphene",container="server"}'
  r=json.load(urllib.request.urlopen('http://127.0.0.1:18428/api/v1/query?'+urllib.parse.urlencode({'query':q}),timeout=15))
  row={'time':datetime.datetime.now(datetime.timezone.utc).isoformat(),'samples':[{'pod':x['metric'].get('pod'),'bytes':float(x['value'][1])} for x in r.get('data',{}).get('result',[])]}
  with p.open('a') as f:f.write(json.dumps(row)+'\n')
  print(row,flush=True)
 except Exception as e:print(type(e).__name__,str(e)[:150],flush=True)
 time.sleep(30)
