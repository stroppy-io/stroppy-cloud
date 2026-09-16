import sys,os,json,datetime as dt,subprocess,importlib.util
from pathlib import Path
P=Path(sys.argv[1]);name=sys.argv[2];outname=sys.argv[3]
LIVE=Path('/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live')
CLI='/tmp/stroppy-catalog-0z9hx668/bin/graphenectl'
os.environ['PATH']=str(Path(CLI).parent)+os.pathsep+os.environ['PATH']
sys.dont_write_bytecode=True
S=json.loads((P/'state.json').read_text());cell=S['cells'][name]
def mod(name):
 spec=importlib.util.spec_from_file_location(name,LIVE/(name+'.py'));m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m);return m
def write(path,d):
 temp=path.with_suffix(path.suffix+'.tmp');temp.write_text(json.dumps(d,indent=2)+'\n');os.replace(temp,path)
# Retry only transient failures in read-only CLI/backend inspection.
_original_check_output = subprocess.check_output
def _read_with_retry(*args, **kwargs):
 import time
 kwargs.setdefault('stderr', subprocess.PIPE)
 for attempt in range(4):
  try:return _original_check_output(*args, **kwargs)
  except subprocess.CalledProcessError as exc:
   stderr=exc.stderr.decode(errors='replace') if isinstance(exc.stderr,bytes) else (exc.stderr or '')
   if attempt==3 or not any(word in stderr.lower() for word in ['tls handshake timeout','context deadline exceeded','connection reset','unexpected eof']):raise
   time.sleep(1)
subprocess.check_output=_read_with_retry
result=json.loads((P/(name+'.result.json')).read_text())
inputcell=next(x for x in json.loads((P/'suite.json').read_text())['cells'] if x['id']==name)['run_spec']
end=dt.datetime.now(dt.timezone.utc).isoformat()
diagnostic='--allow-workload-errors' in sys.argv
exporter_entities=['docker/'+cell['uuid']+'-'+c['name'] for c in inputcell['containers'] if c.get('scrape')]
import hashlib
cache_path=P/(name+'.checked-metrics-cache.json')
cache_key=hashlib.sha256(json.dumps({'input':inputcell,'result':result,'run_id':cell['run_id'],'start':S['started_at'],'diagnostic':diagnostic,'inspectors':[hashlib.sha256((LIVE/(f+'.py')).read_bytes()).hexdigest() for f in ['inspect_native_metrics','inspect_metrics']]},sort_keys=True).encode()).hexdigest()
cache=json.loads(cache_path.read_text()) if cache_path.exists() else {}
if cache.get('key')==cache_key:
 n,m=cache['native'],cache['components']
 print(name,'retained previously checked persisted metric snapshot',flush=True)
else:
 if cell['database'] in ['ydb','cockroach']:
  # Distributed databases emit thousands of series per member. Stream original samples
  # from the same persistent backend instead of exceeding the CLI RPC payload.
  import re,time
  with (P/(name+'.metrics-forward.log')).open('w+') as forward_log:
   forward=subprocess.Popen(['kubectl','--kubeconfig','/tmp/stroppy-mysql-vnafdcv2/kubeconfig','--context','stroppy-live-test','-n','observability','port-forward','--address','127.0.0.1','svc/vmsingle-vm',':8428'],stdout=forward_log,stderr=subprocess.STDOUT)
   try:
    endpoint=None
    for _ in range(100):
     forward_log.seek(0);match=re.search(r'Forwarding from 127[.]0[.]0[.]1:(\d+)',forward_log.read())
     if match:endpoint='http://127.0.0.1:'+match.group(1);break
     if forward.poll() is not None:raise RuntimeError('metrics port-forward exited')
     time.sleep(.2)
    if not endpoint:raise RuntimeError('metrics port-forward did not become ready')
    n=mod('inspect_native_metrics').inspect(CLI,cell['run_id'],result,require_zero_errors=not diagnostic,endpoint=endpoint)
    m=mod('inspect_metrics').inspect(cell['run_id'],'t-stroppy-live',start=S['started_at'],end=end,entities=exporter_entities,endpoint=endpoint,raw_samples=True)
   finally:
    if forward.poll() is None:forward.terminate()
    try:forward.wait(timeout=5)
    except subprocess.TimeoutExpired:forward.kill();forward.wait()
 else:
  n=mod('inspect_native_metrics').inspect(CLI,cell['run_id'],result,require_zero_errors=not diagnostic)
  m=mod('inspect_metrics').inspect(cell['run_id'],'t-stroppy-live',start=S['started_at'],end=end,entities=exporter_entities)
expected_exporters=sum(bool(c.get('scrape')) for c in inputcell['containers'])
expected_disks=sum(len(c.get('disks',[])) for c in inputcell['machines'])
assert len(m['exporters'])==expected_exporters,(len(m['exporters']),expected_exporters)
write(P/(name+'.filesystem-check.json'),{'expected':expected_disks,'observed':m['data_filesystems'],'query_range':m['query_range']})
assert len(m['data_filesystems'])==expected_disks,(len(m['data_filesystems']),expected_disks)
assert m['exporter_runs_observed']==[cell['run_id']]
assert n['status']=='passed' or diagnostic
write(cache_path,{'key':cache_key,'native':n,'components':m})
m['health_scope_note']='Persisted component metrics; native workload verifies database operations. Independent membership checks recorded separately where applicable.'
write(P/(name+'.native-diagnostic.json'),n)
write(P/(name+'.metrics-diagnostic.json'),m)
l=mod('inspect_component_logs').inspect(cell['run_id'],'t-stroppy-live','http://127.0.0.1:19428',start=result['segments'][0]['started_at'],end=result['segments'][-1]['finished_at'])
diagnostics=LIVE/(outname+'.probe-diagnostics.json')
if diagnostics.exists():
 d=json.loads(diagnostics.read_text());assert d['run_id']==cell['run_id']
 allowed={x['message_sha256']:x for x in d['observations']}
 remaining=[]
 for x in l['workload_error_candidates']:
  if x['message_sha256'] in allowed:
   assert x['entity']==allowed[x['message_sha256']]['entity']
   l['workload_observations'].append(allowed[x['message_sha256']])
   classification=allowed[x['message_sha256']].get('classification','diagnostic_probe_query_error')
   l['observation_counts'][classification]=l['observation_counts'].get(classification,0)+1
  else:remaining.append(x)
 l['workload_error_candidates']=remaining
assert not l['workload_error_candidates'],l['workload_error_candidates']
expected_entities={'docker/'+cell['uuid']+'-'+c['name']:c for c in inputcell['containers']}
missing=set(expected_entities)-set(l['component_logs'])
l['silent_initializers']=[]
if missing:
 import base64,hashlib
 raw_events=subprocess.check_output([CLI,'-n','t-stroppy-live','events','run',cell['run_id'],'--jq','.'],text=True,timeout=40)
 (P/(name+'.verification-events.jsonl')).write_text(raw_events)
 ready=set()
 for line in raw_events.splitlines():
  attrs=json.loads(line).get('raw',{}).get('workflowExecutionSignaledEventAttributes',{})
  for payload in attrs.get('input',{}).get('payloads',[]):
   event=json.loads(base64.b64decode(payload['data']))
   if isinstance(event,dict) and event.get('name')=='container.ready':ready.add(event['payload']['container'])
 for entity in sorted(missing):
  c=expected_entities[entity];command=' '.join(c.get('cmd',[]))
  assert c['name'].endswith('-init') and c['name'] in ready and 'grep -qx' in command and 'touch /tmp/stroppy-init-done' in command,(entity,'unexpected missing logs')
  l['silent_initializers'].append({'entity':entity,'status':'ready','stdout':'not emitted by quiet bootstrap command','evidence':'persisted container.ready workflow signal after marker healthcheck','command_sha256':hashlib.sha256(command.encode()).hexdigest()})
assert set(l['component_logs'])<=set(expected_entities),l['component_logs']
t=mod('inspect_traces').inspect(cell['run_id'],'t-stroppy-live','http://127.0.0.1:18043/select/jaeger',S['started_at'],end)
assert t['trace_count']>0 and t['matching_spans']>0
artifacts=[]
now=dt.datetime.now(dt.timezone.utc)
for ref in result['artifacts']:
 r=json.loads(subprocess.check_output([CLI,'-n','t-stroppy-live','get',ref,'--jq','.resource'],text=True,timeout=30))
 assert r['phase']=='ready' and r['owner']=='stand/stroppy-run'
 state=r['state'];keep=state.get('keepUntil')
 if ref.endswith('-config'):assert not keep or keep.startswith('0001-')
 else:
  assert ref.endswith('-log') and keep
  assert 29*86400<(dt.datetime.fromisoformat(keep.replace('Z','+00:00'))-now).total_seconds()<31*86400
 artifacts.append({'ref':ref,'owner':r['owner'],'keepUntil':keep,**state['blob']})
readiness=[a for a in artifacts if '-managed-ydb-readiness-' in a['ref']]
assert not readiness or len(readiness)==2 and inputcell.get('managed_ydb') is not None
assert len(artifacts)==len(result['segments'])*2+len(readiness)
write(P/(name+'.blobs.json'),artifacts)
logsdir=P/(name+'-artifacts');logsdir.mkdir(mode=0o700,exist_ok=True)
r=subprocess.run(['/tmp/stroppy-catalog-0z9hx668/download-logs',str(P/(name+'.blobs.json')),str(logsdir)],capture_output=True,text=True,check=True,env={**os.environ,'GRAPHENE_CONFIG':'/tmp/stroppy-catalog-0z9hx668/forward-config.yaml'})
(P/(name+'.downloads.json')).write_text(r.stdout)
for suffix,body in [('result',result),('metrics',m),('native',n),('logs',l),('traces',t),('artifacts',artifacts)]:write(LIVE/(outname+'.'+suffix+'.json'),body)
write(P/(name+('.diagnostic.json' if diagnostic else '.verified.json')),{'status':'export_verified_with_workload_errors' if diagnostic else 'passed','name':outname,'run_id':cell['run_id'],'checked_at':end,'native':n,'components':m,'logs':l,'traces':t,'artifacts':artifacts})
print(outname,len(n['segments']),'native segments,',len(artifacts),'downloaded artifacts, metrics/logs/traces passed')

if not diagnostic:
 scripts=[s['workload']['script'] for s in inputcell['workload']['segments']]
 if all(script=='execute_sql' for script in scripts):
  write(LIVE/(outname+'.load-progress.json'),{'status':'not_applicable','run_id':cell['run_id'],'reason':'execute_sql runs supplied statements without an insert/population phase','scripts':scripts})
 else:
  subprocess.run(['python3','/tmp/stroppy-mysql-vnafdcv2/verify_progress.py',cell['run_id'],str(LIVE/(outname+'.load-progress.json'))],check=True,timeout=40)
