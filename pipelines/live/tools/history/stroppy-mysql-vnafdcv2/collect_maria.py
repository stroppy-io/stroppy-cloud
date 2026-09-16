import json,subprocess,concurrent.futures,time
from pathlib import Path
P=Path(__file__).parent
CLI='/tmp/stroppy-native-0izo0l0o/release-0210/graphenectl'
def collect(directory):
 state=json.loads((directory/'state.json').read_text())
 for name,cell in state['cells'].items():
  while True:
   listed=json.loads(subprocess.check_output([CLI,'-n','t-stroppy-live','run','list','--jq','.'],text=True))
   row=next((r for r in listed['resources'] if r['ref']=='run/'+cell['run_id']),None)
   if row and row['phase'] in ['Completed','Failed','Canceled']:break
   time.sleep(20)
  r=subprocess.run([CLI,'-n','t-stroppy-live','run','result',cell['run_id'],'--jq','.'],capture_output=True,text=True)
  (directory/(name+'.result.stdout')).write_text(r.stdout)
  (directory/(name+'.result.stderr')).write_text(r.stderr)
  try:
   d=json.loads(r.stdout);(directory/(name+'.result.json')).write_text(json.dumps(d,indent=2))
   print(name,r.returncode,[(s['name'],s['status'],s.get('metrics',{}).get('terminal_errors_total',{}).get('value')) for s in d.get('segments',[])],flush=True)
  except Exception:print(name,'result unavailable',r.returncode,flush=True)
with concurrent.futures.ThreadPoolExecutor() as ex:list(ex.map(collect,[P/'maria']))
