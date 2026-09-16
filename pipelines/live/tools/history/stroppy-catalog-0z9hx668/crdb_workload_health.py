import json,base64,subprocess,concurrent.futures,sys
from pathlib import Path
p=Path('/tmp/stroppy-catalog-0z9hx668/crdb-matrix');name=sys.argv[1];s=json.loads((p/'state.json').read_text())['cells'][name];r=json.loads((p/(name+'.result.json')).read_text());start=r['segments'][0]['started_at'];end=r['segments'][-1]['finished_at'];cli='/tmp/stroppy-catalog-0z9hx668/bin/graphenectl'
def read(node):
 entity=s['uuid']+'-'+node+'-cockroach';x=subprocess.run([cli,'-n','t-stroppy-live','metrics','docker',entity,'--start',start,'--end',end,'--jq','.'],capture_output=True,text=True,timeout=60);x.check_returncode();data=json.loads(base64.b64decode(json.loads(x.stdout)['snapshot']))['data']['result'];out=[]
 for row in data:
  n=row['metric'].get('__name__','')
  if n in ['liveness_livenodes','ranges_unavailable','ranges_underreplicated']:
   vals=[float(v) for _,v in row['values']];out.append({'name':n,'min':min(vals),'max':max(vals),'last':vals[-1],'first_at':row['values'][0][0],'last_at':row['values'][-1][0]})
 return node,out
out=dict(concurrent.futures.ThreadPoolExecutor(max_workers=3).map(read,['db-1','db-2','db-3']));(p/(name+'.workload-health.json')).write_text(json.dumps(out,indent=2));print(json.dumps(out,indent=2))
