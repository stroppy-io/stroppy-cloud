import json,subprocess,sys
from pathlib import Path
p=Path('/tmp/stroppy-catalog-0z9hx668');d=p/'ydb-autonomous';s=json.loads((d/'state.json').read_text());versions=sys.argv[1:] or ['26.1','26.2','26.3']
for ver in versions:
 n='ydb-mirror-3-dc-'+ver+'-autonomous';c=s['cells'][n];f=d/(n+'.partial-result.json')
 r=subprocess.run(['go','run',str(p/'extract_segments.go'),c['run_id'],str(f)],cwd='/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines',capture_output=True,text=True,timeout=60);r.check_returncode();v=json.loads(f.read_text())
 if len(v['segments'])!=3:print(n,'segments incomplete',flush=True);continue
 print(n,[(x['name'],x['status'],x['metrics'].get('terminal_errors_total')) for x in v['segments']],flush=True)
 v['artifacts']=['artifact/'+c['uuid'][:8]+'-stroppy-'+x['name']+'-'+kind for x in v['segments'] for kind in ['config','log']];v['partial_result_source']='Completed workload activities; cleanup still running; not a canonical workflow result';f.write_text(json.dumps(v,indent=2)+'\n')
 commands=[['native_only.py',str(d),n,n,'--partial'],['download_cell_artifacts.py',str(d),n,'--partial'],['inspect_ydb_candidates.py',n,str(d),'--partial'],['classify_ydb_logs.py',str(d),n,'--partial']]
 for cmd in commands:
  r=subprocess.run(['python3',str(p/cmd[0]),*cmd[1:]],capture_output=True,text=True,timeout=180);(d/(n+'.early-'+cmd[0]+'.log')).write_text(r.stdout+r.stderr)
  if r.returncode:print(n,cmd[0],'failed; inspect private log',flush=True);break
  print(r.stdout.strip(),flush=True)
