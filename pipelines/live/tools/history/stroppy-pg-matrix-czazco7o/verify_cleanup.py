import json,pathlib,subprocess,datetime
work=pathlib.Path('/tmp/stroppy-pg-matrix-czazco7o');seen={}
for name in ['instances.json','versions-instances.json','retry/versions-instances.json','final/versions-instances.json','all-instances.json']:
 p=work/name
 if not p.exists():continue
 rows=json.loads(p.read_text())
 for x in rows.values() if isinstance(rows,dict) else rows:seen[x['id']]=x
ids=set(seen);disks=set()
for x in seen.values():
 for d in [x.get('boot_disk')]+(x.get('secondary_disks') or []):
  if d and d.get('disk_id'):disks.add(d['disk_id'])
remaining={};counts={}
for a,b in [('compute','instance'),('compute','disk'),('vpc','network'),('vpc','subnet'),('vpc','security-group')]:
 rows=json.loads(subprocess.check_output(['yc',a,b,'list','--folder-id','b1ghttqg66t14ldkvfcq','--format','json'],timeout=30))
 counts[b]=len(rows);remaining[b]=[{k:x.get(k) for k in ['id','name','status']} for x in rows if x.get('name','').startswith('stroppy-stroppy-live-') or (b=='instance' and x['id'] in ids) or (b=='disk' and x['id'] in disks)]
 print(b,'remaining',len(remaining[b]),flush=True)
result={'checked_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'folder_id':'b1ghttqg66t14ldkvfcq','remaining_test_resources':remaining,'observed_instance_ids':sorted(ids),'observed_boot_and_data_disk_ids':sorted(disks),'folder_resource_counts_including_unrelated_resources':counts}
pathlib.Path('pipelines/live/postgres-matrix.cleanup.json').write_text(json.dumps(result,indent=2)+'\n')
assert not any(remaining.values()),'test resources remain'
