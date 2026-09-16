import os,pty,subprocess,select,time,json,base64,sys,hashlib
from pathlib import Path
agent=sys.argv[1];target=Path(sys.argv[2]);CLI='/tmp/stroppy-native-0izo0l0o/release-0210/graphenectl'
code=Path('/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live/mysql_probe.py').read_bytes()
b64=base64.b64encode(code).decode()
lines=["python3 - <<'STROPPY_PROBE'",'import urllib.request,hashlib,sys,base64,pathlib',"u='https://files.pythonhosted.org/packages/7c/4c/ad33b92b9864cbde84f259d5df035a6447f91891f5be77788e2a3892bce3/pymysql-1.1.2-py3-none-any.whl'","p=pathlib.Path('/tmp/stroppy-pymysql-1.1.2.whl')","b=p.read_bytes() if p.exists() else urllib.request.urlopen(u,timeout=20).read()","assert hashlib.sha256(b).hexdigest()=='e6b1d89711dd51f8f74b1631fe08f039e7d76cf67a42a323d3178f0f25762ed9'","p='/tmp/stroppy-pymysql-1.1.2.whl'","open(p,'wb').write(b)","sys.path.insert(0,p)","exec(base64.b64decode(",*[repr(b64[i:i+1000]) for i in range(0,len(b64),1000)],"),{'__name__':'__main__'})",'STROPPY_PROBE','exit','']
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
for line in raw.splitlines():
 if line.startswith('{"server":'):
  result=json.loads(line);target.write_text(json.dumps({'agent':agent,'source':'read-only SQL through the test database account over loopback TLS','probe_sha256':hashlib.sha256(code).hexdigest(),**result},indent=2)+'\n');print(agent,'SQL probe passed');break
else:
 print('Probe failed:',raw[-1800:]);raise SystemExit(1)
