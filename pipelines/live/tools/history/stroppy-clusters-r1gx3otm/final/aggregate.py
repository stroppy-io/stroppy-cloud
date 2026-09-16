import datetime,fcntl,json,os,subprocess,sys
from pathlib import Path
P=Path(__file__).parent; LIVE=Path('/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live')
with (P/'aggregate.lock').open('w') as lock:
 fcntl.flock(lock,fcntl.LOCK_EX)
 state=json.loads((P/'state.json').read_text())
 phases={r['ref']:r['phase'] for r in json.loads((P/'runs.json').read_text())}
 reports={key:json.loads((LIVE/file).read_text()) for key,file in [('galera','mysql-clusters-check.json'),('single','mysql-family-check.json')]}
 for name,cell in state['cells'].items():
  report=reports[cell['topology']]
  row=next(c for c in report['catalog_cells'] if c['database']==cell['database'] and c['version']==cell['version'] and c['topology']==cell['topology'])
  path=P/(name+'.verified.json');progress=LIVE/(name+'.load-progress.json')
  if not path.exists() or not progress.exists():
   phase=phases.get('run/'+cell['run_id'])
   row['status']='queued' if phase is None else 'verifying' if phase=='Completed' else phase.lower()
   continue
  v=json.loads(path.read_text());pr=json.loads(progress.read_text());assert v['status']==pr['status']=='passed' and v['run_id']==pr['run_id']==cell['run_id']
  assert v['logs']['error_scan_window'] and not v['logs']['workload_error_candidates']
  assert 'raw export' in v['components']['workload_observation']['source']
  collectors=v['components']['workload_observation']['mysql_exporter_collectors'];assert collectors and all(c['min']==1 for c in collectors)
  if cell['topology']=='galera':
   for suffix in ['replication','replication-metrics']:
    evidence=json.loads((LIVE/(name+'.'+suffix+'.json')).read_text());assert evidence['status']=='passed' and evidence['run_id']==cell['run_id']
  versions={r['version'] for r in v['components']['database_versions'].values()};assert len(versions)==1
  result=json.loads((P/(name+'.result.json')).read_text())
  report['native_cells']=[c for c in report['native_cells'] if (c['database'],c['version'],c['topology'])!=(cell['database'],cell['version'],cell['topology'])]
  for seg in v['native']['segments']:
   segment=seg['segment'];assert seg['workload_without_terminal_errors']
   common=dict(status='passed',database=cell['database'],version=cell['version'],topology=cell['topology'],database_actual_version=next(iter(versions)),run_id=cell['run_id'],workload={'simple':'simple','tpcb-tx':'tpcb/tx','tpcb-procs':'tpcb/procs','tpcc-tx':'tpcc/tx','tpcc-procs':'tpcc/procs'}[segment],segment=segment,iterations=seg['measurements']['iterations_total'],native_tps='not_applicable' if segment=='simple' else 'passed',native_otlp_metrics='passed',component_metrics='passed',component_logs='passed',artifacts='passed',replication='passed' if cell['topology']=='galera' else 'not_applicable',pipeline_traces='passed',cleanup='passed',input='suite-mariadb-monitor-fixed.json',checked_at=v['checked_at'],evidence_prefix=name,started_at=result['segments'][0]['started_at'],finished_at=result['segments'][-1]['finished_at'],log_observations=v['logs'].get('observation_counts',{}),retry_attempts=seg['measurements'].get('retry_attempts_total'),note='all native workloads and exporter collectors passed; log observations retained separately; MariaDB SLAVE MONITOR included at bootstrap')
   if v['components'].get('cluster_coverage_note'):common['note']+='; '+v['components']['cluster_coverage_note']
   if segment=='simple':row.update(common,compile='passed')
   else:
    common['preset']='native-otlp-20s-2vus-retry50' if segment.startswith('tpcc') else 'native-otlp-20s-2vus';report['native_cells'].append(common)
 for key,file in [('galera','mysql-clusters-check.json'),('single','mysql-family-check.json')]:
  report=reports[key];passed=sum(c['status']=='passed' for c in report['catalog_cells']);report.update(status='passed' if passed==len(report['catalog_cells']) else 'running',checked_at=datetime.datetime.now(datetime.timezone.utc).isoformat())
  report['validated' if key=='galera' else 'catalog_validated']=passed
  target=LIVE/file;tmp=target.with_suffix('.tmp');tmp.write_text(json.dumps(report,indent=2)+'\n');os.replace(tmp,target)
 subprocess.run(['python3',str(LIVE.parents[1]/'scripts/live-progress.py')],check=True)
