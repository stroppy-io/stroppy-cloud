import subprocess,pathlib,concurrent.futures
p=pathlib.Path(__file__).parent
def build(v):
 with (p/('build'+v+'.log')).open('w') as f:
  r=subprocess.run(['docker','build','-t','stroppy-patroni:'+v+'-4.1.5','--build-arg','PG_MAJOR='+v,'images/patroni'],stdout=f,stderr=subprocess.STDOUT)
 print('PG',v,'build exit',r.returncode,flush=True)
 return r.returncode
with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
 codes=list(pool.map(build,['15','16','18']))
assert not any(codes)
