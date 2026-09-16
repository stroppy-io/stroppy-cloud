import sys,json,subprocess,ipaddress,time,hashlib,datetime,yaml,re,os
from pathlib import Path
P=Path(__file__).parent;source=Path(sys.argv[1]);spec=json.loads(source.read_text());name=source.stem;folder=P/('local-'+name+('-probe' if '--probe' in sys.argv else ''));folder.mkdir(mode=0o700,exist_ok=True)
net='stroppy-pico-'+spec['run_id'][:8];containers=[];volumes=[]
def docker(*args,check=True):
 r=subprocess.run(['docker',*args],capture_output=True,text=True,timeout=600 if "--duration" in args else 120)
 if check and r.returncode:raise RuntimeError(str(args[:2])+': '+r.stderr[-1500:])
 return r
try:
 docker('network','create',net);subnet=json.loads(docker('network','inspect',net).stdout)[0]['IPAM']['Config'][0]['Subnet'];base=ipaddress.ip_network(subnet).network_address
 ips={m['name']:str(base+i+10) for i,m in enumerate(spec['machines'])}
 def render(s):
  for m,ip in ips.items():s=s.replace('${ip:'+m+'}',ip)
  for machine in spec["machines"]:s=s.replace("${ip:role:"+machine["role"]+"}",ips[machine["name"]])
  return s
 cts=[c for c in spec['containers'] if c['name'].endswith('-picodata')];cts.sort(key=lambda c:bool(c.get('depends_on')))
 for c in cts:
  physical=net+'-'+c['name'];args=['run','-d','--name',physical,'--network',net,'--ip',ips[c['machine']]]
  for k,v in c.get('env',{}).items():args+=['-e',k+'='+render(v)]
  for i,f in enumerate(c.get('files',[])):
   path=folder/(c['name']+'-'+str(i));path.write_text(render(f['content']));path.chmod(int(f.get('mode','0644'),8));args+=['-v',str(path)+':'+f['path']+':ro']
  for m in c.get('mounts',[]):
   data=net+'-'+c['machine']+'-data';docker('volume','create','--driver','local','--opt','type=tmpfs','--opt','device=tmpfs','--opt','o=size=2g,uid=1000,gid=1000,mode=0750',data);volumes.append(data);c['local_data']=data;args+=['-v',data+':'+m['target']]
  args+=[c['image'],*[render(x) for x in c.get('cmd',[])]];docker(*args);containers.append(physical)
 for physical in containers:
  for attempt in range(20):
   state=json.loads(docker('inspect',physical).stdout)[0]['State']
   if not state['Running']:raise RuntimeError(physical+' exited '+str(state['ExitCode'])+' '+docker('logs',physical,check=False).stderr[-2000:])
   check=next(c['healthcheck']['cmd'][1] for c in cts if net+'-'+c['name']==physical)
   r=docker('exec',physical,'sh','-c',check,check=False)
   if r.returncode==0:break
   time.sleep(1)
  else:raise RuntimeError(physical+' readiness timed out: '+docker('logs',physical,check=False).stderr[-2000:])
  print(physical,'Lua readiness',r.stdout,flush=True)
 for c in spec['containers']:
  if not c['name'].endswith('-init'):continue
  physical=net+'-'+c['name'];db=next(x for x in cts if x['machine']==c['machine'])
  docker('run','-d','--name',physical,'--network',net,'-v',db['local_data']+':/var/lib/picodata','--entrypoint',c['entrypoint'][0],c['image'],*c['cmd']);containers.append(physical)
  for _ in range(30):
   if docker('exec',physical,'sh','-c',c['healthcheck']['cmd'][1],check=False).returncode==0:break
   if not json.loads(docker('inspect',physical).stdout)[0]['State']['Running']:raise RuntimeError('bootstrap exited: '+docker('logs',physical,check=False).stdout)
   time.sleep(1)
  else:raise RuntimeError('bootstrap did not complete')
  print('bootstrap',docker('logs',physical,check=False).stdout,flush=True)
 proof={}
 for c in cts:
  physical=net+'-'+c['name']
  for _ in range(30):
   r=docker('run','--rm','--network',net,'-e','PGPASSWORD='+c['env']['PICODATA_ADMIN_PASSWORD'],'postgres:17-alpine','psql','-h',ips[c['machine']],'-p','4327','-U','admin','-d','postgres','-At','-F','|','-c','SELECT name, current_state, picodata_version FROM _pico_instance;',check=False)
   if r.returncode==0:
    members=[x.split('|') for x in r.stdout.splitlines() if x]
    if len(members)==len(cts) and all(json.loads(m[1])[0]=='Online' for m in members):break
   time.sleep(1)
  else:raise RuntimeError('not all Picodata members online: '+r.stdout[-2000:]+' '+r.stderr[-1000:])
  proof[physical]=members
 metrics={}
 for c in cts:
  physical=net+'-'+c['name']
  response=docker('exec',physical,'curl','-fsS','http://127.0.0.1:8081/metrics').stdout
  (folder/(physical+'.metrics.txt')).write_text(response)
  metrics[physical]=len([x for x in response.splitlines() if x and not x.startswith('#')])
  if '--probe' in sys.argv:
   q=subprocess.run(['python3','/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live/picodata_probe.py','--host',ips[c['machine']],'--members',str(len(cts)),'--version',name.split('-')[-2]],env=dict(os.environ,PGPASSWORD=c['env']['PICODATA_ADMIN_PASSWORD']),capture_output=True,text=True);print('pgwire probe',q.stdout,q.stderr,flush=True);q.check_returncode()
 if '--native' in sys.argv:
  url='postgres://admin:'+cts[0]['env']['PICODATA_ADMIN_PASSWORD']+'@'+ips[cts[0]['machine']]+':4327?sslmode=disable&default_query_exec_mode=exec'
  for workload in ['simple','tpcb/tx','tpcc/tx']:
   native=net+'-native';containers.append(native)
   args=['run','--rm','--name',native,'--network',net,'-v',str(P/'stroppy-native/stroppy')+':/usr/local/bin/stroppy:ro',spec['workload']['stroppy_image'],'run',workload,'-d','pico','-D','url='+url,'-D','postgres.defaultQueryExecMode=exec','--executor','constant-vus','--duration','5s','--vus','2','--log-level','info','--log-mode','production']
   if workload!='simple':args+=['--scale-factor','2' if workload=='tpcc/tx' else '1','--load-workers','2','--retry-attempts','50']
   r=docker(*args,check=False);(folder/('native-'+workload.replace('/','-')+'.log')).write_text(r.stdout+r.stderr)
   print('native',workload,r.returncode,(r.stdout+r.stderr)[-1800:],flush=True)
   if r.returncode or any(float(x)>0 for x in re.findall(r'terminal_errors_total\s+(\d+(?:\.\d+)?)',r.stdout+r.stderr)):
    query="SELECT CAST(100.0 AS DOUBLE) * SUM(CASE WHEN c_credit = 'BC' THEN 1 ELSE 0 END) / COUNT(*) FROM customer WHERE c_w_id BETWEEN 1 AND 1;"
    q=docker('run','--rm','--network',net,'-e','PGPASSWORD='+cts[0]['env']['PICODATA_ADMIN_PASSWORD'],'postgres:17-alpine','psql','-h',ips[cts[0]['machine']],'-p','4327','-U','admin','-d','postgres','-At','-c',query,check=False)
    print('population SQL double probe',q.returncode,q.stdout,q.stderr,flush=True)
    raise RuntimeError('native '+workload+' failed')
 report={'status':'passed','checked_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'input_sha256':hashlib.sha256(source.read_bytes()).hexdigest(),'checks':proof,'metric_samples':metrics};(folder/'report.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))
finally:
 for physical in reversed(containers):
  r=docker('logs',physical,check=False);(folder/(physical+'.log')).write_text(r.stdout+r.stderr);docker('rm','-f','-v',physical,check=False)
 docker('network','rm',net,check=False)
 for vol in volumes:docker('volume','rm',vol,check=False)
