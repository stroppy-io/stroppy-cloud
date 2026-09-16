import os,pty,subprocess,select,time,json,base64,sys,hashlib
from pathlib import Path
agent=sys.argv[1];target=Path(sys.argv[2]);CLI='/tmp/stroppy-clusters-r1gx3otm/bin/graphenectl'
code=Path('/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live/postgres_probe.py').read_bytes()
b64=base64.b64encode(code).decode()
argv=['probe','--orioledb']+(['--seed'] if '--seed' in sys.argv else [])
lines=["python3 - <<'STROPPY_PROBE'",'import sys,base64',"sys.argv="+repr(argv),"exec(base64.b64decode(",*[repr(b64[i:i+1000]) for i in range(0,len(b64),1000)],"),{'__name__':'__main__'})",'STROPPY_PROBE','exit','']
master,slave=pty.openpty();proc=subprocess.Popen([CLI,'--config','/tmp/stroppy-native-0izo0l0o/config/graphene/config.yaml','--context','native-live','-n','t-stroppy-live','agent','shell',agent],stdin=slave,stdout=slave,stderr=slave);os.close(slave)
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
 if line.startswith('{"query":'):rows.append(json.loads(line))
if len(rows)!=8:
 print('Probe incomplete:',raw[-1500:]);raise SystemExit(1)
assert rows[1]['rows']==[['100']],rows[1]
assert rows[4]['rows'][0][1:3]==['orioledb','UTF8'],rows[4]
assert rows[5]['rows'] and all(r[2]=='orioledb' for r in rows[5]['rows']),rows[5]
settings={r[0]:r[1:] for r in rows[6]['rows']}
assert settings['shared_preload_libraries'][0]=='pg_stat_statements,orioledb',settings
assert settings['pg_stat_statements.max'][1]=='configuration file',settings
assert settings['pg_stat_statements.track'][1]=='configuration file',settings
role='primary' if '--seed' in sys.argv else 'replica'
assert rows[0]['rows'][0][1]==('f' if role=='primary' else 't'),rows[0]
if role=='replica':assert rows[2]['rows'] and rows[2]['rows'][0][0]=='streaming',rows[2]
result={'agent':agent,'role':role,'source':'SQL over loopback through ordinary test agent; primary seeds 100 owned test rows','probe_sha256':hashlib.sha256(code).hexdigest(),'checks':rows}
target.write_text(json.dumps(result,indent=2)+'\n');print(agent,role,'OrioleDB table method, UTF8, pg_stat_statements and replication passed')
