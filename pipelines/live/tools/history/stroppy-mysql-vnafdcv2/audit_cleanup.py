import json,subprocess,datetime
from pathlib import Path
p=Path(__file__).parent;live=Path('/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live')
baseline=json.loads((p/'baseline.json').read_text());created={k:set() for k in baseline};run_ids=[]
for directory in [p,p/'expired-token',p/'maria',p/'remaining',p/'fixed-mysql',p/'fixed-mariadb',p/'semisync',p/'semisync2',p/'family211',p/'single-fixed']:
 ledger=json.loads((directory/'ledger.json').read_text());state=json.loads((directory/'state.json').read_text());run_ids.append(state['run_id'])
 for k,items in ledger.items():created[k].update(items)
 for vm in ledger.get('vm',{}).values():
  if vm.get('boot_disk_id'):created['disk'].add(vm['boot_disk_id'])
  created['disk'].update(vm.get('secondary_disk_ids',[]))
resources={};ok=True
for k,args in {'vm':['compute','instance'],'disk':['compute','disk'],'network':['vpc','network'],'subnet':['vpc','subnet'],'sg':['vpc','security-group']}.items():
 rows=json.loads(subprocess.check_output(['yc',*args,'list','--folder-id','b1ghttqg66t14ldkvfcq','--format','json'],text=True));ids={x['id'] for x in rows};prior={x['id'] for x in baseline[k]}
 remaining=ids & created[k];missing=prior-ids;unrelated_new=ids-prior-created[k];assert not prior & created[k]
 ok &= not remaining and not missing and not unrelated_new
 resources[k]={'baseline_count':len(prior),'current_count':len(ids),'created_count':len(created[k]),'created_ids':sorted(created[k]),'remaining_created_ids':sorted(remaining),'missing_baseline_ids':sorted(missing),'unrelated_new_ids':sorted(unrelated_new)}
cli='/tmp/stroppy-native-0izo0l0o/release-0210/graphenectl';rows=json.loads(subprocess.check_output([cli,'--config','/tmp/stroppy-native-0izo0l0o/config/graphene/config.yaml','--context','native-live','-n','t-stroppy-live','run','list','--jq','.'],text=True))['resources']
runs=[{'ref':r['ref'],'phase':r['phase']} for r in rows if any(r['ref']=='run/'+rid or r['ref'].startswith('run/'+rid+'-') for rid in run_ids)]
terminal=all(r['phase'] in ['Completed','Failed','Canceled'] for r in runs);ok &= terminal
body={'status':'passed' if ok else 'in_progress','checked_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'folder_id':'b1ghttqg66t14ldkvfcq','source':'fresh YC resource lists compared with pre-stage IDs and per-attempt ledgers; Graphene run phases','resources':resources,'runs':runs}
(live/'mysql-family-cleanup.json').write_text(json.dumps(body,indent=2)+'\n');print(body['status'],{k:(v['current_count'],len(v['remaining_created_ids'])) for k,v in resources.items()})
