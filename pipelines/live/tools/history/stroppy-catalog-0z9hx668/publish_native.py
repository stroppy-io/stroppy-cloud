import json,subprocess,os,base64,time,hashlib,datetime,re,sys,shutil
from pathlib import Path
p=Path(__file__).parent/'stroppy-native';kind=sys.argv[1];m=json.loads((p/'release-source.json').read_text());assert 'ok' in (p/(kind+'-integration.log')).read_text();tag='registry.stroppy.io/stroppy-io/stroppy:dev-native-'+m['binary_sha256'][:12];auth=p/'push-auth';auth.mkdir(mode=0o700,exist_ok=True);env=dict(os.environ,YC_CLI_INITIALIZATION_SILENCE='true');password=None
for attempt in range(6):
 try:
  r=subprocess.run(['kubectl','--kubeconfig','/tmp/stroppy-mysql-vnafdcv2/kubeconfig','--context','stroppy-live-test','-n','nexus','get','secret','nexus-admin','-o','json'],capture_output=True,text=True,timeout=40,env=env)
  if r.returncode==0:password=base64.b64decode(json.loads(r.stdout)['data']['password']).decode();break
 except subprocess.TimeoutExpired:pass
 print('Registry credential read transient failure',attempt+1,flush=True);time.sleep(2)
if password is None:raise RuntimeError('Registry credentials unavailable')
try:
 r=subprocess.run(['docker','--config',str(auth),'login','registry.stroppy.io','-u','admin','--password-stdin'],input=password,capture_output=True,text=True);password=None;r.check_returncode()
 with (p/(kind+'-image-push.log')).open('w') as log:subprocess.run(['docker','--config',str(auth),'push',tag],stdout=log,stderr=subprocess.STDOUT,check=True)
finally:
 subprocess.run(['docker','--config',str(auth),'logout','registry.stroppy.io'],capture_output=True,text=True)
log=(p/(kind+'-image-push.log')).read_text();digest=re.search(r'digest: (sha256:[0-9a-f]{64})',log).group(1);pinned='docker.stroppy.io/stroppy-io/stroppy@'+digest
with (p/(kind+'-mirror.log')).open('w') as log:subprocess.run(['docker','pull',pinned],stdout=log,stderr=subprocess.STDOUT,check=True)
actual=subprocess.check_output(['docker','run','--rm','--entrypoint','sha256sum',pinned,'/usr/local/bin/stroppy'],text=True).split()[0];assert actual==m['binary_sha256']
version=subprocess.check_output(['docker','run','--rm',pinned,'version'],text=True);assert m['version'] in version
m.update(tag=tag,pinned=pinned,digest=digest,binary_verified_in_mirror_image=True,checked_at=datetime.datetime.now(datetime.timezone.utc).isoformat(),tests=['make linter_fix','make linter','make tests','make integration'])
for target in [p/(kind+'-image.json'),p/m['version']/(kind+'-image.json'),Path('/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live')/(kind+'-native-image.json')]:target.write_text(json.dumps(m,indent=2)+'\n')
print(pinned,version,flush=True)
