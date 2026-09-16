import json,datetime,subprocess
from pathlib import Path
P=Path(__file__).parent;LIVE=Path('/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live')
s=json.loads((P/'state.json').read_text());report=json.loads((LIVE/'catalog-functional-check.json').read_text());phases={r['ref']:r['phase'] for r in json.loads((P/'runs.json').read_text())};updated=[]
for name,cell in s['cells'].items():
 row=next(c for c in report['catalog_cells'] if (c['database'],c['version'],c['topology'])==(cell['database'],cell['version'],cell['topology']))
 row.update(cell)
 vfile=P/(name+'.verified.json')
 if not vfile.exists():
  phase=phases.get('run/'+cell['run_id']);row['status']='verifying' if phase=='Completed' else phase.lower() if phase else 'queued';continue
 v=json.loads(vfile.read_text());assert v['status']=='passed'
 progress=json.loads((LIVE/(name+'.load-progress.json')).read_text());assert progress['status']=='passed'
 result=json.loads((P/(name+'.result.json')).read_text());versions={r['version'] for r in v['components']['database_versions'].values()};assert len(versions)==1
 assert phases['run/'+cell['run_id']]=='Completed'
 for kind in ['vm','disk','network','subnet','sg']:
  current=json.loads((P/(kind+'-current.json')).read_text());assert not any(cell['uuid'][:8] in r['name'] for r in current)
 report['native_cells']=[c for c in report['native_cells'] if (c['database'],c['version'],c['topology'])!=(cell['database'],cell['version'],cell['topology'])]
 for seg in v['native']['segments']:
  n=seg['segment'];resultseg=next(x for x in result['segments'] if x['name']==n)
  data={**cell,'status':'passed','compile':'passed','database_actual_version':next(iter(versions)),'segment':n,'workload':{'simple':'simple','tpcb-tx':'tpcb/tx','tpcb-procs':'tpcb/procs','tpcc-tx':'tpcc/tx','tpcc-procs':'tpcc/procs'}[n],'native_tps':'not_applicable' if n=='simple' else 'passed','native_otlp_metrics':'passed','component_metrics':'passed','component_logs':'passed','artifacts':'passed','replication':'passed' if cell['topology']=='primary-replica' else 'not_applicable','pipeline_traces':'passed','cleanup':'passed','iterations':seg['measurements']['iterations_total'],'retry_attempts':seg['measurements'].get('retry_attempts_total',0),'input':'suite-orioledb.json','checked_at':v['checked_at'],'started_at':resultseg['started_at'],'finished_at':resultseg['finished_at'],'log_observations':v['logs']['observation_counts'],'local_cluster':'passed','note':'Native workload, durable telemetry and downloaded artifacts verified; actual Oriole table method and UTF8 checked by SQL on every member. Transaction conflict and expected TPC-C rollback log observations retained. Cleanup here proves resource removal, not graceful database shutdown.'}
  if n=='simple':row.update(data)
  else:report['native_cells'].append(data)
 updated.append(name)
report['status']='running';report['checked_at']=datetime.datetime.now(datetime.timezone.utc).isoformat();(LIVE/'catalog-functional-check.json').write_text(json.dumps(report,indent=2)+'\n');subprocess.run(['python3',str(LIVE.parents[1]/'scripts/live-progress.py')],check=True);print('Verified:',updated)
