import json,subprocess,pathlib,sys,datetime
root=pathlib.Path('pipelines/live');work=pathlib.Path('/tmp/stroppy-patroni-017zezx1');rows=[];blobs=[]
for cell in sys.argv[1:]:
 result=json.loads((root/(cell+'.result.json')).read_text())
 for ref in result['artifacts']:
  x=json.loads(subprocess.check_output(['graphenectl','-n','t-stroppy-live','get',*ref.split('/',1),'--jq','.resource']))
  assert x['phase']=='ready' and x['owner']=='stand/stroppy-run' and x['state']['verified']
  keep=x['state'].get('keepUntil');assert bool(keep)==ref.endswith('-log')
  if keep:
   expiry=datetime.datetime.fromisoformat(keep.replace('Z','+00:00'));finished=datetime.datetime.fromisoformat(result['segments'][-1]['finished_at'].replace('Z','+00:00'));assert abs((expiry-finished).total_seconds()-30*86400)<3600,(ref,'unexpected retention')
  rows.append(dict(ref=ref,phase=x['phase'],owner=x['owner'],keep_until=keep))
  blobs.append(dict(ref=ref,**x['state']['blob']))
path=work/'blobs-to-check.json';path.write_text(json.dumps(blobs))
checked=json.loads(subprocess.check_output(['/tmp/stroppy-pg-matrix-czazco7o/check-blobs',str(path)]))
(root/'patroni.artifacts.json').write_text(json.dumps(dict(artifacts=rows,download_checks=checked),indent=2)+'\n')
print('Downloaded and verified',len(checked),'artifacts')
