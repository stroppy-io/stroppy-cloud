import json,datetime,subprocess,os,importlib.util,fcntl
from pathlib import Path
p=Path('/tmp/stroppy-catalog-0z9hx668');live=Path('/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live');out={'checked_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'runs':{},'planned_workloads':[],'stages':{},'verification':{}}
refresh_lock=(p/'progress-refresh.lock').open('w');fcntl.flock(refresh_lock,fcntl.LOCK_EX)
module_spec=importlib.util.spec_from_file_location('progress',live.parents[1]/'scripts/live-progress.py');progress=importlib.util.module_from_spec(module_spec);module_spec.loader.exec_module(progress)
campaigns=['crdb-matrix','ydb-metricsretry']
if (p/'ydb-channelretry/state.json').exists() and 'started_at' in json.loads((p/'ydb-channelretry/state.json').read_text()):campaigns.append('ydb-channelretry')
if (p/'ydb-parallel/state.json').exists() and 'started_at' in json.loads((p/'ydb-parallel/state.json').read_text()):campaigns.append('ydb-parallel')
if (p/'external-live/runner/state.json').exists():campaigns.append('external-live/runner')
if (p/'ydb-autonomous/state.json').exists() and 'started_at' in json.loads((p/'ydb-autonomous/state.json').read_text()):campaigns.append('ydb-autonomous')
for campaign in campaigns:
 d=p/campaign;s=json.loads((d/'state.json').read_text());suite=json.loads((d/'suite.json').read_text());runs=json.loads((d/'runs.json').read_text()) if (d/'runs.json').exists() else json.loads(subprocess.check_output([str(p/'bin/graphenectl'),'-n','t-stroppy-live','run','list','--jq','.'],text=True,timeout=15))['resources']
 for r in runs:out['runs'][r['ref'].removeprefix('run/')]=r['phase']
 for name,cell in s['cells'].items():
  spec=next(c['run_spec'] for c in suite['cells'] if c['id']==name)
  rid=cell['run_id']
  checked=d/(name+'.verified.json');error=d/(name+'.check-error.json')
  verified=json.loads(checked.read_text()) if checked.exists() else {}
  out['verification'][rid]='passed' if verified.get('status')=='passed' and verified.get('run_id')==rid else 'failed' if error.exists() else 'pending'
  if out['runs'].get(rid)=='Running':
   response=subprocess.run([str(p/'bin/graphenectl'),'-n','t-stroppy-live','events','run',rid,'--jq','.'],capture_output=True,text=True,timeout=15)
   if response.returncode==0:
    events=[json.loads(line) for line in response.stdout.splitlines() if line.strip()]
    stage=progress.current_run_stage(events)
    if stage:out['stages'][rid]={**stage,'checked_at':out['checked_at']}

  for seg in spec['workload']['segments']:
   script=seg['workload']['script'];logical=script in ['tpcb/tx','tpcb/procs','tpcc/tx','tpcc/procs','baseline']
   out['planned_workloads'].append({**{k:cell[k] for k in ['database','version','topology','run_id']},'workload':script,'preset':'native-otlp-'+seg['run']['duration']+'-'+str(seg['run']['vus'])+'vus','zone':','.join(sorted({m['location'] for m in spec['machines']})),'input':s['public_input'],'native_tps':'pending' if logical else 'not_applicable','database_metric_distributions':'pending','replication':'pending','full_workloads':'pending','baseline_matrix':'pending','segment_matrix':'pending','fault_campaign':'pending','managed_database_metrics':'not_applicable'})
for campaign in campaigns:
 subprocess.run(['python3',str(p/'accept_verified.py'),str(p/campaign)],check=True,capture_output=True)
target=live/'current-campaigns.json';tmp=target.with_suffix('.json.tmp');tmp.write_text(json.dumps(out,indent=2)+'\n');os.replace(tmp,target)
subprocess.run(['python3',str(live.parents[1]/'scripts/live-progress.py')],check=True)
