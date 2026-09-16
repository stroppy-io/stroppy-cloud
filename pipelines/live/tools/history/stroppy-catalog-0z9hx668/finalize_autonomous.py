import json, subprocess, concurrent.futures
from pathlib import Path
from datetime import datetime, timezone
B=Path('/tmp/stroppy-catalog-0z9hx668'); D=B/'ydb-autonomous'; P=Path('/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live')
read=lambda p:json.loads(p.read_text())
def write(p,x):p.write_text(json.dumps(x,indent=2,ensure_ascii=False)+'\n')
state=read(D/'state.json'); runs=read(D/'runs.json')
assert len(runs)==4 and all(r['phase']=='Completed' for r in runs), [(r['ref'],r['phase']) for r in runs]
assert not list(D.glob('*.check-error.json'))
assert all(read(D/(k+'-current.json'))==[] for k in ['vm','disk','network','subnet','sg','ydb'])
verified={}
for n,c in state['cells'].items():
 v=read(D/(n+'.verified.json')); assert v['status']=='passed' and v['run_id']==c['run_id']
 result=read(D/(n+'.result.json')); partial=read(D/(n+'.partial-result.json'))
 assert result['segments']==partial['segments'] and result['artifacts']==partial['artifacts']
 assert all(s['status']=='completed' for s in result['segments'])
 verified[n]=v
commands={'vm':['compute','instance'],'disk':['compute','disk'],'network':['vpc','network'],'subnet':['vpc','subnet'],'sg':['vpc','security-group']}
def fetch(k):
 data=json.loads(subprocess.check_output(['rtk','proxy','yc',*commands[k],'list','--folder-id','b1ghttqg66t14ldkvfcq','--format','json'],text=True))
 write(D/('final-all-'+k+'.json'),data); return k,data
with concurrent.futures.ThreadPoolExecutor(max_workers=5) as ex: current=dict(ex.map(fetch,commands))
now=datetime.now(timezone.utc).isoformat()
baseline=read(B/'baseline.json'); preserved=read(P/'preserved-resources-check.json')
allowed={'vm':{'fv4115p6tfv785o1l6rp'},'disk':{'fv43bgfo5i1pgbbjnspn'}}
checks={}; ledger=read(D/'ledger.json'); cleanup={}
for k,values in current.items():
 ids={v['id'] for v in values}; orig={v['id'] for v in baseline[k]}
 missing=orig-ids; unexpected=missing-allowed.get(k,set()); assert not unexpected,(k,unexpected)
 checks[k]={'baseline_count':len(orig),'current_count':len(ids),'missing_baseline_ids':sorted(missing),'unexpected_missing_ids':[]}
 remaining=set(ledger[k])&ids; assert not remaining,(k,remaining)
 cleanup[k]={'recorded_created':len(ledger[k]),'remaining_ids':[]}
boots={v['boot_disk_id'] for v in ledger['vm'].values()}; assert len(boots)==39
assert not boots&{v['id'] for v in current['disk']}
cleanup['boot_disks']={'recorded_created':len(boots),'remaining_ids':[]}
preserved.update(checked_at=now,checks=checks,scope='Baseline identities preserved except the previously verified cluster-autoscaler removal of one empty managed Kubernetes node and its boot disk. Final snapshot after test cleanup; full configuration equality is not asserted.')
write(P/'preserved-resources-check.json',preserved)
write(P/'ydb-autonomous-cleanup-check.json',{'status':'passed','checked_at':now,'run_id':state['run_id'],'source':'Fresh YC folder lists intersected with campaign resource ledger, including all 39 boot disk IDs','resources':cleanup,'baseline_evidence':'preserved-resources-check.json'})
w=read(P/'ydb-autonomous-workload-check.json')
w.update(status='passed',checked_at=now,scope='All three fresh workflows Completed. Native measurements, component scalar metrics, logs, traces, artifacts and member probes verified; owned cloud resources including boot disks absent. Database histogram/summary and long baseline/fault campaigns remain outside this acceptance.',cleanup_evidence='ydb-autonomous-cleanup-check.json',result_source='Canonical completed workflow results; segments and artifact references exactly match early activity results')
for entry in w['runs']:
 n=entry['verification_prefix']; entry['native']=verified[n]['native']; entry['verification']='passed'; entry['workflow_phase']='Completed'
write(P/'ydb-autonomous-workload-check.json',w)
observations=[json.loads(l) for l in (D/'provider-observations.jsonl').read_text().splitlines() if l]
latest=observations[-1]; assert latest['provider_pods'][0]['restarts']==0
m=read(P/'yc-provider-memory-check.json'); m['autonomous_verification']={'status':'passed','run_id':state['run_id'],'checked_at':now,'provisioning_evidence':'yc-autonomous-provisioning-check.json','workload_evidence':'ydb-autonomous-workload-check.json','cleanup_evidence':'ydb-autonomous-cleanup-check.json','last_provider_observation':latest}
write(P/'yc-provider-memory-check.json',m)
print(json.dumps({'status':'passed','runs':len(verified),'cleanup':cleanup,'baseline_counts':{k:len(v) for k,v in current.items()}},indent=2))
