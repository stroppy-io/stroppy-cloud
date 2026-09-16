import subprocess,time,json
from pathlib import Path
p=Path('/tmp/stroppy-catalog-0z9hx668');deadline=time.monotonic()+7200
while time.monotonic()<deadline:
 for script in ['autonomous_provider.py','autonomous_brief.py']:
  subprocess.run(['python3',str(p/script)],timeout=60)
 f=p/'ydb-autonomous/runs.json';rows=json.loads(f.read_text()) if f.exists() else []
 if rows and all(x['phase'] in ['Completed','Failed','Canceled'] for x in rows):break
 time.sleep(45)
