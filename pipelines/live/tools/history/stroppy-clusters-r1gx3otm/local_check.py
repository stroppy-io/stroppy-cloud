import json,subprocess,time,sys,ipaddress,os,datetime,urllib.request
from pathlib import Path
root=Path(__file__).parent;inputpath=Path(sys.argv[1]);spec=json.loads(inputpath.read_text());label=inputpath.stem;out=root/('local-'+label);out.mkdir(mode=0o700,exist_ok=True)
name='stroppy-cluster-local-'+spec['run_id'][:8];containers=[];mapping={};result={'status':'running','input':str(inputpath),'started_at':datetime.datetime.now(datetime.timezone.utc).isoformat()}
def cmd(args,check=True,timeout=120):return subprocess.run(args,text=True,capture_output=True,check=check,timeout=timeout)
def expand(value):
 for m,addr in mapping.items():value=value.replace('${ip:'+m+'}',addr)
 return value
def sql(container,query,proxy=False):
 c=next(c for c in spec['containers'] if name+'-'+c['name']==container)
 maria=c['image'].split('/')[-1].startswith('mariadb:') or proxy
 args=['docker','exec','-e','MYSQL_PWD=stroppy_mysql',container,'mariadb' if maria else 'mysql','-h127.0.0.1','-uroot','-Nse',query]
 if proxy:args+=['--skip-ssl','-P6033']
 return cmd(args,check=False,timeout=20)
try:
 cmd(['docker','network','create',name]);net=json.loads(cmd(['docker','network','inspect',name]).stdout)[0];subnet=ipaddress.ip_network(net['IPAM']['Config'][0]['Subnet'])
 for i,m in enumerate(spec['machines']):mapping[m['name']]=str(subnet.network_address+i+10)
 targets=[c for c in spec['containers'] if c['name'].endswith(('-mysql','-mariadb','-proxysql'))]
 pending=targets.copy();ready=set()
 while pending:
  c=next(c for c in pending if set(c.get('depends_on',[]))<=ready);pending.remove(c);cn=name+'-'+c['name'];containers.append(cn)
  args=['docker','run','-d','--name',cn,'--network',name,'--ip',mapping[c['machine']]]
  if not c['name'].endswith('-proxysql'):args+=['--tmpfs','/var/lib/mysql:rw,size=2g']
  for k,v in c.get('env',{}).items():args+=['-e',k+'='+expand(v)]
  for i,f in enumerate(c.get('files',[])):
   fp=out/(c['name']+'-'+str(i));fp.write_text(expand(f['content']));fp.chmod(int(f.get('mode','0644'),8));args+=['-v',str(fp)+':'+f['path']+':ro']
  args+=[c['image']]+[expand(x) for x in c.get('cmd',[])]
  cmd(args,timeout=300)
  health=[expand(x) for x in c['healthcheck']['cmd']];hc=['bash','-c',health[1]] if health[0]=='CMD-SHELL' else health[1:]
  deadline=time.monotonic()+300
  while time.monotonic()<deadline:
   state=cmd(['docker','inspect','--format','{{.State.Running}}',cn]).stdout.strip()
   if state != 'true': raise RuntimeError(c['name']+' exited before readiness')
   r=cmd(['docker','exec',cn,*hc],check=False,timeout=20)
   if r.returncode==0:ready.add(c['name']);print(label,c['name'],'ready',flush=True);break
   time.sleep(2)
  else:
   (out/(c['name']+'.health-error')).write_text(r.stdout+r.stderr)
   raise RuntimeError(c['name']+' readiness failed: '+r.stderr[-1200:])
 proxy=next((name+'-'+c['name'] for c in targets if c['name'].endswith('-proxysql')),name+'-'+targets[0]['name'])
 has_proxy=proxy.endswith('-proxysql')
 r=sql(proxy,'CREATE TABLE stroppy.cluster_probe(id INT PRIMARY KEY, v INT); INSERT INTO stroppy.cluster_probe VALUES (1,101);',has_proxy)
 if r.returncode:raise RuntimeError('Proxy write: '+r.stderr)
 for _ in range(10):
  immediate=sql(proxy,'SELECT v FROM stroppy.cluster_probe WHERE id=1;',has_proxy)
  if immediate.returncode or immediate.stdout.strip()!='101': raise RuntimeError('immediate read through proxy failed: '+immediate.stdout+immediate.stderr)
 result['immediate_proxy_reads']=10
 nodes=[]
 for c in targets:
  if c['name'].endswith('-proxysql'):continue
  cn=name+'-'+c['name'];deadline=time.monotonic()+30
  while time.monotonic()<deadline:
   r=sql(cn,'SELECT v FROM stroppy.cluster_probe WHERE id=1;')
   if r.returncode==0 and r.stdout.strip()=='101':break
   time.sleep(1)
  else:raise RuntimeError('row not replicated: '+c['name']+' '+r.stderr)
  exporter=next(x for x in spec['containers'] if x['machine']==c['machine'] and x['name'].endswith('-mysqld-exporter'))
  en=name+'-'+exporter['name'];containers.append(en)
  args=['docker','run','-d','--name',en,'--network',name,'-p','127.0.0.1::9104']
  for key,value in exporter['env'].items():args+=['-e',key+'='+value]
  args+=[exporter['image']]+[arg.replace('127.0.0.1:3306',mapping[c['machine']]+':3306') for arg in exporter['cmd']]
  cmd(args)
  port=json.loads(cmd(['docker','inspect',en]).stdout)[0]['NetworkSettings']['Ports']['9104/tcp'][0]['HostPort']
  time.sleep(1)
  metrics=urllib.request.urlopen('http://127.0.0.1:'+port+'/metrics',timeout=20).read().decode()
  collectors=[line for line in metrics.splitlines() if line.startswith('mysql_exporter_collector_success{')]
  assert len(collectors)>=4 and all(line.endswith(' 1') for line in collectors),collectors
  (out/(en+'.metrics')).write_text(metrics)
  nodes.append({'node':c['name'],'version':sql(cn,'SELECT VERSION();').stdout.strip(),'replicated_value':r.stdout.strip(),'exporter_collectors':collectors})
 result.update(status='passed',nodes=nodes,proxy_write=True)
except Exception as exc:
 result.update(status='failed',error=str(exc))
 raise
finally:
 for cn in containers:
  r=cmd(['docker','logs',cn],check=False);(out/(cn+'.log')).write_text(r.stdout+r.stderr)
 for cn in reversed(containers):cmd(['docker','rm','-f',cn],check=False)
 cmd(['docker','network','rm',name],check=False)
 result['finished_at']=datetime.datetime.now(datetime.timezone.utc).isoformat();(out/'report.json').write_text(json.dumps(result,indent=2)+'\n');print(label,result['status'],flush=True)
