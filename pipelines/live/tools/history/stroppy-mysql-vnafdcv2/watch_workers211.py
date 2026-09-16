import json,subprocess,time,datetime
from pathlib import Path
p=Path(__file__).parent;state=json.loads((p/'family211/state.json').read_text());wanted={v['run_id'] for n,v in state['cells'].items() if n.startswith('mysql')}
k=['kubectl','--kubeconfig',str(p/'kubeconfig'),'--context','stroppy-live-test']
for _ in range(30):
 d=json.loads(subprocess.check_output([*k,'get','deployments','-n','graphene','-o','json'],text=True));rows=[x for x in d['items'] if x['metadata'].get('labels',{}).get('graphene.io/run') in wanted]
 if len(rows)==2 and all(x.get('status',{}).get('readyReplicas')==1 for x in rows):break
 time.sleep(10)
else:raise SystemExit('Two independently ready workers were not observed')
assert len({x['metadata']['name'] for x in rows})==2
proof=[]
for x in rows:
 run=x['metadata']['labels']['graphene.io/run'];container=next(c for c in x['spec']['template']['spec']['containers'] if c['name']=='run');env={e['name']:e.get('value') for e in container['env']};assert env['GRAPHENE_RUN_ID']==run;assert env['GRAPHENE_NAMESPACE']=='t-stroppy-live'
 proof.append({'run_id':run,'deployment':x['metadata']['name'],'ready_replicas':x['status']['readyReplicas'],'worker_image':container['image']})
live=Path('/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live');(live/'graphene211-workers.json').write_text(json.dumps({'status':'passed','checked_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'checks':proof},indent=2)+'\n');f=live/'graphene211-check.json';d=json.loads(f.read_text());d['live_parallel_validation']='passed: two long sibling run IDs have distinct ready Deployments and matching worker identities';f.write_text(json.dumps(d,indent=2)+'\n');print('Graphene 0.2.11: two independent live workers verified',flush=True)
