import json,subprocess,datetime
from pathlib import Path
P=Path(__file__).parent
baseline=json.loads((P/'baseline.json').read_text())
ledgers=[json.loads((folder/'ledger.json').read_text()) for folder in [P,P/'retry',P/'final']]
resources={}
for kind,args in {'vm':['compute','instance'],'disk':['compute','disk'],'network':['vpc','network'],'subnet':['vpc','subnet'],'sg':['vpc','security-group']}.items():
 current=json.loads(subprocess.check_output(['yc',*args,'list','--folder-id','b1ghttqg66t14ldkvfcq','--format','json'],text=True,timeout=45))
 ids={row['id'] for row in current};base={row['id'] for row in baseline[kind]};owned=set()
 for ledger in ledgers:
  owned.update(ledger[kind])
  if kind=='disk':
   for vm in ledger['vm'].values():
    if vm.get('boot_disk_id'):owned.add(vm['boot_disk_id'])
    owned.update(vm.get('secondary_disk_ids',[]))
 assert not owned & ids,(kind,'leaked',owned & ids)
 assert base <= ids,(kind,'baseline missing',base-ids)
 resources[kind]={'deleted_ids':sorted(owned),'baseline_preserved':len(base),'current_count':len(ids),'unrelated_new_ids':sorted(ids-base)}
cli=str(P/'bin/graphenectl')
runs=json.loads(subprocess.check_output([cli,'-n','t-stroppy-live','run','list','--jq','.'],text=True,timeout=30))['resources']
suite_ids=[json.loads((folder/'state.json').read_text())['run_id'] for folder in [P,P/'retry',P/'final']]
ours=[r for r in runs if any(r['ref'].startswith('run/'+sid) for sid in suite_ids)]
assert ours and all(r['phase'] in ['Completed','Canceled','Failed'] for r in ours)
deployments=json.loads(subprocess.check_output(['kubectl','--kubeconfig','/tmp/stroppy-mysql-vnafdcv2/kubeconfig','--context','stroppy-live-test','-n','graphene','get','deployments','-o','json'],text=True,timeout=30))['items']
workers=[r['metadata']['name'] for r in deployments if any(sid in json.dumps(r['metadata'].get('labels',{})) for sid in suite_ids)]
assert not workers,workers
report={'status':'passed','checked_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'scope':'exact infrastructure IDs of all three cluster attempts, including the MariaDB single regression reruns, including VM boot and secondary disks','resources':resources,'runs':[{'ref':r['ref'],'phase':r['phase']} for r in ours],'managed_workers_remaining':workers}
target=Path('/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live/mysql-clusters-cleanup.json');target.write_text(json.dumps(report,indent=2)+'\n')
print({k:len(v['deleted_ids']) for k,v in resources.items()})
