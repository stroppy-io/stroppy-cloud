import json,pathlib,tempfile,subprocess,hashlib,re,datetime,shutil
p=pathlib.Path('/tmp/stroppy-catalog-0z9hx668');live=pathlib.Path('/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live');source=(p/'classify_ydb_logs.py').read_text();root=pathlib.Path(tempfile.mkdtemp(prefix='log-filter-check-',dir=p));results=[]
cases=[('positive-26.1','26.1','positive'),('positive-26.2','26.2','positive'),('positive-26.3','26.3','positive'),('unknown-error','26.1','unknown'),('different-trace','26.1','trace'),('different-session-actor','26.2','actor'),('different-error-code','26.3','code'),('outside-drop-window','26.3','window'),('native-terminal-error','26.1','native')]
for label,version,change in cases:
 d=root/label;d.mkdir();l=d/'live';l.mkdir();n='ydb-mirror-3-dc-'+version
 for suffix in ['.partial-result.json','.log-candidates.json','.native-diagnostic.json']:(d/(n+suffix)).write_bytes((p/'ydb-parallel'/(n+suffix)).read_bytes())
 (d/'state.json').write_bytes((p/'ydb-parallel/state.json').read_bytes());(d/(n+'-artifacts')).symlink_to(p/'ydb-parallel'/(n+'-artifacts'));(l/(n+'.replication.json')).write_bytes((live/(n+'.replication.json')).read_bytes())
 f=d/(n+'.log-candidates.json');a=json.loads(f.read_text())
 if change=='unknown':a[0]['message']=a[0]['message'].split()[0]+' :KQP_COMPUTE ERROR: unknown injected failure'
 elif change=='trace':
  x=next(x for x in a if 'Handle abort execution event' in x['message']);x['message']=re.sub(r'TraceId : [0-9a-f-]+', 'TraceId : 00000000-0000-0000-0000-000000000000',x['message'])
 elif change=='actor':
  x=next(x for x in a if 'statusCode=UNAVAILABLE.' in x['message'] and 'code: 2005' in x['message']);x['message']=re.sub(r'SessionActorId: \[[^\]]+\]', 'SessionActorId: [0:0:0]',x['message'])
 elif change=='code':
  x=next(x for x in a if 'Received EvWriteResult with wrong shard state.' in x['message']);x['message']=x['message'].replace('code: 2029','code: 2999')
 elif change=='window':
  x=next(x for x in a if 'stroppy_demo' in x['message'] and '200200' in x['message']);stamp=x['message'].split()[0];later=(datetime.datetime.fromisoformat(stamp.replace('Z','+00:00'))+datetime.timedelta(seconds=1)).isoformat().replace('+00:00','Z');x['message']=x['message'].replace(stamp,later,1)
 elif change=='native':
  f2=d/(n+'.native-diagnostic.json');v=json.loads(f2.read_text());v['segments'][0]['measurements']['terminal_errors_total']=1;f2.write_text(json.dumps(v))
 for x in a:x['message_sha256']=hashlib.sha256(x['message'].encode()).hexdigest()
 f.write_text(json.dumps(a));script=d/'classifier.py';script.write_text(source.replace(str(live),str(l)))
 r=subprocess.run(['python3',str(script),str(d),n,'--partial'],capture_output=True,text=True);expected=change=='positive';assert (r.returncode==0)==expected,label
 results.append({'case':label,'accepted':r.returncode==0,'expected_acceptance':expected})
out={'status':'passed','checked_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'scope':'Real saved candidate sets and negative mutations, isolated output; cleanup and full workflow acceptance are checked separately','classifier_sha256':hashlib.sha256(source.encode()).hexdigest(),'schema_drop_cross_host_tolerance_seconds':.05,'observed_26_3_source_after_client_step_ms':26.869535446166992,'clock_offset_measured':False,'checks':results}
(live/'ydb-parallel-log-filter-check.json').write_text(json.dumps(out,indent=2)+'\n');print('passed',len(results),'real-fixture checks')
