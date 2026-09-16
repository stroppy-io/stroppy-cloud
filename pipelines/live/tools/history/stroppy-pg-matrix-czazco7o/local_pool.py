import json,subprocess,time
from pathlib import Path
out=Path('/tmp/stroppy-pg-matrix-czazco7o')
r=json.loads((out/'fixed-inputs/postgres-pgbouncer-17-simple.json').read_text())
names=[]
evidence={}
def call(args,check=True):
 p=subprocess.run(args,capture_output=True,text=True,timeout=30)
 if check and p.returncode:raise RuntimeError(str(args[:4])+': '+p.stderr+p.stdout)
 return p
try:
 for suffix in ['db-1-postgres','proxy-1-haproxy','proxy-1-pgbouncer','proxy-1-pgbouncer-exporter']:
  c=next(c for c in r['containers'] if c['name']==suffix);name='stroppy-local-'+suffix;names.append(name)
  args=['docker','run','-d','--rm','--name',name]
  if suffix!='db-1-postgres':args+=['--network','container:stroppy-local-db-1-postgres']
  else:args+=['--tmpfs','/var/lib/postgresql/data']
  for k,v in c.get('env',{}).items():args+=['-e',k+'='+v]
  for i,f in enumerate(c.get('files',[])):
   p=out/(suffix+'-'+str(i));p.write_text(f['content'].replace('${ip:db-1}','127.0.0.1'));p.chmod(0o644);args+=['-v',str(p)+':'+f['path']+':ro']
  args+=[c['image'],*c.get('cmd',[])];call(args)
  if c.get('healthcheck'):
   hc=c['healthcheck']['cmd'];hc=['sh','-c',hc[1]] if hc[0]=='CMD-SHELL' else hc[1:]
   for _ in range(30):
    p=call(['docker','exec',name,*hc],False)
    if p.returncode==0:break
    time.sleep(1)
   else:raise RuntimeError(suffix+' health failed '+call(['docker','logs',name],False).stderr)
  evidence[suffix]='running'
 p=call(['docker','exec','-e','PGPASSWORD=stroppy_postgres','stroppy-local-db-1-postgres','psql','-h','127.0.0.1','-p','6432','-U','postgres','-d','postgres','-Atc','SELECT 42'])
 assert p.stdout.strip()=='42',p.stdout;evidence['query_through_pool']=p.stdout.strip()
 for port,prefix in [(8405,'haproxy_'),(9127,'pgbouncer_')]:
  p=call(['docker','exec','stroppy-local-proxy-1-haproxy','bash','-ec',f'exec 3<>/dev/tcp/127.0.0.1/{port}; printf "GET /metrics HTTP/1.0\r\n\r\n" >&3; cat <&3'])
  metrics=[s for s in p.stdout.splitlines() if s.startswith(prefix)];assert metrics,p.stdout;evidence[str(port)]={'metric_samples':len(metrics),'errors':[x for x in metrics if 'up ' in x or 'scrape_error' in x]}
 print(json.dumps(evidence,indent=2));(out/'local-pool.evidence.json').write_text(json.dumps(evidence,indent=2))
finally:
 for name in reversed(names):call(['docker','rm','-f',name],False)
