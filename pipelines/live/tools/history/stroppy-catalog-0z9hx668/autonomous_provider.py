import subprocess,json,datetime,pathlib,collections
p=pathlib.Path('/tmp/stroppy-catalog-0z9hx668/ydb-autonomous');s=json.loads((p/'state.json').read_text());prefixes=[x['uuid'][:8] for x in s['cells'].values()];k=['kubectl','--kubeconfig','/tmp/stroppy-mysql-vnafdcv2/kubeconfig','--context','stroppy-live-test'];now=datetime.datetime.now(datetime.timezone.utc).isoformat();out={'checked_at':now}
r=json.loads(subprocess.check_output(k+['-n','crossplane-system','get','pods','-o','json']));pods=[x for x in r['items'] if '6bcccfcfcd' in x['metadata']['name']];out['provider_pods']=[{'name':x['metadata']['name'],'uid':x['metadata']['uid'],'phase':x['status']['phase'],'reason':x['status'].get('reason'),'restarts':sum(c['restartCount'] for c in x['status'].get('containerStatuses',[]))} for x in pods]
r=json.loads(subprocess.check_output(k+['get','instances.compute.yandex-cloud.jet.crossplane.io','--show-managed-fields=true','-o','json']));rows=[x for x in r['items'] if any(z in x['metadata']['name'] for z in prefixes)];out['instances_declared']=len(rows);out['instances_ready']=sum(any(c['type']=='Ready' and c['status']=='True' for c in x.get('status',{}).get('conditions',[])) for x in rows);out['condition_failures']=[{'name':x['metadata']['name'],'condition':c} for x in rows for c in x.get('status',{}).get('conditions',[]) if c['type']=='Synced' and c['status']=='False'];out['resource_usage']=subprocess.check_output(k+['-n','crossplane-system','top','pod','provider-jet-yc-c40f8c9b9edd-6bcccfcfcd-wr6dw'],text=True)
with (p/'provider-observations.jsonl').open('a') as f:f.write(json.dumps(out)+'\n')
print(now,'ready',out['instances_ready'],'/',len(rows),'errors',len(out['condition_failures']),'pods',out['provider_pods']);print(out['resource_usage'])

proof_path=p/'automatic-provisioning.json';proof=json.loads(proof_path.read_text()) if proof_path.exists() else {'status':'running','run_id':s['run_id'],'expected_vm_count':39,'ready_instances':{},'runtime_commit':'28827f692b04e1f01b1bc4bed5f40926418f05bb'}
for x in rows:
 cond=x.get('status',{}).get('conditions',[])
 if not all(any(c['type']==typ and c['status']=='True' for c in cond) for typ in ['Ready','Synced']):continue
 ext=x['metadata'].get('annotations',{}).get('crossplane.io/external-name');assert ext
 owners=[f['manager'] for f in x['metadata'].get('managedFields',[]) if 'f:crossplane.io/external-name' in f.get('fieldsV1',{}).get('f:metadata',{}).get('f:annotations',{})];assert owners==['provider'],(x['metadata']['name'],owners)
 assert x.get('status',{}).get('atProvider',{}).get('id')==ext
 proof['ready_instances'][x['metadata']['name']]={'instance_id':ext,'resource_uid':x['metadata']['uid'],'run_id':x['spec']['forProvider']['labels']['stroppy-run'],'external_name_manager':'provider','ready_checked_at':now}
proof['checked_at']=now;proof['observed_ready_vm_count']=len(proof['ready_instances'])
if len(proof['ready_instances'])==39:proof['status']='passed'
proof_path.write_text(json.dumps(proof,indent=2)+'\n')
public=pathlib.Path('/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live/yc-autonomous-provisioning-check.json');public.write_text(json.dumps(proof,indent=2)+'\n')
