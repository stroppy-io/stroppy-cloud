import subprocess,json,time,datetime,sys
from pathlib import Path
p=Path('/tmp/stroppy-catalog-0z9hx668');live=Path('/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live');image='docker.stroppy.io/library/postgres@sha256:67f41722b7a8cbdb868a44a4995c846eddfdc2973bccb291ce937dce88ad5675';name='stroppy-external-probe-'+str(time.time_ns());sql="CREATE TABLE external_fixture_canary(id integer PRIMARY KEY); INSERT INTO external_fixture_canary VALUES (197); CREATE ROLE external_reader LOGIN PASSWORD 'fixture-test-password'; GRANT CONNECT ON DATABASE stroppy TO external_reader; GRANT USAGE ON SCHEMA public TO external_reader; GRANT SELECT ON external_fixture_canary TO external_reader;";attempts=[]
def run(*args,**kw):return subprocess.run(args,capture_output=True,text=True,check=True,**kw)
try:
 run('docker','run','--detach','--name',name,'--label','stroppy.live.external-probe=true','-e','POSTGRES_PASSWORD=fixture-test-password','-e','POSTGRES_DB=stroppy','-p','127.0.0.1::5432',image)
 for _ in range(60):
  r=subprocess.run(['docker','exec',name,'pg_isready','-U','postgres','-d','stroppy'],capture_output=True)
  if r.returncode==0:break
  time.sleep(.5)
 else:raise RuntimeError('local fixture not ready')
 run('docker','exec','-i',name,'psql','-U','postgres','-d','stroppy','-v','ON_ERROR_STOP=1',input=sql)
 # Docker bridge connections require SCRAM. Test the same ordinary loopback probe
 # inside a temporary Python process sharing the database network namespace.
 probe=(live/'external_canary_probe.py').read_text();wire=(live/'picodata_probe.py').read_text();payload="import types,sys\nm=types.ModuleType('picodata_probe')\nexec("+repr(wire)+",m.__dict__)\nsys.modules['picodata_probe']=m\nexec("+repr(probe)+",{'__name__':'__main__'})\n"
 # Use the host's Python through an existing local host port only after allowing
 # this isolated test reader from the container's bridge gateway, matching trust
 # semantics; the real YC probe always uses the fixture's loopback rule.
 run('docker','exec','-i',name,'sh','-c','printf "host stroppy external_reader 0.0.0.0/0 trust\\n" > /tmp/probe-hba; cat "$PGDATA/pg_hba.conf" >> /tmp/probe-hba; cat /tmp/probe-hba > "$PGDATA/pg_hba.conf"')
 run('docker','exec',name,'psql','-U','postgres','-c','SELECT pg_reload_conf()')
 port=int(run('docker','port',name,'5432').stdout.strip().rsplit(':',1)[1]);base=['python3',str(live/'external_canary_probe.py'),'--port',str(port)]
 a=run(*base);attempts.append({'case':'canary_and_restricted_reader','status':'passed','result':json.loads(a.stdout)})
 for kind,change,restore in [('changed_canary','UPDATE external_fixture_canary SET id=198','UPDATE external_fixture_canary SET id=197'),('writer_privilege','GRANT UPDATE ON external_fixture_canary TO external_reader','REVOKE UPDATE ON external_fixture_canary FROM external_reader')]:
  run('docker','exec',name,'psql','-U','postgres','-d','stroppy','-v','ON_ERROR_STOP=1','-c',change)
  bad=subprocess.run(base,capture_output=True,text=True);assert bad.returncode!=0 and 'Canary contents or reader table privileges changed' in bad.stderr;attempts.append({'case':kind,'status':'correctly_rejected'})
  run('docker','exec',name,'psql','-U','postgres','-d','stroppy','-v','ON_ERROR_STOP=1','-c',restore)
 run(*base)
finally:
 subprocess.run(['docker','rm','-f','-v',name],capture_output=True,check=True)
(live/'external-canary-local-check.json').write_text(json.dumps({'status':'passed','checked_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'image':image,'scope':'Real local PostgreSQL canary/privilege probe only; YC external pipeline remains unverified.','cases':attempts,'cleanup':'owned local container and volume removed'},indent=2)+'\n');print('external canary probe:',len(attempts),'cases passed; local fixture removed')
