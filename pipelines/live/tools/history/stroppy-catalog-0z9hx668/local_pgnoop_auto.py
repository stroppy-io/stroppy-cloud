from pathlib import Path
import subprocess,json,hashlib,time,re
p=Path('/tmp/stroppy-catalog-0z9hx668/pg-noop-package');name='stroppy-pg-noop-local-'+str(time.time_ns());native=name+'-native';img='registry.stroppy.io/stroppy-io/pg-noop:0.1.2-r1';created=[]
def run(args,check=True):
 r=subprocess.run(args,capture_output=True,text=True,timeout=45)
 if check:r.check_returncode()
 return r
try:
 run(['docker','run','-d','--name',name,img]);created.append(name)
 for _ in range(30):
  if run(['docker','exec',name,'nc','-z','127.0.0.1','5432'],False).returncode==0:break
  time.sleep(.2)
 else:raise RuntimeError('pg-noop not ready')
 args=['docker','run','--rm','--name',native,'--network','container:'+name,'docker.stroppy.io/stroppy-io/stroppy@sha256:d1083cac1321793911e39b00043afd8167ed2aa69fec5c0429fa17c7ef2ce945','run','execute_sql','-d','pg','-D','url=postgres://postgres@127.0.0.1:5432/postgres?sslmode=disable','--sql-body','--= smoke\nSELECT 1;','--executor','constant-vus','--duration','5s','--vus','2','--log-level','warn']
 created.append(native);r=run(args,False);log=r.stdout+r.stderr;(p/'native-check.log').write_text(log);print(log[-2200:]);r.check_returncode();metrics={}
 for key in ['iterations_total','run_query_operations_total','terminal_errors_total','failed_queries_total','queries_per_second']:
  vals=re.findall(r'^\s*'+key+r'\s+([0-9.eE+-]+)',log,re.M);assert len(vals)==1,(key,vals);metrics[key]=float(vals[0])
 assert metrics['iterations_total']>0 and metrics['run_query_operations_total']==metrics['iterations_total'] and metrics['terminal_errors_total']==0 and metrics['failed_queries_total']==0
 r=run(['docker','exec',name,'sha256sum','/usr/local/bin/pgnoop']);want=json.loads((p/'release-check.json').read_text());assert r.stdout.split()[0]==want['binary_sha256']
 proof={'status':'passed','scope':'local packaged pg-noop 0.1.2 with automatic CPU-based worker count, real pgwire and native Stroppy execute_sql; YC rerun pending','measurements':metrics,'binary_sha256':want['binary_sha256'],'release_archive_sha256':want['archive_sha256'],'native_log_sha256':hashlib.sha256(log.encode()).hexdigest(),'tps':'not applicable: protocol-only blackhole'}
 Path('/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live/pg-noop-local-check.json').write_text(json.dumps(proof,indent=2)+'\n')
finally:
 for c in reversed(created):
  r=run(['docker','logs',c],False);(p/(c+'.log')).write_text(r.stdout+r.stderr);run(['docker','rm','-f',c],False)
