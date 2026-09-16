import json,subprocess,time
from pathlib import Path
p=Path(__file__).parent;d=p/'semisync'
for _ in range(30):
 if all(not json.loads((p/'fixed-mysql'/(k+'-current.json')).read_text()) for k in ['vm','disk','network','subnet','sg']):break
 time.sleep(20)
else:raise SystemExit('Previous stand cleanup did not finish; not starting another')
state=json.loads((d/'state.json').read_text())
args=['/tmp/stroppy-native-0izo0l0o/release-0210/graphenectl','--config','/tmp/stroppy-native-0izo0l0o/config/graphene/config.yaml','--context','native-live','-n','t-stroppy-live','run','start','stroppy-suite','--run-id',state['run_id'],'--params-file',str(d/'suite.json')]
r=subprocess.run(args,capture_output=True,text=True);(d/'start.stdout').write_text(r.stdout);(d/'start.stderr').write_text(r.stderr);print('start',r.returncode,state['run_id'],flush=True);r.check_returncode()
subprocess.run(['python3',str(d/'monitor.py')],check=True)
