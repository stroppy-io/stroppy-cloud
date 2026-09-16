import json,subprocess,time,concurrent.futures
from pathlib import Path
p=Path(__file__).parent;live=Path('/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live')
def verify(directory):
 state=json.loads((directory/'state.json').read_text())
 for name,c in state['cells'].items():
  if (directory/(name+'.verified.json')).exists():continue
  deadline=time.monotonic()+3600
  while time.monotonic()<deadline:
   f=directory/(name+'.result.json')
   if f.exists() and json.loads(f.read_text()).get('segments'):break
   time.sleep(15)
  else:raise RuntimeError(name+' result timeout')
  r=subprocess.run(['python3',str(p/'verify_one.py'),str(directory),name,name],capture_output=True,text=True)
  (directory/(name+'.verification.log')).write_text(r.stdout+r.stderr)
  if r.returncode:raise RuntimeError(name+' verification failed: '+r.stderr[-1800:])
  r=subprocess.run(['python3',str(p/'verify_progress.py'),c['run_id'],str(live/(name+'.load-progress.json'))],capture_output=True,text=True)
  if r.returncode:raise RuntimeError(name+' progress verification failed: '+r.stderr[-1800:])
  print(name,'native/component/log/trace/artifact/load-progress checks passed',flush=True)
with concurrent.futures.ThreadPoolExecutor() as ex:list(ex.map(verify,[p/'family211']))
