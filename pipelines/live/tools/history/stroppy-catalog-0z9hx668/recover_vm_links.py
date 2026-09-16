import json,subprocess,hashlib,datetime,sys
from pathlib import Path
p=Path('/tmp/stroppy-catalog-0z9hx668'); d=p/'ydb-parallel'; rows=json.loads((d/'vm-adoption-plan.json').read_text());baseline={x['id'] for x in json.loads((p/'baseline.json').read_text())['vm']}
k=['kubectl','--kubeconfig','/tmp/stroppy-mysql-vnafdcv2/kubeconfig','--context','stroppy-live-test'];kind='instances.compute.yandex-cloud.jet.crossplane.io';proof=[]
def desired(spec):
 f=json.loads(json.dumps(spec))
 for d in f.get('secondaryDisk',[]):
  if 'diskIdRef' in d:d.pop('diskId',None)
 for n in f.get('networkInterface',[]):
  if 'subnetIdRef' in n:n.pop('subnetId',None)
  if 'securityGroupIdsRefs' in n:n.pop('securityGroupIds',None)
 return f
for row in rows:
 x=json.loads(subprocess.check_output(k+['get',kind,row['resource'],'-o','json']));assert x['metadata']['uid']==row['uid'];assert not x['metadata'].get('deletionTimestamp');f=x['spec']['forProvider']
 def external(kind,ref):
  obj=json.loads(subprocess.check_output(k+['get',kind,ref,'-o','json']));value=obj['metadata'].get('annotations',{}).get('crossplane.io/external-name');assert value;return value
 for disk in f.get('secondaryDisk',[]):
  disk.setdefault('diskId',external('disks.compute.yandex-cloud.jet.crossplane.io',disk['diskIdRef']['name']))
 for nic in f['networkInterface']:
  nic.setdefault('subnetId',external('subnets.vpc.yandex-cloud.jet.crossplane.io',nic['subnetIdRef']['name']))
  nic.setdefault('securityGroupIds',[external('securitygroups.vpc.yandex-cloud.jet.crossplane.io',a['name']) for a in nic['securityGroupIdsRefs']])
 z=json.loads(subprocess.check_output(['yc','compute','instance','get',row['instance_id'],'--full','--format','json']));assert z['id'] not in baseline
 assert x['metadata']['name']==z['name']==f['name'];assert z['labels']['stroppy-run']==row['run_id'];assert all(z['labels'].get(a)==b for a,b in f['labels'].items())
 assert z['metadata']==f['metadata'];assert z['folder_id']==f['folderId'] and z['zone_id']==f['zone'] and z['platform_id']==f['platformId'];assert z['fqdn']==f['hostname']+'.ru-central1.internal'
 assert int(z['resources']['cores'])==f['resources'][0]['cores'] and int(z['resources']['memory'])==f['resources'][0]['memory']*1024**3
 assert {a['disk_id'] for a in z.get('secondary_disks',[])}=={a['diskId'] for a in f.get('secondaryDisk',[])}
 assert len(z['network_interfaces'])==len(f['networkInterface'])==1
 assert z['network_interfaces'][0]['subnet_id']==f['networkInterface'][0]['subnetId'];assert set(z['network_interfaces'][0]['security_group_ids'])==set(f['networkInterface'][0]['securityGroupIds'])
 disk=json.loads(subprocess.check_output(['yc','compute','disk','get',z['boot_disk']['disk_id'],'--format','json']));expected=f['bootDisk'][0]['initializeParams'][0]
 assert disk['source_image_id']==expected['imageId'] and int(disk['size'])==expected['size']*1024**3 and disk['type_id']==expected['type'];assert z['boot_disk']['auto_delete']==f['bootDisk'][0]['autoDelete']
 current=x['metadata'].get('annotations',{}).get('crossplane.io/external-name');assert current in [None,'',z['id']]
 row['metadata_sha256']=hashlib.sha256(json.dumps(z['metadata'],sort_keys=True).encode()).hexdigest();row['matched']+=['full metadata','boot image/size/type/autodelete','baseline excluded'];row['checked_at']=datetime.datetime.now(datetime.timezone.utc).isoformat()
 if '--apply' in sys.argv and not current:
  for attempt in range(10):
   fresh=json.loads(subprocess.check_output(k+['get',kind,row['resource'],'-o','json']));assert fresh['metadata']['uid']==x['metadata']['uid'];assert not fresh['metadata'].get('deletionTimestamp');assert desired(fresh['spec']['forProvider'])==desired(f)
   existing=fresh['metadata'].get('annotations',{}).get('crossplane.io/external-name');assert existing in [None,'',z['id']]
   if existing==z['id']:row['applied']=True;break
   ops=[{'op':'test','path':'/metadata/uid','value':fresh['metadata']['uid']},{'op':'test','path':'/metadata/resourceVersion','value':fresh['metadata']['resourceVersion']},{'op':'add','path':'/metadata/annotations/crossplane.io~1external-name','value':z['id']}]
   result=subprocess.run(k+['patch',kind,row['resource'],'--type=json','-p',json.dumps(ops)],capture_output=True,text=True)
   if result.returncode==0:row['applied']=True;break
  else:raise RuntimeError('Optimistic patch did not succeed for '+row['resource'])
 else:row['applied']=bool(current)
 proof.append(row);(d/'vm-adoption-check.json').write_text(json.dumps(proof,indent=2)+'\n');print(row['resource'],'matched','linked' if row['applied'] else 'dry-run',flush=True)
 if '--first' in sys.argv:break
