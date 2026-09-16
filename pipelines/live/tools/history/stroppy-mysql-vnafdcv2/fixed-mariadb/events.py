import subprocess,json,selectors,time
from pathlib import Path
p=Path(__file__).parent;s=json.loads((p/"state.json").read_text());cli="/tmp/stroppy-native-0izo0l0o/release-0210/graphenectl"
for name,c in s["cells"].items():
 proc=subprocess.Popen([cli,"-n","t-stroppy-live","events","run/"+c["run_id"],"--jq","."],stdout=subprocess.PIPE,stderr=subprocess.DEVNULL);sel=selectors.DefaultSelector();sel.register(proc.stdout,selectors.EVENT_READ);buf=b"";deadline=time.monotonic()+12
 while time.monotonic()<deadline:
  for key,_ in sel.select(.2):
   chunk=key.fileobj.read1(1048576)
   if chunk:buf+=chunk
 proc.terminate();proc.wait(timeout=5);events=[json.loads(line) for line in buf.splitlines() if line.strip()];(p/(name+"-events.json")).write_text(json.dumps(events));print(name,[{"kind":e["kind"],"subject":e.get("subject","")} for e in events if e.get("subject")][-10:])
