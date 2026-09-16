import json,sys,datetime,subprocess,fcntl,os
from pathlib import Path
lock=open('/tmp/stroppy-catalog-0z9hx668/progress-accept.lock','a');fcntl.flock(lock,fcntl.LOCK_EX)
P=Path(sys.argv[1]);L=Path('/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live');S=json.loads((P/'state.json').read_text());path=L/'catalog-functional-check.json';d=json.loads(path.read_text());key=lambda c:(c['database'],c['version'],c['topology']);accepted=[]
for name,c in S['cells'].items():
 f=P/(name+'.verified.json')
 if not f.exists():continue
 v=json.loads(f.read_text())
 if v.get('status')!='passed' or v.get('run_id')!=c['run_id']:continue
 proofs={}
 suffixes=['native','load-progress']+(['replication'] if c['database'] not in ['ydb_managed','noop','pg_noop','external'] else [])
 for suffix in suffixes:
  f=L/(name+'.'+suffix+'.json')
  if not f.exists():break
  proof=json.loads(f.read_text())
  status=proof.get('status')
  if suffix=='load-progress' and status=='not_applicable':
   configured=next(x['run_spec'] for x in json.loads((P/'suite.json').read_text())['cells'] if x['id']==name)
   assert all(seg['workload']['script']=='execute_sql' for seg in configured['workload']['segments'])
  elif status!='passed':break
  if proof.get('run_id')!=c['run_id']:break
  proofs[suffix]=proof
 if len(proofs)!=len(suffixes):continue
 if c['database']=='external':
  external=L/'external-dsn-lifecycle-check.json'
  if not external.exists():continue
  e=json.loads(external.read_text())
  if e.get('status')!='passed' or e.get('run_id')!=c['run_id'] or e.get('fixture_cleanup')!='passed' or e.get('telemetry_verification')!='passed':continue
 remaining=[]
 for kind in ['vm','disk','network','subnet','sg','ydb']:
  f=P/(kind+'-current.json')
  if not f.exists():remaining.append('missing '+kind);continue
  remaining.extend(x['id'] for x in json.loads(f.read_text()) if c['uuid'][:8] in x.get('name',''))
 if remaining:continue
 result=json.loads((P/(name+'.result.json')).read_text());native=proofs['native'];assert len(result['segments'])==len(native['segments'])
 existing=next((x for x in d['catalog_cells'] if key(x)==key(c)),None)
 if existing and existing.get('run_id')!=c['run_id'] and existing.get('started_at'):
  parse=lambda t:datetime.datetime.fromisoformat(t.replace('Z','+00:00'))
  if parse(existing['started_at'])>parse(result['segments'][0]['started_at']):continue
 row={**c,'status':'passed','compile':'passed','local_cluster':next((x.get('local_cluster','not_verified') for x in d['catalog_cells'] if key(x)==key(c)), 'not_applicable' if c['database'] in ['ydb_managed','noop','pg_noop','external'] else 'not_verified'),'input':S['public_input'],'checked_at':v['checked_at'],'note':'All configured native workloads completed without terminal errors. Persisted native OTLP, scalar component metrics, logs, traces, artifact downloads and removal of owned YC resources verified. Database histogram/summary export remains pending.','log_observations':v['logs']['observation_counts'],'started_at':result['segments'][0]['started_at'],'finished_at':result['segments'][-1]['finished_at']}
 for field in ['native_otlp_metrics','component_metrics','component_logs','artifacts','pipeline_traces','cleanup']:row[field]='passed'
 row['load_progress']=proofs['load-progress']['status']
 if c['database'] in ['noop','pg_noop']:row['note']='Native execute_sql throughput and zero terminal errors verified; no logical transactions, TPS, population phase or database replication. Persisted runner/node metrics, logs, traces, artifact downloads and YC cleanup verified.'
 if c['database']=='external':row['note']='Read-only execute_sql against an owned PostgreSQL fixture; runner-only provisioning, native OTLP, runner telemetry, artifacts and cleanup verified. Fixture resources and canary survived runner cleanup; fixture then removed separately. See external-dsn-lifecycle-check.json. Other external protocols compile but are not live-verified by this run.'
 row['replication']='passed' if 'replication' in proofs else 'not_applicable'
 row['zones']=sorted({m['location'] for item in json.loads((P/'suite.json').read_text())['cells'] if item['id']==name for m in item['run_spec']['machines']} | set(next((item['run_spec'].get('managed_ydb',{}).get('zones',[]) for item in json.loads((P/'suite.json').read_text())['cells'] if item['id']==name), [])))
 if c['database']=='cockroach':
  versions=v['components']['database_versions'];nodes=1 if c['topology']=='single' else 3;assert len(versions)==nodes
  assert all(x['version'].removeprefix('v').startswith(c['version']+'.') for x in versions.values())
  row['database_actual_version']=','.join(sorted(set(x['version'] for x in versions.values())))
 d['catalog_cells']=[x for x in d['catalog_cells'] if key(x)!=key(row)]+[row]
 d['native_cells']=[x for x in d['native_cells'] if key(x)!=key(row)]
 for seg,n in zip(result['segments'],native['segments'],strict=True):
  assert seg['name']==n['segment'] and n['measurements']['terminal_errors_total']==0
  segment_spec=next(seg for item in json.loads((P/'suite.json').read_text())['cells'] if item['id']==name for seg in item['run_spec']['workload']['segments'] if seg['name']==n['segment'])
  preset='native-otlp-'+segment_spec['run']['duration']+'-'+str(segment_spec['run']['vus'])+'vus'
  d['native_cells'].append({**row,'preset':preset,'segment':n['segment'],'workload':segment_spec['workload']['script'],'native_tps':'passed' if 'tps' in n['measurements'] else 'not_applicable','iterations':n['measurements']['iterations_total'],'retry_attempts':n['measurements']['retry_attempts_total'],'started_at':seg['started_at'],'finished_at':seg['finished_at']})
 (P/(name+'.check-error.json')).unlink(missing_ok=True);accepted.append(name)
d['checked_at']=datetime.datetime.now(datetime.timezone.utc).isoformat();temporary=path.with_suffix('.json.tmp');temporary.write_text(json.dumps(d,indent=2)+'\n');os.replace(temporary,path);print('accepted',accepted)
subprocess.run(['python3',str(L.parents[1]/'scripts/live-progress.py')],check=True)
