import csv,io,re,sys,json,subprocess,ipaddress,time,hashlib,datetime
from pathlib import Path
P=Path(__file__).parent;source=Path(sys.argv[1]);spec=json.loads(source.read_text());name=source.stem;folder=P/('local-'+name+'-'+str(time.time_ns()));folder.mkdir(mode=0o700,exist_ok=True)
net='stroppy-crdb-'+spec['run_id'][:8];containers=[]
def docker(*args,check=True):
 r=subprocess.run(['docker',*args],capture_output=True,text=True,timeout=600 if "--duration" in args else 120)
 if check and r.returncode:raise RuntimeError(str(args[:2])+': '+r.stderr[-1500:])
 return r
try:
 docker('network','create',net);subnet=json.loads(docker('network','inspect',net).stdout)[0]['IPAM']['Config'][0]['Subnet'];base=ipaddress.ip_network(subnet).network_address
 ips={m['name']:str(base+i+10) for i,m in enumerate(spec['machines'])}
 def render(s):
  for m,ip in ips.items():s=s.replace('${ip:'+m+'}',ip)
  return s
 cts=[c for c in spec['containers'] if c['name'].endswith(('-cockroach','-init'))];cts.sort(key=lambda c:bool(c.get('depends_on')))
 for c in cts:
  physical=net+'-'+c['name'];args=['run','-d','--name',physical,'--memory','4g']
  if c['name'].endswith('-init'):args+=['--network','container:'+net+'-'+c['machine']+'-cockroach']
  else:args+=['--network',net,'--ip',ips[c['machine']]]
  if c.get('entrypoint'):args+=['--entrypoint',c['entrypoint'][0]]
  for k,v in c.get('env',{}).items():args+=['-e',k+'='+render(v)]
  for i,f in enumerate(c.get('files',[])):
   path=folder/(c['name']+'-'+str(i));path.write_text(render(f['content']));path.chmod(int(f.get('mode','0644'),8));args+=['-v',str(path)+':'+f['path']+':ro']
  for m in c.get('mounts',[]):args+=['--tmpfs',m['target']+':rw,size=2g,mode=1777']
  args+=[c['image'],*[render(x) for x in c.get('cmd',[])]];docker(*args);containers.append(physical)
  time.sleep(.5)
  state=json.loads(docker('inspect',physical).stdout)[0]['State']
  if not state['Running']:raise RuntimeError(physical+' exited '+str(state['ExitCode'])+' '+docker('logs',physical,check=False).stderr[-2500:])
 proof={}
 for c in cts:
  physical=net+'-'+c['name']
  for attempt in range(90):
   if c['name'].endswith('-init'):
    r=docker('exec',physical,'test','-f','/tmp/stroppy-init-done',check=False)
   else:
    r=docker('exec',physical,'/cockroach/cockroach','sql','--insecure','--host=127.0.0.1:26257','--format=json','-e','SELECT version();',check=False)
   if r.returncode==0:break
   state=json.loads(docker('inspect',physical).stdout)[0]['State']
   if not state['Running']:raise RuntimeError(physical+' stopped: '+docker('logs',physical,check=False).stderr[-2500:])
   time.sleep(1)
  else:raise RuntimeError(physical+' readiness failed: '+r.stderr[-1800:]+' '+docker('logs',physical,check=False).stderr[-1800:])
  proof[physical]=r.stdout
  if c['name'].endswith('-cockroach'):
   for membership_attempt in range(60):
    nodes=docker('exec',physical,'/cockroach/cockroach','node','status','--insecure','--host=127.0.0.1:26257','--format=csv').stdout
    members=list(csv.DictReader(io.StringIO(nodes)))
    if len(members)==sum(x['name'].endswith('-cockroach') for x in cts) and all(x.get('is_live')=='true' and x.get('is_available')=='true' for x in members):break
    time.sleep(1)
   else:raise RuntimeError('cluster members did not become available: '+nodes)
   proof[physical+'-nodes']=nodes
  print(physical,'ready',r.stdout[-500:],flush=True)
  if '--probe' in sys.argv and c['name'].endswith('-cockroach'):
   r=subprocess.run(['python3','/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live/cockroach_probe.py','--host',ips[c['machine']],'--members',str(sum(x['name'].endswith('-cockroach') for x in cts)),'--version',name.split('-')[-2]],capture_output=True,text=True);print('pgwire',r.stdout,r.stderr,flush=True);r.check_returncode();proof[physical+'-pgwire']=json.loads(r.stdout)
   import urllib.request
   opener=urllib.request.build_opener(urllib.request.ProxyHandler({}))
   with opener.open('http://'+ips[c['machine']]+':8080/_status/vars',timeout=20) as resp:metrics=resp.read()
   (folder/(physical+'.metrics')).write_bytes(metrics)
   print('prometheus',physical,len(metrics),'bytes',flush=True)

 if '--native' in sys.argv:
  first=next(c for c in cts if c['name'].endswith('-cockroach'))
  url='postgresql://root@'+ips[first['machine']]+':26257/defaultdb?sslmode=disable'
  for workload in (['tpcc/procs'] if '--procs-only' in sys.argv else ['simple','tpcb/tx','tpcb/procs','tpcc/tx','tpcc/procs']):
   native=net+'-native';containers.append(native)
   args=['run','--rm','--name',native,'--network',net,'-v','/tmp/stroppy-catalog-0z9hx668/stroppy-native/dev-native-296997f45fd0/stroppy'+':/usr/local/bin/stroppy:ro',spec['workload']['stroppy_image'],'run',workload,'-d','pg','-D','url='+url,'--executor','constant-vus','--duration','5s','--vus','2','--log-level','info','--log-mode','production']
   if workload!='simple':
    args.insert(args.index(workload)+1,'crdb.sql')
    if '--local-sql' in sys.argv:
     sql=Path('/home/yaroher/devel/github/stroppy-io/stroppy/workloads')/workload.split('/')[0]/('crdb24.sql' if '24.1' in name and workload.startswith('tpcc/') else 'crdb.sql');args[1:1]=['-v',str(sql)+':/tmp/workload.sql:ro'];args[args.index('crdb.sql')]='/tmp/workload.sql'
   if workload!='simple':args+=['--scale-factor','1','--load-workers','2','--retry-attempts','50']
   r=docker(*args,check=False);(folder/('native-'+workload.replace('/','-')+'.log')).write_text(r.stdout+r.stderr)
   print('native',workload,r.returncode,(r.stdout+r.stderr)[-800:],flush=True)
   if r.returncode or any(float(x)>0 for x in re.findall(r'terminal_errors_total\s+(\d+(?:\.\d+)?)',r.stdout+r.stderr)):raise RuntimeError('native '+workload+' failed')
 report={'status':'passed','checked_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'input_sha256':hashlib.sha256(source.read_bytes()).hexdigest(),'checks':proof};(folder/'report.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))
finally:
 for physical in reversed(containers):
  r=docker('logs',physical,check=False);(folder/(physical+'.log')).write_text(r.stdout+r.stderr);docker('rm','-f','-v',physical,check=False)
 docker('network','rm',net,check=False)
