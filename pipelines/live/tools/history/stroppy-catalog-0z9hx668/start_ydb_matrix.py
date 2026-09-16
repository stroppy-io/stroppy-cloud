from pathlib import Path
import subprocess,json,datetime
p=Path('/tmp/stroppy-catalog-0z9hx668');old=p/'managed-queryready';dest=p/'ydb-matrix';cli='/tmp/stroppy-clusters-r1gx3otm/bin/graphenectl'
for k in ['vm','disk','network','subnet','sg','ydb']:assert not json.loads((old/(k+'-current.json')).read_text()),k
s=json.loads((dest/'state.json').read_text());assert 'started_at' not in s
v=json.loads((dest/'suite.json').read_text());assert v['concurrency']==1 and len(v['cells'])==8
for c in v['cells']:
 rs=c['run_spec'];assert rs['observability']['otlp_headers'];assert all(m['location'] in ['ru-central1-a','ru-central1-b','ru-central1-d'] for m in rs['machines']);assert '@sha256:' in rs['workload']['stroppy_image']
r=subprocess.run([cli,'-n','t-stroppy-live','get','pipeline/stroppy-run','--jq','{image:.resource.state.image}'],capture_output=True,text=True,timeout=30);r.check_returncode();image=json.loads(r.stdout)['image'];assert image.endswith(':932541bfa871c3f2')
s['started_at']=datetime.datetime.now(datetime.timezone.utc).isoformat();s['pipeline_version']=image.rsplit(':',1)[-1];(dest/'state.json').write_text(json.dumps(s,indent=2)+'\n')
r=subprocess.run([cli,'-n','t-stroppy-live','run','start','stroppy-suite','--run-id',s['run_id'],'--params-file',str(dest/'suite.json'),'--jq','.'],capture_output=True,text=True,timeout=60);(dest/'start.stdout').write_text(r.stdout);(dest/'start.stderr').write_text(r.stderr);r.check_returncode();print('started',s['run_id'],'worker',s['pipeline_version'],flush=True)
with (dest/'monitor.log').open('a') as out:
 monitor=subprocess.Popen(['python3',str(dest/'monitor.py')],stdout=out,stderr=subprocess.STDOUT)
 try:
  watcher=subprocess.run(['python3',str(dest/'watch_cells.py')]);watcher.check_returncode();monitor.wait()
 finally:
  if monitor.poll() is None:monitor.terminate();monitor.wait(timeout=5)
