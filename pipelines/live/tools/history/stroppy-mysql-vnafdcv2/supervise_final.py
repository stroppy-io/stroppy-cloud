import subprocess,time
from pathlib import Path
p=Path(__file__).parent
names=['family211/monitor.py','single-fixed/monitor.py','collect211.py','collect_single_fixed.py','watch_verify_final.py']
running={}
for name in names:
 log=(p/(name.replace('/','-')+'.supervisor.log')).open('a')
 running[name]=subprocess.Popen(['python3',str(p/name)],stdout=log,stderr=subprocess.STDOUT)
while running:
 for name,process in list(running.items()):
  code=process.poll()
  if code is not None:
   print(name,'finished',code,flush=True);del running[name]
 time.sleep(5)
