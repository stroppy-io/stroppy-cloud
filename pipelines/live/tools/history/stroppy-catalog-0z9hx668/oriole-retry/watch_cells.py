import concurrent.futures, datetime, json, subprocess, time, os, select
from pathlib import Path
P=Path(__file__).parent
LIVE=Path('/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live')
S=json.loads((P/'state.json').read_text()); SUITE=json.loads((P/'suite.json').read_text())
CLI=str(P/'bin/graphenectl'); os.environ['PATH']=str(P/'bin')+os.pathsep+os.environ['PATH']
def dump(p,x):p.write_text(json.dumps(x,indent=2)+'\n')
def run(args):return subprocess.run(args,capture_output=True,text=True,timeout=180)
def cell(item):
 name,c=item; spec=next(x['run_spec'] for x in SUITE['cells'] if x['id']==name)
 nodes=[x['machine'] for x in spec['containers'] if x['name'].endswith('-orioledb')]
 checked={};deadline=time.monotonic()+5400
 try:
  while time.monotonic()<deadline:
   r=run([CLI,'-n','t-stroppy-live','run','list','--jq','.'])
   if r.returncode:time.sleep(15);continue
   rows=json.loads(r.stdout)['resources']
   row=next((r for r in rows if r['ref']=='run/'+c['run_id']),None)
   parent=next((r for r in rows if r['ref']=='run/'+S['run_id']),None)
   if not row and parent and parent['phase'] in ['Completed','Failed','Canceled']:
    print(name,'not started before suite finished',flush=True);return
   if row and row['phase'] in ['Completed','Failed','Canceled']:break
   if row:
    for node in nodes:
     if node in checked:continue
     target=P/(name+'-'+node+'.sql.json')
     r=run(['python3',str(P/'probe_remote.py'),c['uuid'][:8]+'-'+node,str(target)]+(['--seed'] if node=='db-1' else []))
     target.with_suffix('.attempt.log').write_text(r.stdout+r.stderr)
     if r.returncode==0:checked[node]=json.loads(target.read_text())
     else:break
    if nodes and len(checked)==len(nodes) and not (LIVE/(name+'.replication.json')).exists():
     dump(LIVE/(name+'.replication.json'),dict(status='passed',run_id=c['run_id'],checked_at=datetime.datetime.now(datetime.timezone.utc).isoformat(),source='SQL through ordinary agents on every OrioleDB YC member; primary seeds 100 owned test rows',checks=checked))
     print(name,'all OrioleDB member SQL probes passed',flush=True)
   time.sleep(15)
  else:raise RuntimeError('run timeout')
  r=run([CLI,'-n','t-stroppy-live','run','result',c['run_id'],'--jq','.'])
  (P/(name+'.result.stdout')).write_text(r.stdout);(P/(name+'.result.stderr')).write_text(r.stderr)
  r.check_returncode();result=json.loads(r.stdout);dump(P/(name+'.result.json'),result);dump(LIVE/(name+'.result.json'),result)
  print(name,'result',[(s['name'],s['status']) for s in result.get('segments',[])],flush=True)
  if len(checked)!=len(nodes):raise RuntimeError('live SQL replication checks incomplete')
  for attempt in range(3):
   r=run(['python3',str(P/'verify_one.py'),str(P),name,name])
   (P/(name+'.verification.log')).write_text(r.stdout+r.stderr)
   if r.returncode==0:break
   time.sleep(20)
  r.check_returncode()
  r=run(['python3','/tmp/stroppy-mysql-vnafdcv2/verify_progress.py',c['run_id'],str(LIVE/(name+'.load-progress.json'))])
  (P/(name+'.progress.log')).write_text(r.stdout+r.stderr);r.check_returncode()
  print(name,'all persisted checks passed',flush=True)
 except Exception as exc:
  dump(P/(name+'.check-error.json'),{'error':str(exc)});print(name,'check failed',str(exc)[-300:],flush=True)
pf=[]
try:
 for svc,ports in [('victoria-logs-victoria-logs-single-server','19428:9428'),('victoria-traces-vt-single-server','18043:10428')]:
  proc=subprocess.Popen(['kubectl','--kubeconfig','/tmp/stroppy-mysql-vnafdcv2/kubeconfig','--context','stroppy-live-test','-n','observability','port-forward','svc/'+svc,ports],stdout=subprocess.PIPE,stderr=subprocess.STDOUT,text=True)
  pf.append(proc);deadline=time.monotonic()+30;ready=False;lines=[]
  while time.monotonic()<deadline:
   if proc.poll() is not None:raise RuntimeError('port-forward ended: '+''.join(lines))
   if not select.select([proc.stdout],[],[],.5)[0]:continue
   line=proc.stdout.readline();lines.append(line)
   if 'Forwarding from 127.0.0.1:' in line:ready=True;break
  if not ready:raise RuntimeError('port-forward timeout: '+''.join(lines))
 with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:list(pool.map(cell,S['cells'].items()))
finally:
 for proc in pf:
  proc.terminate();proc.wait(timeout=10)
