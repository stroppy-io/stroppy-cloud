from pathlib import Path
import subprocess,json,datetime,time
p=Path('/tmp/stroppy-catalog-0z9hx668');old=p/'ydb-metricsretry';dest=p/'ydb-channelretry';cli=str(p/'bin/graphenectl')
while True:
 counts={k:len(json.loads((old/(k+'-current.json')).read_text())) for k in ['vm','disk','network','subnet','sg','ydb']}
 if not any(counts.values()):break
 print(time.strftime('%H:%M:%S'),'waiting for failed mirror cleanup',counts,flush=True);time.sleep(20)
# Verify the removed stand directly against YC as well as the observer cache.
for kind,args in {'vm':['compute','instance'],'disk':['compute','disk'],'network':['vpc','network'],'subnet':['vpc','subnet'],'sg':['vpc','security-group'],'ydb':['ydb','database']}.items():
 rows=json.loads(subprocess.check_output(['yc',*args,'list','--folder-id','b1ghttqg66t14ldkvfcq','--format','json'],text=True,timeout=30))
 assert not any('536620e1' in x.get('name','') for x in rows),kind
assert json.loads((p/'ydb-channel-local/report.json').read_text())['status']=='passed'
s=json.loads((dest/'state.json').read_text());assert 'started_at' not in s
v=json.loads((dest/'suite.json').read_text());assert v['concurrency']==1 and len(v['cells'])==4
for c in v['cells']:
 rs=c['run_spec'];assert rs['observability']['otlp_headers'];assert all(m['location'] in ['ru-central1-a','ru-central1-b','ru-central1-d'] for m in rs['machines']);assert '@sha256:' in rs['workload']['stroppy_image']
r=subprocess.run([cli,'-n','t-stroppy-live','get','pipeline/stroppy-run','--jq','{image:.resource.state.image}'],capture_output=True,text=True,timeout=30);r.check_returncode();image=json.loads(r.stdout)['image'];assert image.endswith(':f53623d265c372b2')
q=json.loads(subprocess.check_output(['yc','quota-manager','quota-limit','list','--resource-id','b1gt6m4l9gfhaobcgb81','--resource-type','resource-manager.cloud','--service','vpc','--format','json'],text=True,timeout=30));net=next(x for x in q['quota_limits'] if x['quota_id']=='vpc.networks.count');assert net['usage']<net['limit']
s['started_at']=datetime.datetime.now(datetime.timezone.utc).isoformat();s['pipeline_version']=image.rsplit(':',1)[-1];(dest/'state.json').write_text(json.dumps(s,indent=2)+'\n')
r=subprocess.run([cli,'-n','t-stroppy-live','run','start','stroppy-suite','--run-id',s['run_id'],'--params-file',str(dest/'suite.json'),'--jq','.'],capture_output=True,text=True,timeout=60);(dest/'start.stdout').write_text(r.stdout);(dest/'start.stderr').write_text(r.stderr);r.check_returncode();print('started',s['run_id'],'worker',s['pipeline_version'],flush=True)
with (dest/'monitor.log').open('a') as out:
 monitor=subprocess.Popen(['python3',str(dest/'monitor.py')],stdout=out,stderr=subprocess.STDOUT)
 try:
  watcher=subprocess.run(['python3',str(dest/'watch_cells.py')]);watcher.check_returncode();monitor.wait()
 finally:
  if monitor.poll() is None:monitor.terminate();monitor.wait(timeout=5)
