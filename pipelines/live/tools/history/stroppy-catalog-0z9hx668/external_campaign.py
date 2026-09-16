import json,subprocess,time,datetime,ipaddress,copy,sys,os
from pathlib import Path
P=Path('/tmp/stroppy-catalog-0z9hx668');D=P/'external-live';LIVE=Path('/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live');CLI=str(P/'bin/graphenectl');K=['kubectl','--kubeconfig','/tmp/stroppy-mysql-vnafdcv2/kubeconfig','--context','stroppy-live-test'];NS='t-stroppy-live';FOLDER='b1ghttqg66t14ldkvfcq';S=json.loads((D/'state.json').read_text());TYPES={'vm':['compute','instance'],'disk':['compute','disk'],'network':['vpc','network'],'subnet':['vpc','subnet'],'sg':['vpc','security-group'],'ydb':['ydb','database']}
def write(p,v):
 t=p.with_suffix(p.suffix+'.tmp');t.write_text(json.dumps(v,indent=2)+'\n');os.replace(t,p)
def cmd(args,timeout=60):
 r=subprocess.run(args,capture_output=True,text=True,timeout=timeout)
 if r.returncode:
  write(D/'last-command-error.json',{'command':args[:4],'stderr':r.stderr[-3000:]});raise RuntimeError('Command failed; private diagnostic recorded')
 return r.stdout
def gql(*args):return json.loads(cmd([CLI,'-n',NS,*args,'--jq','.']))
def rows(kind):return json.loads(cmd(['yc',*TYPES[kind],'list','--folder-id',FOLDER,'--format','json']))
def owned(kind,uid):return [x for x in rows(kind) if x.get('name','').startswith('stroppy-stroppy-live-'+uid[:8]+'-')]
def capacity(extra):
 q=json.loads(cmd(['yc','quota-manager','quota-limit','list','--resource-id','b1gt6m4l9gfhaobcgb81','--resource-type','resource-manager.cloud','--service','vpc','--format','json']))
 net=next(x for x in q['quota_limits'] if x['quota_id']=='vpc.networks.count')
 return net['usage']+extra<=min(net['limit'],6)
def phase(rid):
 return next((r['phase'] for r in gql('run','list')['resources'] if r['ref']=='run/'+rid),None)
def wait_until(fn,seconds,label):
 deadline=time.monotonic()+seconds
 while time.monotonic()<deadline:
  value=fn()
  if value:return value
  print(time.strftime('%H:%M:%S'),label,flush=True);time.sleep(20)
 raise RuntimeError('Timed out: '+label)
def completed(rid):
 state=phase(rid)
 if state in ['Failed','Canceled']:raise RuntimeError(rid+' '+state)
 return state=='Completed'
def ensure_clean(uid):
 a={k:owned(k,uid) for k in TYPES};write(D/(uid[:8]+'.remaining.json'),a)
 return all(not x for x in a.values())
def probe(stage):
 target=D/(stage+'.canary.json');cmd(['python3',str(P/'probe_remote_external.py'),S['fixture_uuid'][:8]+'-db-1',str(target)],timeout=90);return json.loads(target.read_text())
fixture_started=False;external_started=False
try:
 assert S.get('not_started'), 'Refusing to start an existing campaign twice'
 # Start the one-VM fixture alongside the final Cockroach cell, then consume
 # that queue's released network for the external runner. No fourth stand.
 wait_until(lambda:phase('matrix-cockroach-74ee56ed-cockroach-three-node-26.3') in ['Running','Completed'] and capacity(1),5400,'waiting for final Cockroach cell and fixture quota')
 S['started_at']=datetime.datetime.now(datetime.timezone.utc).isoformat();S['not_started']=False;write(D/'state.json',S)
 fixture_started=True
 cmd([CLI,'-n',NS,'run','start','stroppy-run','--run-id',S['fixture_run_id'],'--params-file',str(D/'fixture.json'),'--jq','.']);fixture_started=True;print('started owned external fixture',S['fixture_run_id'],flush=True)
 wait_until(lambda:completed(S['fixture_run_id']),1500,'fixture provisioning and canary workload')
 fr=gql('run','result',S['fixture_run_id']);write(D/'fixture.result.json',fr);assert len(fr['segments'])==1 and fr['segments'][0]['status']=='completed' and fr['summary'].get('errors',0)==0
 before=probe('before');fixture_vm=owned('vm',S['fixture_uuid']);assert len(fixture_vm)==1 and fixture_vm[0]['status']=='RUNNING';fvm=fixture_vm[0];ip=fvm['network_interfaces'][0]['primary_v4_address']['one_to_one_nat']['address'];ipaddress.ip_address(ip)
 fixture_ids={k:[x['id'] for x in owned(k,S['fixture_uuid'])] for k in TYPES};write(D/'fixture-resources-before.json',fixture_ids)
 wait_until(lambda:completed('matrix-cockroach-74ee56ed') and all(not json.loads((P/'crdb-matrix'/(k+'-current.json')).read_text()) for k in TYPES) and capacity(1),1800,'waiting for Cockroach cleanup and external runner quota')
 r=json.loads((P/'external-protocol-inputs/pg/external-dsn-external-execute_sql.json').read_text());f=json.loads((D/'fixture.json').read_text());password=json.loads((D/'fixture-reader.json').read_text())['password'];database=next(c['env']['POSTGRES_DB'] for c in f['containers'] if c['name']=='db-1-postgres');r['run_id']=S['external_uuid'];r['provider']=f['provider'];r['observability']=f['observability'];r['workload']['stroppy_image']=f['workload']['stroppy_image'];r['workload']['url']='postgres://external_reader:'+password+'@'+ip+':5432/'+database+'?sslmode=disable';r['workload']['segments']=copy.deepcopy(f['workload']['segments']);r['workload']['segments'][0]['name']='external-canary';r['workload']['segments'][0]['run']['duration']='2m';r['host_prep']=[{'role':'runner','kind':'script','content':'#!/bin/sh\nset -eu\npython3 - <<\'WAIT\'\nimport socket,time\nfor attempt in range(100):\n try:\n  with socket.create_connection(('+repr(ip)+',5432),timeout=2):pass\n  break\n except OSError:time.sleep(3)\nelse:raise SystemExit("fixture ingress did not become reachable")\nWAIT\n'}]
 assert len(r['machines'])==1 and all(c['role']=='metrics' or c['name'].endswith('-node-exporter') for c in r['containers'])
 public=copy.deepcopy(r);public['observability']['otlp_headers']='';public['workload']['url']='postgres://external_reader:FIXTURE_PASSWORD@FIXTURE_HOST:5432/'+database+'?sslmode=disable';public.pop('host_prep',None);write(LIVE/'external-dsn-input.json',public)
 write(D/'external.json',r);cmd(['go','run',str(P/'validate_input.go'),str(D/'external.json')],timeout=120)
 now=datetime.datetime.now(datetime.timezone.utc).isoformat();campaign={'run_id':S['external_run_id'],'started_at':now,'public_input':'external-dsn-input.json','cells':{'external-dsn':{'database':'external','version':'external','topology':'dsn','uuid':S['external_uuid'],'run_id':S['external_run_id']}}};C=D/'runner';C.mkdir(mode=0o700,exist_ok=True);write(C/'state.json',campaign);write(C/'suite.json',{'cells':[{'id':'external-dsn','run_spec':r}]})
 external_started=True
 cmd([CLI,'-n',NS,'run','start','stroppy-run','--run-id',S['external_run_id'],'--params-file',str(D/'external.json'),'--jq','.']);external_started=True;print('started external-only runner',S['external_run_id'],flush=True)
 vm=wait_until(lambda:next((x for x in owned('vm',S['external_uuid']) if x.get('network_interfaces',[{}])[0].get('primary_v4_address',{}).get('one_to_one_nat',{}).get('address')),None),600,'external runner public address')
 runner_ip=vm['network_interfaces'][0]['primary_v4_address']['one_to_one_nat']['address'];ipaddress.ip_address(runner_ip);sg_name='stroppy-stroppy-live-'+S['fixture_uuid'][:8]+'-sg';sg=json.loads(cmd(K+['get','securitygroups.vpc.yandex-cloud.jet.crossplane.io',sg_name,'-o','json']));assert sg['metadata']['name']==sg_name and not sg['metadata'].get('deletionTimestamp')
 rule={'description':'owned external runner '+S['external_uuid'][:8],'protocol':'TCP','port':5432,'v4CidrBlocks':[runner_ip+'/32']};patch=[{'op':'test','path':'/metadata/resourceVersion','value':sg['metadata']['resourceVersion']},{'op':'add','path':'/spec/forProvider/ingress/-','value':rule}];write(D/'fixture-sg-patch.json',patch);cmd(K+['patch','securitygroups.vpc.yandex-cloud.jet.crossplane.io',sg_name,'--type=json','--patch-file',str(D/'fixture-sg-patch.json')]);write(D/'ingress-proof.json',{'fixture_sg':sg_name,'source':runner_ip+'/32','port':5432,'protocol':'TCP','runner_vm':vm['id']})
 wait_until(lambda:completed(S['external_run_id']),1800,'external workload and automatic runner cleanup');rr=gql('run','result',S['external_run_id']);write(C/'external-dsn.result.json',rr);assert len(rr['segments'])==1 and rr['segments'][0]['status']=='completed' and rr['summary'].get('errors',0)==0
 wait_until(lambda:ensure_clean(S['external_uuid']),300,'external resources removed')
 for k in TYPES:write(C/(k+'-current.json'),owned(k,S['external_uuid']))
 after_ids={k:[x['id'] for x in owned(k,S['fixture_uuid'])] for k in TYPES};assert fixture_ids==after_ids,'External cleanup changed fixture resources';after=probe('after')
 write(LIVE/'external-dsn-lifecycle-check.json',{'status':'passed','fixture_run_id':S['fixture_run_id'],'run_id':S['external_run_id'],'checked_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'external_machines':1,'external_database_containers':0,'before':before,'after_runner_cleanup':after,'fixture_resources_unchanged':True,'runner_cleanup':'all owned YC resource types absent','fixture_cleanup':'pending','telemetry_verification':'pending','scope':'Owned PostgreSQL fixture, read-only external workload, /32 ingress and independent data-preservation probe; no user databases accessed'})
 # Accept telemetry independently; the lifecycle proof alone is not a matrix pass.
 cmd(['python3',str(P/'verify_database.py'),str(C),'external-dsn','external-dsn'],timeout=900)
 print('external runner telemetry and lifecycle verified',flush=True)
finally:
 # Normal lifecycle cleanup only, exact owned root. Never force-delete finalizers.
 if external_started and phase(S['external_run_id'])=='Running':
  cmd([CLI,'-n',NS,'run','cancel',S['external_run_id']]);wait_until(lambda:phase(S['external_run_id'])!='Running',1800,'cancelled external runner cleanup')
 if fixture_started:
  if phase(S['fixture_run_id'])=='Running':
   cmd([CLI,'-n',NS,'run','cancel',S['fixture_run_id']]);wait_until(lambda:phase(S['fixture_run_id'])!='Running',1800,'cancelled fixture cleanup')
  net='stroppy-stroppy-live-'+S['fixture_uuid'][:8]+'-net'
  if owned('network',S['fixture_uuid']):
   root=gql('get','k8s.vpc.yandex-cloud.jet.crossplane.io.v1alpha1.Network/'+net)['resource'];assert root['owner']=='stand/stroppy-run';cmd([CLI,'-n',NS,'delete','k8s.vpc.yandex-cloud.jet.crossplane.io.v1alpha1.Network',net,'--wait'],timeout=1800)
  wait_until(lambda:ensure_clean(S['fixture_uuid']),300,'owned fixture resources removed')
  proof=LIVE/'external-dsn-lifecycle-check.json'
  if proof.exists():
   d=json.loads(proof.read_text());d['fixture_cleanup']='passed';d['telemetry_verification']='passed' if (D/'runner/external-dsn.verified.json').exists() else 'pending';write(proof,d)
 print('external campaign finished; owned fixture cleanup checked',flush=True)
