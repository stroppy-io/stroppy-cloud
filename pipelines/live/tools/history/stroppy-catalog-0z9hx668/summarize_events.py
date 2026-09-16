import sys,json,subprocess,base64
from pathlib import Path
p=Path(sys.argv[1]);s=json.loads((p/'state.json').read_text());name=sys.argv[2] if len(sys.argv)>2 else next(iter(s['cells']));rid=s['cells'][name]['run_id']
r=subprocess.run(['/tmp/stroppy-catalog-0z9hx668/bin/graphenectl','-n','t-stroppy-live','events','run',rid,'--jq','.'],capture_output=True,text=True,timeout=40);r.check_returncode();(p/(name+'.events.jsonl')).write_text(r.stdout);rows=[json.loads(x) for x in r.stdout.splitlines()];scheduled={}
for e in rows:
 raw=e.get('raw',{});a=raw.get('activityTaskScheduledEventAttributes',{})
 if a:scheduled[str(e['eventId'])]=a['activityType']['name']
 a=raw.get('activityTaskCompletedEventAttributes',{})
 if a and scheduled.get(str(a['scheduledEventId'])) in ['docker.install','stroppy.database.ready','stroppy.segment.run']:
  for pay in a.get('result',{}).get('payloads',[]):
   v=json.loads(base64.b64decode(pay['data']));q=v.get('result',{});print(scheduled[str(a['scheduledEventId'])],{k:v.get(k) for k in ['ready','query_attempts','attempts','version']},q.get('name'),q.get('status'),'terminal_errors',q.get('metrics',{}).get('terminal_errors_total'))
print('latest scheduled',list(scheduled.items())[-4:])
