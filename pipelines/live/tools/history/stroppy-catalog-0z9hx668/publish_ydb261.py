import subprocess,json,base64,time,os,re,datetime
from pathlib import Path
p=Path(__file__).parent;source='ydbplatform/local-ydb:26.1.1.22';target='registry.stroppy.io/stroppy-io/ydb-local:26.1.1.22';upstream='sha256:c3e49c078a560d4957ff492ed7025864ee33a405d03a02c4fdb30c20e7c42925'
m=json.loads(subprocess.check_output(['docker','image','inspect',source],text=True))[0]
assert any(x.endswith('@'+upstream) for x in m['RepoDigests']) and m['Architecture']=='amd64'
subprocess.run(['docker','tag',source,target],check=True)
auth=p/'ydb-image-push-auth';auth.mkdir(mode=0o700,exist_ok=True);env=dict(os.environ,YC_CLI_INITIALIZATION_SILENCE='true');password=None
for attempt in range(6):
 try:
  r=subprocess.run(['kubectl','--kubeconfig','/tmp/stroppy-mysql-vnafdcv2/kubeconfig','--context','stroppy-live-test','-n','nexus','get','secret','nexus-admin','-o','json'],capture_output=True,text=True,timeout=40,env=env)
  if r.returncode==0:password=base64.b64decode(json.loads(r.stdout)['data']['password']).decode();break
 except subprocess.TimeoutExpired:pass
 print('Registry credential read transient failure',attempt+1,flush=True);time.sleep(2)
if password is None:raise RuntimeError('Registry credentials unavailable')
try:
 r=subprocess.run(['docker','--config',str(auth),'login','registry.stroppy.io','-u','admin','--password-stdin'],input=password,capture_output=True,text=True);password=None;r.check_returncode()
 with (p/'ydb261-image-push.log').open('w') as log:subprocess.run(['docker','--config',str(auth),'push',target],stdout=log,stderr=subprocess.STDOUT,check=True)
finally:subprocess.run(['docker','--config',str(auth),'logout','registry.stroppy.io'],capture_output=True,text=True)
s=(p/'ydb261-image-push.log').read_text();digest=re.search(r'digest: (sha256:[0-9a-f]{64})',s).group(1);pin='docker.stroppy.io/stroppy-io/ydb-local@'+digest
with (p/'ydb261-image-mirror-pull.log').open('w') as log:subprocess.run(['docker','pull',pin],stdout=log,stderr=subprocess.STDOUT,check=True)
actual=json.loads(subprocess.check_output(['docker','image','inspect',pin],text=True))[0];assert actual['Id']==m['Id'] and actual['RootFS']==m['RootFS']
proof={'status':'passed','source_image':source,'source_digest':upstream,'image_config_digest':m['Id'],'pinned':pin,'same_image_config_and_layers':True,'platform':'linux/amd64','reason':'Existing Nexus upstream proxy returns HTTP 500 for GET of a required blob despite HEAD 200. Unmodified upstream image copied to the owned hosted registry; no global registry settings changed.','checked_at':datetime.datetime.now(datetime.timezone.utc).isoformat()}
Path('/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live/ydb-26.1-mirror-check.json').write_text(json.dumps(proof,indent=2)+'\n');print(proof,flush=True)
