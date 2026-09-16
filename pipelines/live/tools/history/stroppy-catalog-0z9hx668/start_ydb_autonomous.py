from pathlib import Path
import subprocess,json,datetime,time
p=Path('/tmp/stroppy-catalog-0z9hx668');d=p/'ydb-autonomous';old=p/'ydb-parallel';cli=str(p/'bin/graphenectl');s=json.loads((d/'state.json').read_text());v=json.loads((d/'suite.json').read_text());assert 'started_at' not in s;assert len(v['cells'])==v['concurrency']==3
while True:
 counts={k:len(json.loads((old/(k+'-current.json')).read_text())) for k in ['vm','disk','network','subnet','sg','ydb']}
 phase=subprocess.check_output([cli,'-n','t-stroppy-live','get','run/matrix-ydbparallel-9176f5d9','--jq','.status'],text=True,timeout=30).strip()
 if phase in ['Canceled','Failed']:raise RuntimeError('Successful business result was not preserved: '+phase)
 if not any(counts.values()) and phase=='Completed':break
 print(time.strftime('%H:%M:%S'),'waiting for completed 25.4 cleanup',counts,flush=True);time.sleep(20)
# Reserve all three stands against fresh authoritative quota usage.
ms=[m for c in v['cells'] for m in c['run_spec']['machines']]
required={'compute.instances.count':len(ms),'compute.instanceCores.count':sum(m['cpu'] for m in ms),'compute.instanceMemory.size':sum(m['memory_gb'] for m in ms)*1024**3,'compute.disks.count':len(ms)+sum(len(m.get('disks',[])) for m in ms),'compute.ssdDisks.size':(40*len(ms)+sum(x['gb'] for m in ms for x in m.get('disks',[])))*1024**3,'vpc.networks.count':3,'vpc.subnets.count':9,'vpc.securityGroups.count':3,'vpc.externalAddresses.count':len(ms)}
proof={}
for service in ['compute','vpc']:
 q=json.loads(subprocess.check_output(['yc','quota-manager','quota-limit','list','--resource-id','b1gt6m4l9gfhaobcgb81','--resource-type','resource-manager.cloud','--service',service,'--format','json'],text=True,timeout=30))
 for x in q['quota_limits']:
  key=x['quota_id']
  if key in required:
   proof[key]={'usage':x['usage'],'reserved':required[key],'projected':x['usage']+required[key],'limit':x['limit']};assert proof[key]['projected']<=x['limit'],proof[key]
assert set(proof)==set(required)
image=json.loads(subprocess.check_output([cli,'-n','t-stroppy-live','get','pipeline/stroppy-run','--jq','{image:.resource.state.image}'],text=True,timeout=30))['image'];assert image.endswith(':f53623d265c372b2')
s['started_at']=datetime.datetime.now(datetime.timezone.utc).isoformat();s['pipeline_version']=image.rsplit(':',1)[-1];(d/'state.json').write_text(json.dumps(s,indent=2)+'\n')
public=Path('/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live');(public/'ydb-autonomous-quota-check.json').write_text(json.dumps({'status':'passed','checked_at':s['started_at'],'concurrent_stands':3,'reservations':proof},indent=2)+'\n')
r=subprocess.run([cli,'-n','t-stroppy-live','run','start','stroppy-suite','--run-id',s['run_id'],'--params-file',str(d/'suite.json'),'--jq','.'],capture_output=True,text=True,timeout=60);(d/'start.stdout').write_text(r.stdout);(d/'start.stderr').write_text(r.stderr);r.check_returncode();print('started',s['run_id'],'concurrency 3',flush=True)
with (d/'monitor.log').open('a') as out:
 monitor=subprocess.Popen(['python3',str(d/'monitor.py')],stdout=out,stderr=subprocess.STDOUT)
 try:
  watcher=subprocess.run(['python3',str(d/'watch_cells.py')]);watcher.check_returncode();monitor.wait()
 finally:
  if monitor.poll() is None:monitor.terminate();monitor.wait(timeout=5)
subprocess.run(['python3',str(p/'refresh_progress.py')],check=True)
