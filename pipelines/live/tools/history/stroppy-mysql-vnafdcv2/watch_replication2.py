import json,subprocess,time,concurrent.futures,datetime
from pathlib import Path
p=Path(__file__).parent;d=p/'semisync2';live=Path('/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live')
state=json.loads((d/'state.json').read_text())
def probe(agent,target):
 r=subprocess.run(['python3',str(p/'probe_remote.py'),agent,str(target)],capture_output=True,text=True)
 target.with_suffix('.attempt.log').write_text(r.stdout+r.stderr)
 return r.returncode==0
def check_cell(item):
 name,c=item
 prefix=c['uuid'][:8];deadline=time.monotonic()+2400
 while time.monotonic()<deadline:
  targets=[d/(name+'-source.json'),d/(name+'-replica.json')];agents=[prefix+'-db-1',prefix+'-db-replica-1']
  with concurrent.futures.ThreadPoolExecutor() as ex:ok=list(ex.map(probe,agents,targets))
  if all(ok):
   source,replica=[json.loads(t.read_text()) for t in targets]
   status={x['Variable_name']:x['Value'] for x in source['semisync']};rs={x['Variable_name']:x['Value'] for x in replica['semisync']};settings={x['Variable_name']:x['Value'] for x in source['settings']}
   assert status['Rpl_semi_sync_source_status']=='ON';assert int(status['Rpl_semi_sync_source_clients'])>=1;assert int(status['Rpl_semi_sync_source_yes_tx'])>0
   assert replica['server'][0]['read_only']==1;assert source['server'][0]['read_only']==0;assert rs['Rpl_semi_sync_replica_status']=='ON';assert settings['rpl_semi_sync_source_wait_for_replica_count']=='1'
   r=replica['replication'][0];assert r['Replica_IO_Running']=='Yes' and r['Replica_SQL_Running']=='Yes';assert r['Last_IO_Errno']==0 and r['Last_SQL_Errno']==0
   assert source['replicated_rows'][0]['rows_present']==replica['replicated_rows'][0]['rows_present']==100
   checks={role:{k:x[k] for k in ['agent','server','plugins','semisync','settings','replicated_rows']} for role,x in [('source',source),('replica',replica)]}
   checks['replica']['replication']={k:r[k] for k in ['Replica_IO_Running','Replica_SQL_Running','Seconds_Behind_Source','Last_IO_Errno','Last_SQL_Errno','Auto_Position']}
   body={'status':'passed','run_id':c['run_id'],'checked_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'source':'read-only SQL on both live YC nodes during simple workload','checks':checks}
   (live/(name+'.replication.json')).write_text(json.dumps(body,indent=2)+'\n');print(name,'live semi-sync SQL passed',flush=True);break
  time.sleep(15)
 else:raise SystemExit(name+' replication probe timeout')

with concurrent.futures.ThreadPoolExecutor() as ex:list(ex.map(check_cell,[(n,c) for n,c in state['cells'].items() if n.startswith('mysql')]))
