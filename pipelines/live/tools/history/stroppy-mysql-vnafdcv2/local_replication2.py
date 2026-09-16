import json,subprocess,time,uuid
from pathlib import Path
p=Path(__file__).parent/'semisync2-inputs'
def cmd(args,check=True):
 r=subprocess.run(['docker',*args],capture_output=True,text=True,timeout=120)
 if check and r.returncode:raise RuntimeError(r.stderr[-1800:])
 return r
for v in ['8.4','8.0']:
 tag=uuid.uuid4().hex[:8];network='stroppy-semisync-'+tag;names=[];proof={}
 try:
  cmd(['network','create',network])
  data=json.loads((p/('mysql-semi-sync-'+v+'-simple.json')).read_text())
  for role in ['db','db-replica']:
   c=next(x for x in data['containers'] if x['role']==role and x['name'].endswith('-mysql'));name=tag+'-'+c['machine'];names.append(name)
   args=['run','-d','--name',name,'--network',network,'--network-alias',c['machine'],'--tmpfs','/var/lib/mysql:rw,size=2g']
   for k,val in c['env'].items():args+=['-e',k+'='+val]
   for i,f in enumerate(c['files']):
    path=p/(tag+'-'+role+'-'+str(i));content=f['content'].replace('${ip:db-1}','db-1').replace('${ip:db-replica-1}','db-replica-1');path.write_text(content);args+=['-v',str(path)+':'+f['path']+':ro']
   cmd(args+[c['image']])
   deadline=time.monotonic()+180
   while time.monotonic()<deadline:
    r=cmd(['exec',name,'sh','-c',c['healthcheck']['cmd'][-1]],check=False)
    if r.returncode==0:break
    time.sleep(3)
   else:raise RuntimeError(role+' health timeout: '+r.stdout+r.stderr)
   print(v,role,'ready',flush=True)
  root=['mysql','-h127.0.0.1','-uroot','-pstroppy_mysql','-Nse']
  cmd(['exec',names[0],*root,'CREATE TABLE stroppy.stroppy_demo(id INT PRIMARY KEY); INSERT INTO stroppy.stroppy_demo VALUES (1);'])
  for role,name in zip(['source','replica'],names):
   sql="SHOW GLOBAL STATUS LIKE 'Rpl_semi_sync%'; SELECT COUNT(*) FROM stroppy.stroppy_demo; SELECT @@GLOBAL.read_only; SHOW REPLICA STATUS;"
   r=cmd(['exec',name,*root,sql]);proof[role]=r.stdout
  assert 'Rpl_semi_sync_source_status\tON' in proof['source']
  assert 'Rpl_semi_sync_replica_status\tON' in proof['replica']
  (p/('mysql'+v+'-local-replication.json')).write_text(json.dumps({'status':'passed','version':v,'checks':proof},indent=2)+'\n')
  print(v,'local semisync passed',flush=True)
 finally:
  for name in names:
   r=cmd(['logs',name],check=False);(p/(name+'.log')).write_text(r.stdout+r.stderr);cmd(['rm','-f',name],check=False)
  cmd(['network','rm',network],check=False)
