import re,select,subprocess,time

def inspect(inspector,run_id,start,end,entities):
 proc=subprocess.Popen(['kubectl','--kubeconfig','/tmp/stroppy-mysql-vnafdcv2/kubeconfig','--context','stroppy-live-test','-n','observability','port-forward','svc/vmsingle-vm',':8428'],stdout=subprocess.PIPE,stderr=subprocess.STDOUT,text=True)
 try:
  deadline=time.monotonic()+30;lines=[];port=None
  while time.monotonic()<deadline:
   if proc.poll() is not None:raise RuntimeError('port-forward ended: '+''.join(lines))
   if not select.select([proc.stdout],[],[],.5)[0]:continue
   line=proc.stdout.readline();lines.append(line)
   match=re.search(r'Forwarding from 127\.0\.0\.1:(\d+)',line)
   if match:port=match[1];break
  if not port:raise RuntimeError('port-forward timeout: '+''.join(lines))
  return inspector.inspect(run_id,'t-stroppy-live',endpoint='http://127.0.0.1:'+port,start=start,end=end,entities=entities,raw_samples=True)
 finally:
  proc.terminate();proc.wait(timeout=10)
