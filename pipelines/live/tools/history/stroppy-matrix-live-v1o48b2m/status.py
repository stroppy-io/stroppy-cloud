import json,subprocess,sys
rid=sys.argv[1]
def call(*args):
 return subprocess.check_output(["graphenectl","-n","t-stroppy-live",*args],text=True)
print(rid,call("get","run",rid,"--jq",".status").strip())
e=[json.loads(x) for x in call("events","run",rid,"--jq",".").splitlines() if x.startswith("{")]
pending={}
for x in e:
 k=x.get("kind","");a=x.get("activityId")
 if k=="activity-scheduled": pending[a]=x.get("subject")
 if k in ("activity-completed","activity-failed","activity-canceled","activity-timed-out"): pending.pop(a,None)
 if k=="activity-scheduled" and x.get("subject")=="stroppy.events.emit":
  v=x.get("input",{});print("milestone",v.get("name"),{k:v for k,v in v.get("payload",{}).items() if k in ("phase","machine","segment","container","error")})
 if "failed" in k or "timed-out" in k:
  print("failure event",x.get("eventId"),k,x.get("subject"))
  def failure(v):
   if isinstance(v,dict):
    if "message" in v and isinstance(v["message"],str):print(v["message"][:1000])
    for a in v.values():failure(a)
   elif isinstance(v,list):
    for a in v:failure(a)
  failure(x.get("raw",{}))
print("pending",pending)
