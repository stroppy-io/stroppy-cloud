import subprocess,json,time,datetime,base64,importlib.util,pexpect
from pathlib import Path
out=Path(__file__).parent
public=Path('pipelines/live')
suite=json.loads((public/'suite-postgres-patroni.json').read_text());rid=(out/'run-id').read_text()
ms=importlib.util.spec_from_file_location('metrics',public/'inspect_metrics.py');metrics=importlib.util.module_from_spec(ms);ms.loader.exec_module(metrics)
from agent_sessions import inspect as inspect_sessions
created=json.loads((out/'instances.json').read_text()) if (out/'instances.json').exists() else {};last={};probed={rid+'-'+c['id'] for c in suite['cells'] if (public/(c['id']+'.agents.json')).exists()};saved={rid+'-'+c['id'] for c in suite['cells'] if (public/(c['id']+'.result.json')).exists()}
def cli(*args):return subprocess.check_output(['graphenectl','-n','t-stroppy-live',*args],text=True,timeout=25)
def remote(agent,code):
 b64=base64.b64encode(code.encode()).decode()
 c=pexpect.spawn('graphenectl',['-n','t-stroppy-live','agent','shell',agent],encoding='utf-8',timeout=25)
 try:
  c.expect(r'\$ ');c.sendline('stty -echo -icanon');c.expect(r'\$ ')
  c.sendline('python3 -c "import base64; exec(base64.b64decode(\''+b64+'\'))"');c.expect(r'\$ ')
  return c.before
 finally:c.sendline('exit');c.close(force=True)
def cluster(agent):
 text=remote(agent,"import json,urllib.request; print('PROOF:'+json.dumps(json.load(urllib.request.urlopen('http://127.0.0.1:8008/cluster'))))")
 return json.loads(next(x.split('PROOF:',1)[1] for x in text.splitlines() if x.startswith('PROOF:')))
def probe(agent,seed):
 code=('import sys;sys.argv=["probe"]'+(';sys.argv.append("--seed")' if seed else '')+'\n'+(public/'postgres_probe.py').read_text())
 text=remote(agent,code);rows=[json.loads(s) for s in text.splitlines() if s.startswith('{"query"')]
 if len(rows)!=4:raise RuntimeError(text[-1000:])
 return rows
for _ in range(240):
 now=datetime.datetime.now(datetime.timezone.utc).isoformat()
 try:
  instances=json.loads(subprocess.check_output(['yc','compute','instance','list','--folder-id','b1ghttqg66t14ldkvfcq','--format','json'],text=True,timeout=25))
  prefixes=[c['run_spec']['run_id'][:8] for c in suite['cells']]
  for x in instances:
   if any(p in x['name'] for p in prefixes):
    previous=created.get(x['id'],{})
    seen_disks={d['disk_id']:d for d in (previous.get('secondary_disks') or [])+(x.get('secondary_disks') or []) if d.get('disk_id')}
    created[x['id']]={k:x.get(k) for k in ['id','name','status']}
    created[x['id']]['boot_disk']=previous.get('boot_disk') or x.get('boot_disk')
    created[x['id']]['secondary_disks']=list(seen_disks.values())
  (out/'instances.json').write_text(json.dumps(created,indent=2))
  listed=json.loads(cli('run','list','--jq','.'));rows={x['ref']:x for x in listed.get('resources',[]) if rid in x['ref']}
  active=[]
  for cell in suite['cells']:
   cr=rid+'-'+cell['id'];row=rows.get('run/'+cr)
   if not row:continue
   phase=row.get('phase');active.append((cell['id'],phase))
   if phase=='Running':
    status=cli('run','status',cr)
    if status!=last.get(cr):print(now,cell['id'],status.strip().splitlines()[1:5],flush=True);last[cr]=status
    if 'stroppy.segment.run' in status and cr not in probed:
     prefix=cell['run_spec']['run_id'][:8]
     try:
      info=cluster(prefix+'-db-1');members=info['members'];leader=next(x['name'] for x in members if x['role']=='leader')
      proof={leader:probe(prefix+'-'+leader,True)}
      for m in members:
       if m['name']!=leader:proof[m['name']]=probe(prefix+'-'+m['name'],False)
      assert len(proof)==3 and all(x[1]['rows']==[['100']] for x in proof.values())
      assert proof[leader][0]['rows'][0][1]=='f'
      for name,x in proof.items():
       if name!=leader:assert x[0]['rows'][0][1]=='t' and x[2]['rows'][0][0]=='streaming'
      assert any(x[1]=='sync' for x in proof[leader][3]['rows'])
      (public/(cell['id']+'.replication.json')).write_text(json.dumps({'run_id':cr,'checked_at':now,'cluster':info,'nodes':proof},indent=2)+'\n')
      sessions=inspect_sessions(cell['run_spec'])
      (public/(cell['id']+'.agents.json')).write_text(json.dumps(sessions,indent=2)+'\n')
      probed.add(cr);print(now,cell['id'],'3 members, rows100, synchronous replication verified',flush=True)
     except Exception as e:print(now,cell['id'],'probe retry',str(e)[-350:],flush=True)
   elif phase in ['Completed','Failed','Canceled'] and cr not in saved:
    try:
     result=json.loads(cli('run','result',cr,'-o','json'));(public/(cell['id']+'.result.json')).write_text(json.dumps(result,indent=2)+'\n')
     saved.add(cr)
     print(now,cell['id'],phase,'iterations',result.get('metrics',{}).get('smoke.iterations_total'),flush=True)
    except Exception as e:print(now,cell['id'],phase,'capture',str(e)[-300:],flush=True)
  print(now,'cells',active,flush=True)
  phase=rows.get('run/'+rid,{}).get('phase')
  if phase in ['Completed','Failed','Canceled']:
   if phase=='Completed':
    result=json.loads(cli('run','result',rid,'-o','json'));(public/'suite-postgres-patroni.result.json').write_text(json.dumps(result,indent=2)+'\n');print('FINAL',result.get('total'),result.get('done'),result.get('failed'),flush=True)
   else:print('FINAL',phase,flush=True)
   break
 except Exception as e:print(now,'monitor error',str(e)[-400:],flush=True)
 time.sleep(20)
