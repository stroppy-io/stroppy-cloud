import sys,json,subprocess,ipaddress,time,hashlib,datetime,re
from pathlib import Path
P=Path(__file__).parent;source=Path(sys.argv[1]);spec=json.loads(source.read_text());name=source.stem;folder=P/('local-'+name+'-'+str(time.time_ns()));folder.mkdir(mode=0o700,exist_ok=True)
net='stroppy-ydb-'+spec['run_id'][:8];containers=[]
def docker(*args,check=True):
 r=subprocess.run(['docker',*args],capture_output=True,text=True,timeout=900 if "--duration" in args else 120)
 if check and r.returncode:raise RuntimeError(str(args[:2])+': '+r.stderr[-1500:])
 return r
try:
 docker('network','create',net);subnet=json.loads(docker('network','inspect',net).stdout)[0]['IPAM']['Config'][0]['Subnet'];base=ipaddress.ip_network(subnet).network_address
 ips={m['name']:str(base+i+10) for i,m in enumerate(spec['machines'])}
 def render(s):
  for m,ip in ips.items():s=s.replace('${ip:'+m+'}',ip)
  for machine in spec['machines']:s=s.replace('${ip:role:'+machine['role']+'}',ips[machine['name']])
  return s
 cts=[c for c in spec['containers'] if c['name'].endswith(('-ydb','-init'))];cts.sort(key=lambda c:2 if c['role']=='db-compute' else 1 if c['name'].endswith('-init') else 0)
 for c in cts:
  physical=net+'-'+c['name'];args=['run','-d','--name',physical,'--memory','4g']
  if c['name'].endswith('-init'):args+=['--network','container:'+net+'-'+c['machine']+'-ydb']
  else:args+=['--network',net,'--ip',ips[c['machine']]]
  if c.get('entrypoint'):args+=['--entrypoint',c['entrypoint'][0]]
  for k,v in c.get('env',{}).items():args+=['-e',k+'='+render(v)]
  for i,f in enumerate(c.get('files',[])):
   path=folder/(c['name']+'-'+str(i));path.write_text(render(f['content']));path.chmod(int(f.get('mode','0644'),8));args+=['-v',str(path)+':'+f['path']+':ro']
  for m in c.get('mounts',[]):
   data=folder/(c['machine']+'-data');data.mkdir(exist_ok=True)
   disk=data/'pdisk-1.data'
   if not disk.exists():
    with disk.open('wb') as f:f.truncate(40*1024**3)
   args+=['-v',str(data)+':'+m['target']]
  args+=[c['image'],*[render(x) for x in c.get('cmd',[])]];docker(*args);containers.append(physical)
  if c['name'].endswith('-init'):
   for _ in range(90):
    if docker('exec',physical,'test','-f','/tmp/stroppy-init-done',check=False).returncode==0:break
    time.sleep(1)
   else:raise RuntimeError('YDB bootstrap failed: '+docker('logs',physical,check=False).stderr[-2500:])
  time.sleep(.5)
  state=json.loads(docker('inspect',physical).stdout)[0]['State']
  if not state['Running']:raise RuntimeError(physical+' exited '+str(state['ExitCode'])+' '+docker('logs',physical,check=False).stderr[-2500:])
 proof={}
 native_proof={}
 viewer_proof={}
 binary=Path('/home/yaroher/devel/github/stroppy-io/stroppy/build/stroppy')
 binary_sha=hashlib.sha256(binary.read_bytes()).hexdigest()
 for attempt in range(60):
  for physical in containers:
   state=json.loads(docker('inspect',physical).stdout)[0]['State']
   if not state['Running']:raise RuntimeError(physical+' stopped: '+docker('logs',physical,check=False).stderr[-5000:])
  compute=next(c for c in cts if c['role']=='db-compute')
  physical=net+'-'+compute['name']
  r=docker('exec',physical,*[render(x) for x in compute['healthcheck']['cmd'][1:]],check=False)
  if r.returncode==0:proof[physical]=r.stdout;break
  time.sleep(1)
 else:raise RuntimeError('YDB readiness timed out: '+r.stderr[-2000:])
 import urllib.request
 opener=urllib.request.build_opener(urllib.request.ProxyHandler({}))
 for c in cts:
  if not c['name'].endswith('-ydb'):continue
  for endpoint in ['/counters/prometheus', '/counters/counters=ydb/prometheus']:
   try:
    with opener.open('http://'+ips[c['machine']]+':8765'+endpoint,timeout=10) as response:body=response.read().decode()
    (folder/(c['name']+'-'+endpoint.replace('/','_')+'.metrics')).write_text(body)
    print(c['name'],endpoint,len(body),'bytes',flush=True)
   except Exception as exc:print('metrics probe',str(exc),flush=True)
 if '--viewer' in sys.argv:
  for c in cts:
   if not c['name'].endswith('-ydb'):continue
   for endpoint in ['/viewer/json/nodes', '/viewer/json/cluster', '/viewer/json/storage', '/viewer/json/tenantinfo?database=/Root/stroppy']:
    try:
     with opener.open('http://'+ips[c['machine']]+':8765'+endpoint,timeout=30) as response:body=response.read().decode()
     (folder/(c['name']+'-'+endpoint.split('?')[0].replace('/','_')+'.json')).write_text(body)
     value=json.loads(body);print(c['name'],endpoint,'keys',list(value)[:15],flush=True)
     if endpoint == '/viewer/json/tenantinfo?database=/Root/stroppy':
      probe=subprocess.run(['python3','/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live/ydb_probe.py','--endpoint','http://'+ips[c['machine']]+':8765','--version',name.split('-')[2],'--storage-nodes','1','--compute-nodes','1','--zones','ru-central1-a'],capture_output=True,text=True)
      (folder/(c['name']+'-viewer-probe.json')).write_text(probe.stdout)
      (folder/(c['name']+'-viewer-probe.err')).write_text(probe.stderr)
      probe.check_returncode();viewer_proof[c['name']]=json.loads(probe.stdout);print(c['name'],'viewer checks passed',flush=True)

    except Exception as exc:print('viewer probe',endpoint,str(exc),flush=True)
 if '--viewer' in sys.argv:assert len(viewer_proof)==sum(c['name'].endswith('-ydb') for c in cts),viewer_proof
 if '--native' in sys.argv:
  for workload in ['simple','tpcb/tx','tpcc/tx']:
   native=net+'-native';containers.append(native)
   args=['run','--rm','--name',native,'--network',net,'-v','/home/yaroher/devel/github/stroppy-io/stroppy/build/stroppy:/usr/local/bin/stroppy:ro',spec['workload']['stroppy_image'],'run',workload,'-d','ydb','-D','url='+render(spec['workload']['url']),'--executor','constant-vus','--duration','5s','--vus','2','--log-level','info','--log-mode','production']
   if workload!='simple':args+=['--scale-factor','1','--load-workers','2','--retry-attempts','50']
   r=docker(*args,check=False);(folder/('native-'+workload.replace('/','-')+'.log')).write_text(r.stdout+r.stderr);print('native',workload,r.returncode,(r.stdout+r.stderr)[-1200:],flush=True)
   if r.returncode or any(float(x)>0 for x in re.findall(r'terminal_errors_total\s+(\d+(?:\.\d+)?)',r.stdout+r.stderr)):raise RuntimeError('native '+workload+' failed')
   measurements={}
   for metric in ['iterations_total','terminal_errors_total']+([] if workload=='simple' else ['tps']):
    values=re.findall(r'^\s*'+metric+r'\s+([0-9.eE+-]+)',r.stdout+r.stderr,re.M)
    assert len(values)==1,(workload,metric,values)
    measurements[metric]=float(values[0])
   assert measurements['iterations_total']>0 and measurements['terminal_errors_total']==0
   assert workload=='simple' or measurements['tps']>0
   native_proof[workload]=measurements
 report={'status':'passed','checked_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'input_sha256':hashlib.sha256(source.read_bytes()).hexdigest(),'checks':proof,'native':native_proof,'viewer':viewer_proof,'binary_sha256':binary_sha};(folder/'report.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))
finally:
 for physical in reversed(containers):
  r=docker('logs',physical,check=False);(folder/(physical+'.log')).write_text(r.stdout+r.stderr);docker('rm','-f','-v',physical,check=False)
 docker('network','rm',net,check=False)
