import json,pathlib,subprocess,datetime,re
work=pathlib.Path('/tmp/stroppy-patroni-017zezx1');seen={}
for name in ['instances.json','initial-instances.json','stream-failure-instances.json']:
 p=work/name
 if not p.exists():continue
 rows=json.loads(p.read_text())
 for x in rows.values() if isinstance(rows,dict) else rows:seen[x['id']]=x
recovered=json.loads((work/'recovered-boot-disks.json').read_text())
for r in recovered:
 x=seen[r['instance_id']]
 if (x.get('boot_disk') or {}).get('disk_id'):assert x['boot_disk']['disk_id']==r['disk_id']
 else:x['boot_disk']={'disk_id':r['disk_id']}
ids=set(seen);disks=set()
for x in seen.values():
 for d in [x.get('boot_disk')]+(x.get('secondary_disks') or []):
  if d and d.get('disk_id'):disks.add(d['disk_id'])
boot_ids={x['boot_disk']['disk_id'] for x in seen.values() if x.get('boot_disk',{}).get('disk_id')}
assert len(boot_ids)==len(seen),(len(boot_ids),len(seen))
data_names=sorted(x['name']+'-data' for x in seen.values() if re.search(r'-(db(?:-replica)?|etcd)-[0-9]+$',x['name']))
assert len(seen)==48 and len(data_names)==36,(len(seen),len(data_names))
remaining={};counts={}
for a,b in [('compute','instance'),('compute','disk'),('vpc','network'),('vpc','subnet'),('vpc','security-group')]:
 rows=json.loads(subprocess.check_output(['yc',a,b,'list','--folder-id','b1ghttqg66t14ldkvfcq','--format','json'],timeout=30))
 counts[b]=len(rows);remaining[b]=[{k:x.get(k) for k in ['id','name','status']} for x in rows if x.get('name','').startswith('stroppy-stroppy-live-') or (b=='instance' and x['id'] in ids) or (b=='disk' and x['id'] in disks)]
 print(b,'remaining',len(remaining[b]),flush=True)
result={'checked_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'folder_id':'b1ghttqg66t14ldkvfcq','remaining_test_resources':remaining,'observed_instance_ids':sorted(ids),'observed_boot_and_data_disk_ids':sorted(disks),'boot_disk_ids':sorted(boot_ids),'recovered_boot_disks_from_persisted_metrics':recovered,'expected_data_disk_names':data_names,'disk_verification':'48 boot disk IDs plus 36 expected named data disks; resource-name prefix and observed IDs are checked against the complete YC disk list','folder_resource_counts_including_unrelated_resources':counts}
pathlib.Path('pipelines/live/patroni.cleanup.json').write_text(json.dumps(result,indent=2)+'\n')
assert not any(remaining.values()),'test resources remain'
