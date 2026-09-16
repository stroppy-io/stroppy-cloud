import json,subprocess,sys,datetime
from pathlib import Path
P=Path(__file__).parent;D=P/sys.argv[1];s=json.loads((D/'state.json').read_text());rows=json.loads((D/'runs.json').read_text()) if (D/'runs.json').exists() else []
print('UTC',datetime.datetime.now(datetime.timezone.utc).isoformat());print('phases',[(r['ref'],r['phase']) for r in rows]);print('YC',{k:len(json.loads((D/(k+'-current.json')).read_text())) for k in ['vm','disk','network','subnet','sg','ydb'] if (D/(k+'-current.json')).exists()})
for name,c in s['cells'].items():
 if not any(r['ref']=='run/'+c['run_id'] and r['phase']=='Running' for r in rows):continue
 r=subprocess.run([str(P/'bin/graphenectl'),'-n','t-stroppy-live','events','run',c['run_id'],'--jq','.'],text=True,capture_output=True,timeout=30)
 if r.returncode:print('events unavailable');continue
 (D/(name+'.events.jsonl')).write_text(r.stdout);es=[json.loads(x)['raw'] for x in r.stdout.splitlines()];a=[e for e in es if 'activityTaskScheduledEventAttributes' in e];print(name,'latest activities',[(e['eventTime'],e['activityTaskScheduledEventAttributes']['activityType']['name']) for e in a[-5:]])
 print('failed activities',sum('activityTaskFailedEventAttributes' in e for e in es))
