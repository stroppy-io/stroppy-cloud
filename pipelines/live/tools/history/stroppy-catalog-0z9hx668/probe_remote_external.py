import os,pty,subprocess,select,time,json,base64,sys,hashlib
from pathlib import Path
agent=sys.argv[1];target=Path(sys.argv[2]);CLI='/tmp/stroppy-catalog-0z9hx668/bin/graphenectl'
live=Path('/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live')
pgwire=(live/'picodata_probe.py').read_text()
main=(live/'external_canary_probe.py').read_text()
code=("import sys,types\nmodule=types.ModuleType('picodata_probe')\nexec("+repr(pgwire)+",module.__dict__)\nsys.modules['picodata_probe']=module\n"+main).encode()
fixture=json.loads(Path('/tmp/stroppy-catalog-0z9hx668/external-live/fixture.json').read_text())
database=next(c['env']['POSTGRES_DB'] for c in fixture['containers'] if c['name']=='db-1-postgres')
password=''
payload=base64.b64encode(json.dumps({'code':code.decode(),'password':password,'args':['probe','--database',database]}).encode()).decode()
lines=["stty -echo", "python3 - <<'STROPPY_PROBE'",'import sys,base64,json,os',"p=json.loads(base64.b64decode(",*[repr(payload[i:i+1000]) for i in range(0,len(payload),1000)],"))", "sys.argv=p['args']; os.environ['PGPASSWORD']=p['password']", "exec(p['code'],{'__name__':'__main__'})",'STROPPY_PROBE','exit','']
master,slave=pty.openpty();proc=subprocess.Popen([CLI,'--config','/tmp/stroppy-catalog-0z9hx668/forward-config.yaml','--context','native-live','-n','t-stroppy-live','agent','shell',agent],stdin=slave,stdout=slave,stderr=slave);os.close(slave)
buf=b'';deadline=time.monotonic()+60;sent=False
try:
 while time.monotonic()<deadline:
  ready,_,_=select.select([master],[],[],.2)
  if ready:
   try:chunk=os.read(master,65536)
   except OSError:break
   if not chunk:break
   buf+=chunk
   if not sent and b'$ ' in buf:
    os.write(master,'\n'.join(lines).encode());sent=True
  if proc.poll() is not None:break
finally:
 if proc.poll() is None:proc.terminate()
 proc.wait(timeout=10);os.close(master)
raw=buf.decode(errors='replace');target.with_suffix('.raw').write_text(raw)
rows=[]
for line in raw.splitlines():
 if line.startswith('{"probe":'):rows.append(json.loads(line))
if len(rows)!=1 or rows[0].get('status')!='passed':
 print('Probe incomplete:',raw[-1000:]);raise SystemExit(1)
result={'agent':agent,'source':'Read-only pgwire canary query through the owned fixture agent; no Docker or privilege escalation','probe_sha256':hashlib.sha256(code).hexdigest(),'checks':rows[0]}
target.write_text(json.dumps(result,indent=2)+'\n');print(agent,'external canary and read-only role passed')
