import json,subprocess,sys,os
from pathlib import Path
p=Path(sys.argv[1]);n=sys.argv[2];cli='/tmp/stroppy-catalog-0z9hx668/bin/graphenectl'
os.environ['PATH']=str(Path(cli).parent)+os.pathsep+os.environ['PATH']
r=json.loads((p/(n+('.partial-result.json' if '--partial' in sys.argv else '.result.json'))).read_text());a=[]
for ref in r['artifacts']:
 v=json.loads(subprocess.check_output([cli,'-n','t-stroppy-live','get',ref,'--jq','.resource'],text=True,timeout=30));a.append({'ref':ref,'owner':v['owner'],'keepUntil':v['state'].get('keepUntil'),**v['state']['blob']})
f=p/(n+'.early-blobs.json');f.write_text(json.dumps(a));dest=p/(n+'-artifacts');dest.mkdir(mode=0o700,exist_ok=True)
v=subprocess.run(['/tmp/stroppy-catalog-0z9hx668/download-logs',str(f),str(dest)],capture_output=True,text=True,env={**os.environ,'GRAPHENE_CONFIG':'/tmp/stroppy-catalog-0z9hx668/forward-config.yaml'});(p/(n+'.early-download-error.txt')).write_text(v.stderr);v.check_returncode();(p/(n+'.early-downloads.json')).write_text(v.stdout);print(n,'downloaded',len(a))
