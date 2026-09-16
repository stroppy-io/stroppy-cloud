import datetime, fcntl, json, os, subprocess, sys
from pathlib import Path
P=Path(sys.argv[1]); LIVE=Path('/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live')
with (P/'aggregate.lock').open('w') as lock:
 fcntl.flock(lock,fcntl.LOCK_EX)
 state=json.loads((P/'state.json').read_text())
 report=json.loads((LIVE/'mysql-clusters-check.json').read_text())
 report['native_cells']=[];passed=0
 phases={r['ref']:r['phase'] for r in json.loads((P/'runs.json').read_text())}
 for row,(name,cell) in zip(report['catalog_cells'],state['cells'].items()):
  path=P/(name+'.verified.json')
  if not path.exists():
   phase=phases.get('run/'+cell['run_id'])
   row['status']='queued' if phase is None else 'verifying' if phase=='Completed' else phase.lower()
   continue
  verified=json.loads(path.read_text());assert verified['status']=='passed'
  progress_path=LIVE/(name+'.load-progress.json')
  if not progress_path.exists():continue
  progress=json.loads(progress_path.read_text())
  assert progress['status']=='passed' and progress['run_id']==cell['run_id']
  replication=json.loads((LIVE/(name+'.replication.json')).read_text())
  cluster_metrics=json.loads((LIVE/(name+'.replication-metrics.json')).read_text())
  assert replication['status']==cluster_metrics['status']=='passed'
  assert replication['run_id']==cluster_metrics['run_id']==cell['run_id']
  versions={v['version'] for v in verified['components']['database_versions'].values()}
  assert len(versions)==1
  result=json.loads((P/(name+'.result.json')).read_text())
  for segment in verified['native']['segments']:
   name_=segment['segment'];assert segment['workload_without_terminal_errors']
   common=dict(status='passed',database=row['database'],version=row['version'],topology=row['topology'],database_actual_version=next(iter(versions)),run_id=cell['run_id'],workload={'simple':'simple','tpcb-tx':'tpcb/tx','tpcb-procs':'tpcb/procs','tpcc-tx':'tpcc/tx','tpcc-procs':'tpcc/procs'}[name_],segment=name_,iterations=segment['measurements']['iterations_total'],native_tps='not_applicable' if name_=='simple' else 'passed',native_otlp_metrics='passed',component_metrics='passed',component_logs='passed',artifacts='passed',replication='passed',pipeline_traces='passed',cleanup='passed',input='suite-mysql-clusters.json',checked_at=verified['checked_at'],evidence_prefix=name,started_at=result['segments'][0]['started_at'],finished_at=result['segments'][-1]['finished_at'])
   if name_=='simple':row.update(common,compile='passed',local_cluster='passed',note='causal reads; five native workload segments; all cluster/proxy/component/artifact checks passed')
   else:
    common['preset']='native-otlp-20s-2vus-retry50' if name_.startswith('tpcc') else 'native-otlp-20s-2vus'
    report['native_cells'].append(common)
  passed+=1
 report.update(status='passed' if passed==5 else 'running',validated=passed,checked_at=datetime.datetime.now(datetime.timezone.utc).isoformat())
 target=LIVE/'mysql-clusters-check.json';tmp=target.with_suffix('.tmp');tmp.write_text(json.dumps(report,indent=2)+'\n');os.replace(tmp,target)
 subprocess.run(['python3',str(LIVE.parents[1]/'scripts/live-progress.py')],check=True)
