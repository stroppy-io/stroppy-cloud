import json,subprocess,time,socket,hashlib,datetime,ipaddress
from pathlib import Path
P=Path(__file__).parent; out=P/'ydb-channel-local';out.mkdir(mode=0o700,exist_ok=True)
reports=[]
def docker(*args,check=True):
 r=subprocess.run(['docker',*args],text=True,capture_output=True,timeout=120)
 if check and r.returncode:raise RuntimeError(r.stderr[-1500:])
 return r
for source in sorted((P/'ydb-channel-inputs').glob('ydb-*.json')):
 spec=json.loads(source.read_text()); c=next(c for c in spec['containers'] if c['name']=='db-1-ydb');name='stroppy-channel-'+spec['run_id'][:8]; folder=out/source.stem;folder.mkdir(mode=0o700,exist_ok=True)
 try:
  docker('network','create',name); subnet=json.loads(docker('network','inspect',name).stdout)[0]['IPAM']['Config'][0]['Subnet']; base=ipaddress.ip_network(subnet).network_address
  ips={m['name']:str(base+i+10) for i,m in enumerate(spec['machines'])};ip=ips['db-1']
  def render(v):
   for n,a in ips.items():v=v.replace('${ip:'+n+'}',a)
   for m in spec['machines']:v=v.replace('${ip:role:'+m['role']+'}',ips[m['name']])
   return v
  args=['run','-d','--name',name,'--network',name,'--ip',ip,'--memory','4g','--entrypoint','/ydbd']
  for i,f in enumerate(c['files']):
   path=folder/('config-'+str(i));path.write_text(render(f['content']));path.chmod(0o644);args+=['-v',str(path)+':'+f['path']+':ro']
  data=folder/'data';data.mkdir(exist_ok=True)
  with (data/'pdisk-1.data').open('wb') as f:f.truncate(40*1024**3)
  for m in c.get('mounts',[]):args+=['-v',str(data)+':'+m['target']]
  args += [c['image'],*[render(x) for x in c['cmd']]]
  docker(*args);ready=False
  for attempt in range(30):
   time.sleep(1);state=json.loads(docker('inspect',name).stdout)[0]['State']
   if not state['Running']:raise RuntimeError('YDB exited '+str(state['ExitCode'])+': '+docker('logs',name,check=False).stderr[-1200:])
   try:
    with socket.create_connection((ip,2135),timeout=1):pass
    with socket.create_connection((ip,8765),timeout=1):pass
    ready=True
   except OSError:pass
   if ready and attempt>=9:break
  assert ready,'gRPC/monitoring not listening'
  reports.append({'input':source.name,'input_sha256':hashlib.sha256(source.read_bytes()).hexdigest(),'image':c['image'],'status':'passed','grpc_listening':True,'monitoring_listening':True,'scope':'single storage process startup; distributed quorum and workloads are verified separately on YC'})
  print(source.name,'startup passed',flush=True)
 finally:
  r=docker('logs',name,check=False);(folder/'container.log').write_text(r.stdout+r.stderr);docker('rm','-f','-v',name,check=False);docker('network','rm',name,check=False)
report={'status':'passed','checked_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'cases':reports};(out/'report.json').write_text(json.dumps(report,indent=2)+'\n')
Path('/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live/ydb-channel-local-check.json').write_text(json.dumps(report,indent=2)+'\n')
