import json,sys,urllib.parse,urllib.request
from pathlib import Path
rid=sys.argv[1];out=Path(sys.argv[2])
q='"graphene.namespace":="t-stroppy-live" AND "graphene.run":='+json.dumps(rid)+' AND "graphene.activity":="stroppy.segment.run" AND "insert progress"'
r=urllib.request.urlopen(urllib.request.Request('http://127.0.0.1:19428/select/logsql/query',data=urllib.parse.urlencode({'query':q,'limit':1000}).encode()),timeout=30)
rows=[json.loads(l) for l in r if l.strip()];assert len(rows)<1000
samples=[]
for row in rows:
 try:m=json.loads(row.get('_msg','{}'))
 except ValueError:continue
 if m.get('event')!='completed':continue
 sample={k:m.get(k) for k in ['workload','event','table','method','rows','total_rows','generated_rows','confirmed_rows','inflight_rows','percent']}
 assert sample['generated_rows']==sample['confirmed_rows']==sample['total_rows'],sample
 assert sample['inflight_rows']==0 and abs(sample['percent']-100)<0.0001,sample
 samples.append(sample)
assert samples
out.write_text(json.dumps({'status':'passed','run_id':rid,'source':'persisted native Stroppy insert-progress logs, exact namespace and run selector','samples':samples},indent=2)+'\n')
print(rid,len(samples),'completed load progress samples verified')
