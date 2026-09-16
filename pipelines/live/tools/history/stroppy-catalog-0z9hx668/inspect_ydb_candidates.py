import json,urllib.request,urllib.parse,hashlib,datetime,re,collections,sys,os
from pathlib import Path
name=sys.argv[1];p=Path(sys.argv[2]) if len(sys.argv)>2 else Path('/tmp/stroppy-catalog-0z9hx668/ydb-metricsretry');state=json.loads((p/'state.json').read_text())['cells'][name];result=json.loads((p/(name+('.partial-result.json' if '--partial' in sys.argv else '.result.json'))).read_text());parse=lambda s:datetime.datetime.fromisoformat(s.replace('Z','+00:00'));start=parse(result['segments'][0]['started_at']);end=parse(result['segments'][-1]['finished_at'])
query='"graphene.namespace":="t-stroppy-live" AND "graphene.run":='+json.dumps(state['run_id'])+' AND "graphene.entity":~"ydb$"'
req=urllib.request.Request('http://127.0.0.1:19428/select/logsql/query',data=urllib.parse.urlencode({'query':query}).encode());groups=collections.Counter();items=[];count=0;target=p/(name+'.raw-component-logs.json');tmp=target.with_suffix('.json.tmp')
with urllib.request.urlopen(req,timeout=45) as response,tmp.open('w') as out:
 out.write('[')
 for line in response:
  if not line.strip():continue
  row=json.loads(line);assert row['graphene.namespace']=='t-stroppy-live' and row['graphene.run']==state['run_id']
  if count:out.write(',')
  json.dump(row,out);count+=1;msg=row.get('_msg','')
  if not (start<=parse(row['_time'])<=end and re.search(r'\b(?:ERROR|FATAL|PANIC):|\s[EF]>\s',msg)):continue
  items.append({'entity':row['graphene.entity'],'observed_at':row['_time'],'message_sha256':hashlib.sha256(msg.encode()).hexdigest(),'message':msg})
  groups[re.sub(r'\b[0-9]+\b','#',msg.split('] ',1)[-1])]+=1
 out.write(']')
os.replace(tmp,target);(p/(name+'.log-candidates.json')).write_text(json.dumps(items,indent=2));print('records',count,'candidates',len(items),'distinct patterns',len(groups))
