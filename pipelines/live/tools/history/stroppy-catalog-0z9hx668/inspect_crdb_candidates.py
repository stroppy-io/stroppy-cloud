import json,urllib.request,urllib.parse,hashlib,datetime,re,collections,sys
from pathlib import Path
p=Path('/tmp/stroppy-catalog-0z9hx668/crdb-matrix');name=sys.argv[1];state=json.loads((p/'state.json').read_text())['cells'][name];result=json.loads((p/(name+'.result.json')).read_text());parse=lambda s:datetime.datetime.fromisoformat(s.replace('Z','+00:00'));start=parse(result['segments'][0]['started_at']);end=parse(result['segments'][-1]['finished_at'])
query='"graphene.namespace":="t-stroppy-live" AND "graphene.run":='+json.dumps(state['run_id'])+' AND "graphene.entity":~"cockroach$"'
req=urllib.request.Request('http://127.0.0.1:19428/select/logsql/query',data=urllib.parse.urlencode({'query':query,'limit':20000}).encode());rows=[json.loads(l) for l in urllib.request.urlopen(req,timeout=45) if l.strip()];assert len(rows)<20000;(p/(name+'.raw-component-logs.json')).write_text(json.dumps(rows));groups=collections.defaultdict(list);items=[]
for row in rows:
 msg=row.get('_msg','')
 if not (start<=parse(row['_time'])<=end and re.match(r'^[EF][0-9]{6}\s',msg)):continue
 items.append({'entity':row['graphene.entity'],'observed_at':row['_time'],'message_sha256':hashlib.sha256(msg.encode()).hexdigest(),'message':msg})
 key='lease queue' if 'needs lease, not adding:' in msg else 'compaction cancelled' if 'compaction cancelled by a concurrent operation' in msg else re.sub(r'\b[0-9]+\b','#',msg.split('] ',1)[-1])
 groups[key].append(msg)
(p/(name+'.log-candidates.json')).write_text(json.dumps(items,indent=2));print('candidates',len(items),'groups',len(groups))
for key,messages in list(groups.items())[:8]:print(len(messages),messages[0][:1400])
