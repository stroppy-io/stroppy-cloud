import subprocess,time,os,signal,json
from pathlib import Path
p=Path('/tmp/stroppy-catalog-0z9hx668');campaigns=['crdb-matrix','ydb-metricsretry','ydb-channelretry']
# Only replace local observation processes; no Graphene/YC cancellation.
expected={str(p/c/f) for c in campaigns for f in ['monitor.py','watch_cells.py']}
for proc in Path('/proc').iterdir():
 if not proc.name.isdigit():continue
 try:args=proc.joinpath('cmdline').read_bytes().split(b'\0')
 except OSError:continue
 if len(args)>1 and args[1].decode(errors='replace') in expected:
  os.kill(int(proc.name),signal.SIGTERM)
children=[];handles=[]
try:
 for c in campaigns:
  for f,log in [('monitor.py','monitor.log'),('watch_cells.py','watcher-resumed.log')]:
   handle=(p/c/log).open('a');handles.append(handle);child=subprocess.Popen(['python3',str(p/c/f)],stdout=handle,stderr=subprocess.STDOUT);children.append((c,f,child))
 print('resumed local observers; strict YDB diagnostic classification enabled; existing proofs retained',flush=True)
 while children:
  remaining=[]
  for c,f,child in children:
   code=child.poll()
   if code is None:remaining.append((c,f,child))
   else:print(c,f,'finished',code,flush=True)
  children=remaining
  if children:
   result=subprocess.run(['python3',str(p/'refresh_progress.py')],capture_output=True,text=True,timeout=20)
   if result.returncode:print('CSV refresh deferred after read race',flush=True)
   time.sleep(30)
finally:
 for _,_,child in children:
  if child.poll() is None:child.terminate()
 for handle in handles:handle.close()
