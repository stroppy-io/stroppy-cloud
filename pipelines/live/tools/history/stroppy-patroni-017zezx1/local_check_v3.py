import json,subprocess,pathlib,ipaddress,time,os
p=pathlib.Path(__file__).parent
spec=json.load(open('/tmp/stroppy-patroni-inputs-v3/postgres-patroni-ha-17-simple.json'))
net='stroppy-patroni-local'
def run(args,check=True):
 r=subprocess.run(['docker',*args],capture_output=True,text=True)
 if check and r.returncode: raise RuntimeError(str(args[:3])+': '+r.stderr[-2500:])
 return r
run(['network','create',net]); subnet=json.loads(run(['network','inspect',net]).stdout)[0]['IPAM']['Config'][0]['Subnet']
base=ipaddress.ip_network(subnet).network_address
ips={m['name']:str(base+i+10) for i,m in enumerate(spec['machines'])}
(p/'local-ips.json').write_text(json.dumps(ips))
cts=[c for c in spec['containers'] if c['name'].endswith(('-etcd','-postgres','-haproxy'))]
cts.sort(key=lambda c: {'etcd':0,'db':1,'db-replica':1,'proxy':2}[c['role']])
def render(s):
 for m,ip in ips.items(): s=s.replace('${ip:'+m+'}',ip)
 return s
for c in cts:
 name='pat-local-'+c['name'];d=p/'local-v3'/c['name'];d.mkdir(parents=True);d.chmod(0o755)
 # Each simulated VM receives a distinct container IP. Container files and commands come unchanged from RunSpec.
 args=['run','-d','--name',name,'--network',net,'--ip',ips[c['machine']]]
 for k,v in c.get('env',{}).items():args+=['-e',k+'='+render(v)]
 for f in c.get('files',[]):
  path=d/('file-'+str(len(args)));path.write_text(render(f['content']));path.chmod(int(f.get('mode','0644'),8));args+=['-v',str(path)+':'+f['path']+':ro']
 for m in c.get('mounts',[]):
  path=d/'data';path.mkdir(exist_ok=True);path.chmod(0o777);args+=['-v',str(path)+':'+m['target']]
 image='stroppy-patroni:17-4.1.5' if c['name'].endswith('-postgres') else c['image']
 args += [image,*[render(x) for x in c.get('cmd',[])]]
 run(args);print('started',name,flush=True)
(p/'local-containers.json').write_text(json.dumps(['pat-local-'+c['name'] for c in cts]))
for attempt in range(60):
 healthy=[]
 for c in cts:
  cmd=c['healthcheck']['cmd'];cmd=['sh','-c',cmd[1]] if cmd[0]=='CMD-SHELL' else cmd[1:]
  r=run(['exec','pat-local-'+c['name'],*cmd],False)
  if not r.returncode:healthy.append(c['name'])
 if len(healthy)==len(cts):print('ALL HEALTHY',flush=True);break
 if attempt%5==0:print('healthy',healthy,flush=True)
 time.sleep(3)
else:
 for c in cts:
  print(c['name'],run(['logs','--tail','18','pat-local-'+c['name']],False).stderr,flush=True)
 raise SystemExit(1)
