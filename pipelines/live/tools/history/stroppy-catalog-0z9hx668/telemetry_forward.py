import subprocess, threading, os, time
from pathlib import Path
P=Path("/tmp/stroppy-catalog-0z9hx668")
# SQL probes must run even while telemetry port-forward is reconnecting.
import threading
os.environ['YC_CLI_INITIALIZATION_SILENCE']='true'
stop=threading.Event()
def forward(svc,ports):
 while not stop.is_set():
  with (P/(svc+'.port-forward.log')).open('a') as log:
   proc=subprocess.Popen(['kubectl','--kubeconfig','/tmp/stroppy-mysql-vnafdcv2/kubeconfig','--context','stroppy-live-test','-n','observability','port-forward','svc/'+svc,ports],stdout=log,stderr=subprocess.STDOUT)
   try:
    while proc.poll() is None and not stop.wait(1):pass
   finally:
    if proc.poll() is None:proc.terminate()
    try:proc.wait(timeout=5)
    except subprocess.TimeoutExpired:proc.kill();proc.wait()
  stop.wait(3)
threads=[]
for svc,ports in [('victoria-logs-victoria-logs-single-server','19428:9428'),('victoria-traces-vt-single-server','18043:10428')]:
 t=threading.Thread(target=forward,args=(svc,ports));t.start();threads.append(t)
try:
 while True:time.sleep(10)
finally:
 stop.set()
 for t in threads:t.join()
