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
result=json.loads((P/(name+('.partial-result.json' if '--partial' in sys.argv else '.result.json'))).read_text())
inputcell=next(x for x in json.loads((P/'suite.json').read_text())['cells'] if x['id']==name)['run_spec']
end=dt.datetime.now(dt.timezone.utc).isoformat()
diagnostic='--allow-workload-errors' in sys.argv
exporter_entities=['docker/'+cell['uuid']+'-'+c['name'] for c in inputcell['containers'] if c.get('scrape')]
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
   write(P/(name+'.native-diagnostic.json'),n)
   print(name,'native persisted metrics verified')
  finally:
   if forward.poll() is None:forward.terminate()
   try:forward.wait(timeout=5)
   except subprocess.TimeoutExpired:forward.kill();forward.wait()
