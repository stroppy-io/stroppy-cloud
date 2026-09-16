import json,hashlib,subprocess,tarfile,shutil
from pathlib import Path
r=Path('/home/yaroher/devel/github/stroppy-io/stroppy');p=Path('/tmp/stroppy-catalog-0z9hx668/stroppy-native');ctx=p/'image-build';ctx.mkdir(mode=0o700,exist_ok=True)
def git(*args):return subprocess.check_output(['git',*args],cwd=r,text=True).splitlines()
files=sorted(set(git('diff','--name-only')+git('ls-files','--others','--exclude-standard')));files=[x for x in files if not x.startswith('easyp_vendor/') and (r/x).is_file()]
m={'base_commit':git('rev-parse','HEAD')[0],'files':{f:hashlib.sha256((r/f).read_bytes()).hexdigest() for f in files}};h=hashlib.sha256(json.dumps(m,sort_keys=True).encode()).hexdigest();m.update(source_hash=h,version='dev-native-'+h[:12])
snapshot=p/m['version'];snapshot.mkdir(mode=0o700,exist_ok=True)
with tarfile.open(snapshot/'release-source.tar.gz','w:gz') as t:
 for f in files:t.add(r/f,arcname=f)
shutil.copy2(snapshot/'release-source.tar.gz',p/'release-source.tar.gz')
m['source_archive_sha256']=hashlib.sha256((p/'release-source.tar.gz').read_bytes()).hexdigest()
with (p/'release-build.log').open('w') as log:subprocess.run(['make','build','VERSION='+m['version']],cwd=r,stdout=log,stderr=subprocess.STDOUT,check=True)
shutil.copy2(r/'build/stroppy',ctx/'stroppy');(ctx/'Dockerfile').write_text((p/'Dockerfile').read_text());m['binary_sha256']=hashlib.sha256((ctx/'stroppy').read_bytes()).hexdigest();(p/'release-source.json').write_text(json.dumps(m,indent=2));shutil.copy2(p/'release-source.json',snapshot/'release-source.json');print(m['version'],m['binary_sha256'],flush=True)
