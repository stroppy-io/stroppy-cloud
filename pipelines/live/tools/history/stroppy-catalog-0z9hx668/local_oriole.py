import sys,json,subprocess,ipaddress,time,hashlib,datetime
from pathlib import Path
P=Path(__file__).parent;source=Path(sys.argv[1]);spec=json.loads(source.read_text());name=source.stem;folder=P/('local-'+name);folder.mkdir(mode=0o700,exist_ok=True)
net='stroppy-oriole-'+spec['run_id'][:8];containers=[]
def docker(*args,check=True):
 r=subprocess.run(['docker',*args],capture_output=True,text=True,timeout=120)
 if check and r.returncode:raise RuntimeError(str(args[:2])+': '+r.stderr[-1500:])
 return r
try:
 docker('network','create',net);subnet=json.loads(docker('network','inspect',net).stdout)[0]['IPAM']['Config'][0]['Subnet'];base=ipaddress.ip_network(subnet).network_address
 ips={m['name']:str(base+i+10) for i,m in enumerate(spec['machines'])}
 def render(s):
  for m,ip in ips.items():s=s.replace('${ip:'+m+'}',ip)
  return s
 cts=[c for c in spec['containers'] if c['name'].endswith('-orioledb')];cts.sort(key=lambda c:bool(c.get('depends_on')))
 for c in cts:
  physical=net+'-'+c['name'];args=['run','-d','--name',physical,'--network',net,'--ip',ips[c['machine']]]
  for k,v in c.get('env',{}).items():args+=['-e',k+'='+render(v)]
  for i,f in enumerate(c.get('files',[])):
   path=folder/(c['name']+'-'+str(i));path.write_text(render(f['content']));path.chmod(int(f.get('mode','0644'),8));args+=['-v',str(path)+':'+f['path']+':ro']
  for m in c.get('mounts',[]):args+=['--tmpfs',m['target']+':rw,size=2g,mode=1777']
  args+=[c['image'],*[render(x) for x in c.get('cmd',[])]];docker(*args);containers.append(physical)
  for attempt in range(35):
   r=docker('exec',physical,'psql','-h','127.0.0.1','-U','postgres','-d','postgres','-Atc','SELECT 1',check=False)
   if r.returncode==0:break
   state=json.loads(docker('inspect',physical).stdout)[0]['State']
   if not state['Running']:raise RuntimeError(physical+' exited '+str(state['ExitCode']))
   time.sleep(1)
  else:raise RuntimeError(physical+' readiness timed out')
  print(physical,'SQL ready',flush=True)
 primary=containers[0]
 sql="CREATE TABLE stroppy_engine_probe(id integer primary key, value text); INSERT INTO stroppy_engine_probe VALUES (1,'oriole'); CREATE INDEX ON stroppy_engine_probe(value); SELECT am.amname FROM pg_class c JOIN pg_am am ON am.oid=c.relam WHERE c.relname='stroppy_engine_probe';"
 output=docker('exec',primary,'psql','-h','127.0.0.1','-U','postgres','-d','postgres','-v','ON_ERROR_STOP=1','-Atc',sql).stdout
 assert 'orioledb' in output.splitlines(),output
 settings=docker('exec',primary,'psql','-h','127.0.0.1','-U','postgres','-d','postgres','-v','ON_ERROR_STOP=1','-Atc',"SELECT name,setting,unit,min_val,boot_val,source FROM pg_settings WHERE name LIKE 'orioledb.%buffers' OR name LIKE 'pg_stat_statements.%' OR name='shared_preload_libraries'; SELECT count(*) FROM pg_stat_statements;").stdout
 print(settings,flush=True)
 proof={}
 for physical in containers:
  for _ in range(20):
   result=docker('exec',physical,'psql','-h','127.0.0.1','-U','postgres','-d','postgres','-v','ON_ERROR_STOP=1','-Atc',"SELECT version(); SELECT extversion FROM pg_extension WHERE extname='orioledb'; SHOW data_directory; SHOW default_table_access_method; SELECT pg_is_in_recovery(); SELECT count(*) FROM stroppy_engine_probe;",check=False)
   if result.returncode==0:break
   time.sleep(1)
  assert result.returncode==0,result.stderr;proof[physical]=result.stdout.splitlines();assert proof[physical][-1]=='1'
 report={'status':'passed','checked_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'input_sha256':hashlib.sha256(source.read_bytes()).hexdigest(),'checks':proof};(folder/'report.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))
finally:
 for physical in reversed(containers):
  r=docker('logs',physical,check=False);(folder/(physical+'.log')).write_text(r.stdout+r.stderr);docker('rm','-f','-v',physical,check=False)
 docker('network','rm',net,check=False)
