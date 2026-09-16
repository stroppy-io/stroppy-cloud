import json,subprocess,datetime,itertools
from pathlib import Path
B=Path('/tmp/stroppy-catalog-0z9hx668');L=Path('/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live');CLI=str(B/'bin/graphenectl')
folders=['pg-soak-5h','pg-native-final']
suites={f:json.loads((B/f/'suite.json').read_text()) for f in folders}
allruns=json.loads(subprocess.check_output([CLI,'-n','t-stroppy-live','run','list','--jq','.'],text=True,timeout=30))['resources']
active=[r for r in allruns if r['phase'] in ['Running','Queued']]
for r in active:
 assert r['ref']=='run/matrix-oriole-0d49e1e2', 'Other live runs must be accounted for: '+r['ref']
 events=[json.loads(l)['raw'] for l in subprocess.check_output([CLI,'-n','t-stroppy-live','events','run','matrix-oriole-0d49e1e2','--jq','.'],text=True,timeout=30).splitlines()]
 assert len(events)==3 and events[-1]['eventType']=='EVENT_TYPE_WORKFLOW_EXECUTION_CANCEL_REQUESTED'
 assert not any('activityTaskScheduledEventAttributes' in e for e in events)
 assert not any(x.get('owner')==r['ref'] for x in allruns)
(L/'overnight-preflight-check.json').write_text(json.dumps({'status':'passed','ignored_nonexecuting_legacy_run':'matrix-oriole-0d49e1e2' if active else None,'reason':'Only workflow start/task scheduling/cancel request, no activity or child run ever started; authoritative quota usage still included'},indent=2)+'\n')
long=suites['pg-soak-5h']['cells'][0]['run_spec']
shorts=[c['run_spec'] for c in suites['pg-native-final']['cells']]
def requirements(spec):
 ms=spec['machines'];return {'compute.instances.count':len(ms),'compute.instanceCores.count':sum(m['cpu'] for m in ms),'compute.instanceMemory.size':sum(m['memory_gb'] for m in ms)*1024**3,'compute.disks.count':len(ms)+sum(len(m.get('disks',[])) for m in ms),'compute.ssdDisks.size':(40*len(ms)+sum(d['gb'] for m in ms for d in m.get('disks',[])))*1024**3,'vpc.networks.count':1,'vpc.subnets.count':len({m['location'] for m in ms}),'vpc.securityGroups.count':1,'vpc.externalAddresses.count':len(ms)}
base=requirements(long);combinations=[(requirements(a),requirements(b)) for a,b in itertools.combinations(shorts,2)]
reserved={k:base[k]+max(a[k]+b[k] for a,b in combinations) for k in base};proof={}
for service in ['compute','vpc']:
 q=json.loads(subprocess.check_output(['rtk','proxy','yc','quota-manager','quota-limit','list','--resource-id','b1gt6m4l9gfhaobcgb81','--resource-type','resource-manager.cloud','--service',service,'--format','json'],text=True,timeout=30))
 for x in q['quota_limits']:
  k=x['quota_id']
  if k in reserved:
   proof[k]={'usage':x['usage'],'reserved':reserved[k],'projected':x['usage']+reserved[k],'limit':x['limit']};assert proof[k]['projected']<=x['limit'],proof[k]
assert set(proof)==set(reserved)
now=datetime.datetime.now(datetime.timezone.utc)
credential=json.loads((L/'overnight-credential-check.json').read_text());assert datetime.datetime.fromisoformat(credential['expires_at'])-now>datetime.timedelta(hours=7)
for name,tag in [('stroppy-run','f53623d265c372b2'),('stroppy-suite','81bba44e13ff1ef7')]:
 image=json.loads(subprocess.check_output([CLI,'-n','t-stroppy-live','get','pipeline/'+name,'--jq','.resource.state.image'],text=True));assert image.endswith(':'+tag)
(L/'overnight-quota-check.json').write_text(json.dumps({'status':'passed','checked_at':now.isoformat(),'concurrent_stands':3,'scope':'One five-hour Patroni stand plus any two short stands, including cleanup','reservations':proof},indent=2)+'\n')
for f in folders:
 d=B/f;s=json.loads((d/'state.json').read_text());assert 'started_at' not in s
 s['started_at']=datetime.datetime.now(datetime.timezone.utc).isoformat();s['pipeline_version']='f53623d265c372b2';(d/'state.json').write_text(json.dumps(s,indent=2)+'\n')
 r=subprocess.run([CLI,'-n','t-stroppy-live','run','start','stroppy-suite','--run-id',s['run_id'],'--params-file',str(d/'suite.json'),'--jq','.'],capture_output=True,text=True,timeout=60)
 (d/'start.stdout').write_text(r.stdout);(d/'start.stderr').write_text(r.stderr);r.check_returncode();print('started',s['run_id'],flush=True)
