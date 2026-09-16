import subprocess,json,time,pathlib,datetime
out=pathlib.Path('/tmp/stroppy-pg-matrix-czazco7o');seen={}
for f in [out/'versions-instances.json',out/'retry/versions-instances.json']:
 if f.exists():seen.update(json.loads(f.read_text()))
for _ in range(160):
 try:
  v=json.loads(subprocess.check_output(['yc','compute','instance','list','--folder-id','b1ghttqg66t14ldkvfcq','--format','json'],timeout=20))
  active=[]
  for x in v:
   if x.get('name','').startswith('stroppy-stroppy-live-'):
    seen[x['id']]={k:x.get(k) for k in ['id','name','status','boot_disk','secondary_disks']};active.append(x['name'])
  (out/'all-instances.json').write_text(json.dumps(seen,indent=2))
  n=json.loads(subprocess.check_output(['yc','vpc','network','list','--folder-id','b1ghttqg66t14ldkvfcq','--format','json'],timeout=20))
  status={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'total_networks':len(n),'test_networks':[x['name'] for x in n if x.get('name','').startswith('stroppy-stroppy-live-')],'test_instances':active}
  (out/'cleanup-status.json').write_text(json.dumps(status,indent=2));print(status,flush=True)
 except Exception as e:print(type(e).__name__,str(e)[:200],flush=True)
 time.sleep(30)
