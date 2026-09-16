import json,subprocess,time
from pathlib import Path
P=Path(__file__).parent
state=json.loads((P/'state.json').read_text())
CLI='/tmp/stroppy-patroni-017zezx1/release-029/graphenectl'
prefixes=[v['uuid'][:8] for v in state['cells'].values()]
ledger={}
while True:
 result=subprocess.run([CLI,'-n','t-stroppy-live','run','list','--jq','.'],capture_output=True,text=True)
 if result.returncode:
  print('run list temporarily failed',flush=True);time.sleep(20);continue
 runs=[r for r in json.loads(result.stdout).get('resources',[]) if state['run_id'] in r['ref']]
 for kind,args in {'vm':['compute','instance'],'disk':['compute','disk'],'network':['vpc','network'],'subnet':['vpc','subnet'],'sg':['vpc','security-group']}.items():
  r=subprocess.run(['yc',*args,'list','--folder-id','b1ghttqg66t14ldkvfcq','--format','json'],capture_output=True,text=True)
  if r.returncode:continue
  allrows=json.loads(r.stdout)
  rows=[x for x in allrows if any(v in x.get('name','') for v in prefixes)]
  ids=ledger.setdefault(kind,{})
  for row in rows:
   item=ids.setdefault(row['id'],{'id':row['id'],'name':row['name']})
   if kind=='vm':
    boot=row.get('boot_disk',{}).get('disk_id')
    if boot:item['boot_disk_id']=boot
    attached=set(item.get('secondary_disk_ids',[]))
    attached.update(x['disk_id'] for x in row.get('secondary_disks',[]) if x.get('disk_id'))
    item['secondary_disk_ids']=sorted(attached)
  (P/(kind+'-current.json')).write_text(json.dumps(rows))
 (P/'ledger.json').write_text(json.dumps(ledger,indent=2))
 (P/'runs.json').write_text(json.dumps(runs,indent=2))
 print(time.strftime('%H:%M:%S'),[(r['ref'],r['phase']) for r in runs], 'VMs',len(json.loads((P/'vm-current.json').read_text())),flush=True)
 if len(runs)==3 and all(r['phase'] in ['Completed','Failed','Canceled'] for r in runs):
  if all(not json.loads((P/(k+'-current.json')).read_text()) for k in ledger):break
 time.sleep(20)
