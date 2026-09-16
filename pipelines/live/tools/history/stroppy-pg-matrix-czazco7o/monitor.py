import subprocess,json,time,datetime,base64,importlib.util,pexpect
from pathlib import Path
out=Path('/tmp/stroppy-pg-matrix-czazco7o')
suite=json.loads(Path('pipelines/live/suite-postgres-versions.json').read_text())
rid=(out/'versions-run-id').read_text()
metricspec=importlib.util.spec_from_file_location('metrics','pipelines/live/inspect_metrics.py');metrics=importlib.util.module_from_spec(metricspec);metricspec.loader.exec_module(metrics)
created={};last={};probed=set();saved=set()
def cli(*args):return subprocess.check_output(['graphenectl','-n','t-stroppy-live',*args],text=True,timeout=20)
def probe(agent,seed):
 code=base64.b64encode(Path('pipelines/live/postgres_probe.py').read_bytes()).decode()
 c=pexpect.spawn('graphenectl',['-n','t-stroppy-live','agent','shell',agent],encoding='utf-8',timeout=20)
 try:
  c.expect(r'\$ ');c.sendline('stty -echo -icanon');c.expect(r'\$ ')
  c.sendline('python3 -c "import base64; exec(base64.b64decode(\''+code+'\'))"'+(' --seed' if seed else ''))
  c.expect(r'\$ ')
  rows=[json.loads(s) for s in c.before.splitlines() if s.startswith('{"query"')]
  if len(rows)!=4:raise RuntimeError(c.before[-1000:])
  return rows
 finally:c.sendline('exit');c.close(force=True)
for _ in range(240):
 now=datetime.datetime.now(datetime.timezone.utc).isoformat()
 try:
  instances=json.loads(subprocess.check_output(['yc','compute','instance','list','--folder-id','b1ghttqg66t14ldkvfcq','--format','json'],text=True,timeout=20))
  prefixes=[c['run_spec']['run_id'][:8] for c in suite['cells']]+['9ccad320','b5047236']
  for x in instances:
   if any(p in x['name'] for p in prefixes):created[x['id']]={k:x.get(k) for k in ['id','name','status','boot_disk','secondary_disks']}
  (out/'versions-instances.json').write_text(json.dumps(created,indent=2))
  listed=json.loads(cli('run','list','--jq','.'))
  rows={x['ref']:x for x in listed.get('resources',[]) if rid in x['ref']}
  active=[]
  for cell in suite['cells']:
   cr=rid+'-'+cell['id'];row=rows.get('run/'+cr)
   if not row:continue
   phase=row.get('phase');active.append((cell['id'],phase))
   if phase=='Running':
    status=cli('run','status',cr)
    if 'primary-replica' in cell['id'] and 'stroppy.segment.run' in status and cr not in probed:
     prefix=cell['run_spec']['run_id'][:8]
     try:
      p=probe(prefix+'-db-1',True);r=probe(prefix+'-db-replica-1',False)
      assert p[1]['rows']==[['100']] and r[1]['rows']==[['100']]
      assert p[0]['rows'][0][1]=='f' and r[0]['rows'][0][1]=='t'
      assert r[2]['rows'][0][0]=='streaming'
      (out/(cell['id']+'.replication.json')).write_text(json.dumps({'primary':p,'replica':r},indent=2));probed.add(cr);print(now,cell['id'],'replication verified',flush=True)
     except Exception as e:print(now,cell['id'],'probe retry',str(e)[-300:],flush=True)
    if status!=last.get(cr):print(now,status.strip(),flush=True);last[cr]=status
   elif phase in ['Completed','Failed','Canceled'] and cr not in saved:
    try:
     result=json.loads(cli('run','result',cr,'-o','json'));(out/(cell['id']+'.result.json')).write_text(json.dumps(result,indent=2));m=metrics.inspect(cr,'t-stroppy-live');(out/(cell['id']+'.metrics.json')).write_text(json.dumps(m,indent=2));saved.add(cr)
     print(now,cell['id'],phase,'iterations',result.get('metrics',{}).get('smoke.iterations_total'),'exporters',len(m['exporters']),flush=True)
    except Exception as e:print(now,cell['id'],phase,'capture',str(e)[-300:],flush=True)
  print(now,'cells',active,flush=True)
  parent=rows.get('run/'+rid,{})
  if parent.get('phase') in ['Completed','Failed','Canceled']:
   result=json.loads(cli('run','result',rid,'-o','json'));(out/'versions.result.json').write_text(json.dumps(result,indent=2));print('FINAL',result.get('total'),result.get('done'),result.get('failed'),flush=True);break
 except Exception as e:print(now,'monitor error',str(e)[-400:],flush=True)
 time.sleep(25)
