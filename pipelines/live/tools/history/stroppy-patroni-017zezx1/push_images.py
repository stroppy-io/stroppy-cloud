import subprocess,pathlib,concurrent.futures,json,urllib.request
p=pathlib.Path(__file__).parent
def push(v):
 tag='registry.stroppy.io/stroppy-io/patroni:pg'+v+'-4.1.5'
 subprocess.run(['docker','tag','stroppy-patroni:'+v+'-4.1.5',tag],check=True)
 with (p/('push'+v+'.log')).open('w') as f:
  r=subprocess.run(['docker','--config',str(p/'docker'),'push',tag],stdout=f,stderr=subprocess.STDOUT)
 print('PG',v,'push exit',r.returncode,flush=True)
 if r.returncode: raise RuntimeError('push failed '+v)
 req=urllib.request.Request('https://docker.stroppy.io/v2/stroppy-io/patroni/manifests/pg'+v+'-4.1.5',headers={'Accept':'application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.list.v2+json'})
 with urllib.request.urlopen(req) as resp: digest=resp.headers.get('Docker-Content-Digest'); data=json.load(resp)
 return v,{'pull_digest':digest,'manifest':data}
with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
 out=dict(pool.map(push,['17','15','16','18']))
(p/'images.json').write_text(json.dumps(out,indent=2))
print('All published and pull manifests verified',flush=True)
