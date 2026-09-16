import json,subprocess,time,tempfile,re,hashlib,datetime
from pathlib import Path
p=Path('/tmp/stroppy-catalog-0z9hx668/external-live');f=json.loads((p/'fixture-retry.json').read_text());c=next(c for c in f['containers'] if c['name']=='db-1-postgres');name='stroppy-external-init-'+str(time.time_ns());native=name+'-native';paths=[];directory=Path(tempfile.mkdtemp(prefix='fixture-init-',dir=p));created=[]
def run(args,check=True,timeout=60):
 r=subprocess.run(args,capture_output=True,text=True,timeout=timeout)
 if check:r.check_returncode()
 return r
try:
 args=['docker','run','-d','--name',name,'--label','stroppy.live.external-fixture=true','--mount','type=volume,destination=/data']
 for k,v in c['env'].items():args+=['-e',k+'='+v]
 for i,file in enumerate(c['files']):
  content=file['content']
  if file['path'].endswith('postgresql.conf'):
   content=re.sub(r'(?m)^shared_buffers\s*=.*$',"shared_buffers = '16MB'",content)
  source=directory/str(i);source.write_text(content);source.chmod(0o644);args+=['-v',str(source)+':'+file['path']+':ro']
 args+=[c['image'],*c['cmd']];run(args);created.append(name)
 for _ in range(100):
  ready=run(['docker','exec',name,'sh','-c',c['healthcheck']['cmd'][1]],False)
  if ready.returncode==0:break
  time.sleep(.2)
 else:raise RuntimeError('SQL canary health failed during local initialization')
 logs=run(['docker','logs',name]);(p/'local-fixture-postgres.log').write_text(logs.stdout+logs.stderr);assert 'ERROR:' not in logs.stdout+logs.stderr and 'FATAL:' not in logs.stdout+logs.stderr
 ip=run(['docker','inspect',name,'--format','{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}']).stdout.strip();url=f['workload']['url'].replace('${ip:db-1}',ip);query=f['workload']['segments'][0]['workload']['sql_body'];created.append(native)
 result=run(['docker','run','--rm','--name',native,f['workload']['stroppy_image'],'run','execute_sql','-d','pg','-D','url='+url,'--sql-body',query,'--executor','constant-vus','--duration','5s','--vus','2','--log-level','warn']);log=result.stdout+result.stderr;(p/'local-fixture-native.log').write_text(log);metrics={}
 for k in ['iterations_total','run_query_operations_total','terminal_errors_total','failed_queries_total','queries_per_second']:
  a=re.findall(r'^\s*'+k+r'\s+([0-9.eE+-]+)',log,re.M);assert len(a)==1,(k,a);metrics[k]=float(a[0])
 assert metrics['iterations_total']>0 and metrics['terminal_errors_total']==metrics['failed_queries_total']==0
 # Authentication and row privileges passed through an actual SCRAM bridge connection.
 proof={'status':'passed','checked_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'database':c['env']['POSTGRES_DB'],'postgres_image':c['image'],'stroppy_image':f['workload']['stroppy_image'],'init_script_sha256':hashlib.sha256(c['files'][-1]['content'].encode()).hexdigest(),'healthcheck':'read-only canary SQL, not listener availability','native_measurements':metrics,'scope':'Real PostgreSQL entrypoint and init files, database derived from compiled POSTGRES_DB, SCRAM connection from a separate native container. Local shared_buffers lowered to 16MB; YC resource settings unchanged.'}
finally:
 for container in reversed(created):run(['docker','rm','-f','-v',container],False)
Path('/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live/external-fixture-local-check.json').write_text(json.dumps(proof,indent=2)+'\n');print('compiled fixture init, SQL canary health and SCRAM native workload passed; owned local containers and volumes removed',metrics)
