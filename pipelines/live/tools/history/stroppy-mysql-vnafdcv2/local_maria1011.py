import json,subprocess,time,uuid
from pathlib import Path
p=Path(__file__).parent/'mariadb-correct-inputs';data=json.loads((p/'mariadb-single-10.11-simple.json').read_text());c=next(x for x in data['containers'] if x['name']=='db-1-mariadb');name='stroppy-maria-init-'+uuid.uuid4().hex[:8]
def cmd(args,check=True):
 r=subprocess.run(['docker',*args],capture_output=True,text=True,timeout=120)
 if check and r.returncode:raise RuntimeError(r.stderr[-1800:])
 return r
try:
 args=['run','-d','--name',name,'--tmpfs','/var/lib/mysql:rw,size=2g']
 for k,v in c['env'].items():args+=['-e',k+'='+v]
 for i,f in enumerate(c['files']):
  path=p/(name+'-'+str(i));path.write_text(f['content'].replace('${ip:db-1}','db-1'));args+=['-v',str(path)+':'+f['path']+':ro']
 cmd(args+[c['image']]);deadline=time.monotonic()+120
 while time.monotonic()<deadline:
  r=cmd(['exec',name,'sh','-c',c['healthcheck']['cmd'][-1]],check=False)
  if r.returncode==0:break
  time.sleep(3)
 else:raise RuntimeError('MariaDB 10.11 readiness timeout')
 r=cmd(['exec',name,'mariadb','-h127.0.0.1','-uroot','-pstroppy_mysql','-Nse','SELECT VERSION(),@@GLOBAL.tx_isolation; CREATE TABLE stroppy.test(id INT); INSERT INTO stroppy.test VALUES(1); SELECT COUNT(*) FROM stroppy.test;']);assert 'REPEATABLE-READ' in r.stdout
 (p/'local-startup.json').write_text(json.dumps({'status':'passed','version_and_isolation_and_rows':r.stdout},indent=2)+'\n');print('MariaDB 10.11 fresh startup, isolation and insert passed',flush=True)
finally:
 r=cmd(['logs',name],check=False);(p/(name+'.log')).write_text(r.stdout+r.stderr);cmd(['rm','-f',name],check=False)
