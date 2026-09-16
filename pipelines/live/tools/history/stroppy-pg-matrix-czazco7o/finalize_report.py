import json,pathlib,subprocess,datetime
root=pathlib.Path('pipelines/live');work=pathlib.Path('/tmp/stroppy-pg-matrix-czazco7o')
rows=json.loads((work/'telemetry-validation.json').read_text());expected={x['id'] for x in json.loads((root/'suite-postgres-versions.json').read_text())['cells']};assert {x['cell'] for x in rows}==expected and len(rows)==12
refs=set()
listed=json.loads(subprocess.check_output(['graphenectl','-n','t-stroppy-live','run','list','--jq','.'],timeout=30));states={r['ref']:r['phase'] for r in listed['resources']}
for row in rows:
 version,topology=row['cell'][2:].split('-',1)
 row['template_input']='matrix/postgres-'+topology+'-'+version+'-simple.json'
 if row['run_id'].startswith('matrix-pg-versions-'):
  row['suite_input']='suite-postgres-versions.json';row['server_versions_during_run']=['0.2.5'];row['worker_image']='stroppy-run:5d4c94c13c882225'
 elif row['run_id'].startswith('matrix-pg-retry-'):
  row['suite_input']='suite-postgres-versions-retry.json';row['server_versions_during_run']=['0.2.6','0.2.7'] if topology=='primary-replica' else ['0.2.6'];row['worker_image']='stroppy-run:5d4c94c13c882225'
 else:
  row['suite_input']='suite-postgres-versions-final.json';row['server_versions_during_run']=['0.2.7'];row['worker_image']='stroppy-run:166453f8f98a639b'
 assert states.get('run/'+row['run_id'])=='Completed',(row['cell'],'not completed')
 result=json.loads((root/(row['cell']+'.result.json')).read_text());refs.update(result['artifacts'])
 if 'replica' in row['cell']:
  p=json.loads((root/(row['cell']+'.replication.json')).read_text())
  assert p['primary'][1]['rows']==[['100']] and p['replica'][1]['rows']==[['100']]
  assert p['primary'][0]['rows'][0][1]=='f' and p['replica'][0]['rows'][0][1]=='t'
  assert p['replica'][2]['rows'][0][0]=='streaming'
cleanup=json.loads((root/'postgres-matrix.cleanup.json').read_text());assert not any(cleanup['remaining_test_resources'].values())
artifacts=json.loads((root/'postgres-versions.artifacts.json').read_text());assert len(refs)==24;assert {x['ref'] for x in artifacts['artifacts']}==refs;assert {x['ref'] for x in artifacts['download_checks'] if x['download_verified']}==refs
ownership={}
for row in rows:
 tree=json.loads(subprocess.check_output(['graphenectl','-n','t-stroppy-live','tree','run/'+row['run_id'],'--jq','.'],timeout=30));assert not tree,(row['cell'],'ownership not empty');ownership[row['run_id']]=tree
(root/'postgres-matrix.ownership.json').write_text(json.dumps(ownership,indent=2)+'\n')
p=root/'postgres-matrix-check.json';x=json.loads(p.read_text());x.pop('server_version',None);x.pop('worker_image',None);x['current_server_version']='0.2.7';x.update({'status':'passed','validated':12,'cells':sorted(rows,key=lambda x:x['cell']),'checked_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'artifacts_verified':24,'replication_versions_verified':[15,16,17,18],'cleanup_evidence':'postgres-matrix.cleanup.json','ownership_evidence':'postgres-matrix.ownership.json','retention':{'configs':'until explicit deletion','artifact_logs':'30 days','shared_metrics':'90 days unchanged','shared_logs':'30 days unchanged','shared_traces':'14 days unchanged'}});p.write_text(json.dumps(x,indent=2)+'\n');print('12/12 passed; 24 artifacts verified; all test resources removed')
