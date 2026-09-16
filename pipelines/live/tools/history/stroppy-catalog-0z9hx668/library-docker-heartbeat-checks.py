import concurrent.futures,subprocess
from pathlib import Path
p=Path('/tmp/stroppy-catalog-0z9hx668')
def check(name):
 with (p/('docker-heartbeat-'+name+'.log')).open('w') as out:r=subprocess.run(['make',name],cwd='/home/yaroher/devel/github/graphene-ci/library',stdout=out,stderr=subprocess.STDOUT)
 print(name,r.returncode,flush=True)
 if r.returncode:print((p/('docker-heartbeat-'+name+'.log')).read_text()[-4000:])
 return r.returncode
with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:codes=list(pool.map(check,['test','lint']))
raise SystemExit(max(codes))
