import json,datetime,hashlib,re,collections,sys
from pathlib import Path
p=Path(sys.argv[1]);name=sys.argv[2];live=Path('/tmp/stroppy-catalog-0z9hx668/log-filter-check-ppscz4ec/positive-26.3/live');parse=lambda s:datetime.datetime.fromisoformat(s.replace('Z','+00:00'))
r=json.loads((p/(name+('.partial-result.json' if '--partial' in sys.argv else '.result.json'))).read_text());state=json.loads((p/'state.json').read_text())['cells'][name];a=json.loads((p/(name+'.log-candidates.json')).read_text());native=json.loads((p/(name+'.native-diagnostic.json')).read_text());health=json.loads((live/(name+'.replication.json')).read_text());assert native['status']==health['status']=='passed' and native['run_id']==health['run_id']==state['run_id']
for seg in native['segments']:assert seg['measurements']['terminal_errors_total']==0
start=parse(r['segments'][0]['started_at']);windows={};counts=collections.Counter();out=[]
for segment in r['segments']:
 f=p/(name+'-artifacts')/(state['uuid'][:8]+'-stroppy-'+segment['name']+'-log.log');rows=[]
 for line in f.read_text().splitlines():
  try:row=json.loads(line)
  except ValueError:continue
  if isinstance(row,dict):rows.append(row)
 lo=next(x['ts'] for x in rows if x.get('msg')=="Start of 'drop_schema' step")
 hi=next(x['ts'] for x in rows if x.get('msg','').startswith("End of 'drop_schema' step"))
 windows[segment['name']]={'start_unix':lo,'end_unix':hi,'cross_host_clock_tolerance_seconds':.05,'source_artifact_sha256':hashlib.sha256(f.read_bytes()).hexdigest()}
for x in a:
 x=x.copy();m=x.pop('message');stamp=parse(m.split()[0]);ref=None
 if stamp<start:
  classification='ydb_bootstrap_before_workload';note='Container source timestamp precedes the first workload; ingested later. All expected members subsequently passed Green/RUNNING/storage HTTP checks. Bootstrap message retained.'
 elif ' :TX_PROXY ERROR:' in m and (re.search(r'issues: \{ message: "Path does not exist" issue_code: 200200 severity: 1 \}$',m) or 'issue_code: 200200 severity: 1' in m and re.search(r'Path `/Root/stroppy/[a-z_]+` does not exist',m)):
  matched=[k for k,w in windows.items() if w['start_unix']-.05<=stamp.timestamp()<=w['end_unix']+.05];assert len(matched)==1,(m,matched)
  counts[matched[0]]+=1;classification='ydb_initial_drop_missing_table';note='Exact missing-path status during successful DROP TABLE IF EXISTS setup; source timestamps matched to downloaded Stroppy step boundaries, 50ms cross-host tolerance; exact schema-drop counts are required.';ref='https://github.com/ydb-platform/ydb/blob/stable-25-4-1/ydb/core/tx/tx_proxy/schemereq.cpp#L570'
 elif ' :TX_PROXY ERROR:' in m and '/Root/stroppy/.metadata/workload_manager/pools/default' in m and 'path exist, request accepts it' in m and 'type: EPathTypeResourcePool, state: EPathStateNoChanges' in m:
  assert start<=stamp and stamp.timestamp()<windows['simple']['start_unix']
  classification='ydb_default_pool_already_exists';note='Default resource pool already exists during first connection. Pinned source returns StatusAlreadyExists when acceptAlreadyExist is true; pool state is NoChanges, native requests succeed.';ref='https://github.com/ydb-platform/ydb/blob/stable-26-3-1/ydb/core/tx/schemeshard/schemeshard_path.cpp#L695'
 elif ' :KQP_EXECUTER ERROR:' in m and (' ABORTED: ' in m or 'Runtime error Status# ABORTED Issues#' in m or 'Runtime error actorId=' in m and 'marker=KQPEX status=ABORTED' in m) and 'Transaction locks invalidated.' in m and 'code: 2001' in m or ' :KQP_COMPUTE ERROR:' in m and (re.search(r'\bstatusCode=ABORTED(?:[.\s]|$)',m) and 'Transaction locks invalidated.' in m or ('Got LOCKS BROKEN for table' in m or re.search(r'Received (?:external )?EvWriteResult with locks broken status[.]',m)) and ('Operation is aborting because locks are not valid' in m or 'Operation is aborting because it cannot acquire locks' in m)) and 'code: 2001' in m:
  classification='ydb_transaction_conflict';note='Optimistic lock conflict; entire transaction must be retried. Native retry counters retained; terminal errors are checked independently.';ref='https://ydb.tech/docs/en/reference/ydb-sdk/ydb-status-codes#400040-aborted'
 elif ' :KQP_COMPUTE ERROR:' in m and 'Read request aborted' in m and 'Read conflict with concurrent transaction' in m and ('InternalError: ABORTED' in m or 'Source[0] fatal error:' in m):
  classification='ydb_read_transaction_conflict';note='Read was aborted because of a concurrent transaction; native retries and terminal errors checked independently.';ref='https://ydb.tech/docs/en/reference/ydb-sdk/ydb-status-codes#400040-aborted'
 elif ' :KQP_COMPUTE ERROR:' in m and 'Read request aborted' in m and 'Transaction was already committed or aborted' in m and ('InternalError: ABORTED' in m or 'Source[0] fatal error:' in m):
  classification='ydb_finished_transaction_read_abort';note='A late read cannot acquire a lock for a transaction already committed or aborted. Pinned datashard source returns ABORTED for EEnsureCurrentLock::Abort. Native retries and zero terminal errors checked independently.';ref='https://github.com/ydb-platform/ydb/blob/92bb02100ac7025660adf9c83d2cb2cfe3ef16b5/ydb/core/tx/datashard/datashard__read_iterator.cpp#L2063'
 elif ' :KQP_COMPUTE ERROR:' in m and 'Handle abort execution event from:' in m and 'status: ABORTED, reason: { <main>: Error: Terminate execution }' in m:
  tx=re.search(r'TxId: ([0-9]+)',m).group(1)
  trace=re.search(r'TraceId : ([0-9a-f-]+)[.]',m).group(1)
  session=re.search(r'SessionId : (\S+)',m).group(1)
  members={'docker/'+state['uuid']+'-'+node+'-ydb' for node in health['checks']}
  assert x['entity'] in members
  matching=[v for v in a if v['entity'] in members and ' :KQP_COMPUTE ERROR:' in v['message'] and 'TxId: '+tx+',' in v['message'] and 'TraceId : '+trace+'.' in v['message'] and 'SessionId : '+session in v['message'] and ('Read conflict with concurrent transaction' in v['message'] or 'Read request aborted' in v['message'] and 'Transaction was already committed or aborted' in v['message']) and ('InternalError: ABORTED' in v['message'] or 'Source[0] fatal error:' in v['message']) and -.025<=(stamp-parse(v['message'].split()[0])).total_seconds()<=1];assert matching,x['message_sha256']
  x['correlation']={'read_conflict_hashes':[v['message_sha256'] for v in matching],'same_transaction_trace_session':True,'cross_member':any(v['entity']!=x['entity'] for v in matching),'max_source_delay_seconds':max((stamp-parse(v['message'].split()[0])).total_seconds() for v in matching),'allowed_delay_seconds':1,'cross_host_clock_tolerance_seconds':.025}
  classification='ydb_finished_transaction_sibling_abort' if any('Transaction was already committed or aborted' in v['message'] for v in matching) else 'ydb_read_conflict_sibling_abort';note='Correlated sibling cancellation: same transaction, trace and session as an explicit read-abort report (concurrent conflict or already finished transaction) on verified members, within one second. Pinned executor source propagates an ABORTED result to unfinished compute tasks. Native retries and zero terminal errors checked independently.';ref='https://github.com/ydb-platform/ydb/blob/stable-25-4-1/ydb/core/kqp/executer_actor/kqp_executer_impl.h#L1094'
 elif ' :TX_DATASHARD ERROR:' in m and ('Errors while proposing transaction' in m or 'Prepare transaction failed.' in m) and 'Status: STATUS_LOCKS_BROKEN' in m and 'issue_code: 2001' in m and 'Operation is aborting because it cannot acquire locks' in m:
  classification='ydb_transaction_conflict';note='Transaction could not acquire optimistic locks; native retries and zero terminal errors checked separately.';ref='https://ydb.tech/docs/en/troubleshooting/performance/queries/transaction-lock-invalidation'
 elif (' :KQP_COMPUTE ERROR:' in m or ' :KQP_EXECUTER ERROR:' in m) and ('WRONG SHARD STATE' in m or 'wrong shard state.' in m or 'UNAVAILABLE' in m) and 'is in a pre/offline state assuming this is due to a finished split (wrong shard state), code: 2029' in m:
  classification='ydb_finished_split_wrong_shard';note='Write addressed a shard made pre/offline by a finished split, explicit issue 2029. Pinned write actor retries resolution or returns UNAVAILABLE; native retries and zero terminal errors checked independently.';ref='https://github.com/ydb-platform/ydb/blob/92bb02100ac7025660adf9c83d2cb2cfe3ef16b5/ydb/core/kqp/runtime/kqp_write_actor.cpp#L888'
 elif ' :KQP_COMPUTE ERROR:' in m and 'kqp_write_actor.cpp:' in m and 'statusCode=UNAVAILABLE.' in m and 'Wrong shard state. Table' in m and 'code: 2005' in m:
  actor=re.search(r'SessionActorId: (\[[^\]]+\])',m).group(1);table=re.search(r'Table `([^`]+)`',m).group(1)
  matching=[v for v in a if v['entity']==x['entity'] and 'SessionActorId: '+actor in v['message'] and table in v['message'] and 'is in a pre/offline state assuming this is due to a finished split (wrong shard state), code: 2029' in v['message'] and 0<=(stamp-parse(v['message'].split()[0])).total_seconds()<.001];assert matching,x['message_sha256']
  x['correlation']={'wrong_shard_report_hashes':[v['message_sha256'] for v in matching],'same_member_session_actor_table':True,'maximum_delay_seconds':.001}
  classification='ydb_finished_split_wrong_shard';note='UNAVAILABLE status matches explicit finished-split issue 2029 on the same member, session actor and table within 1ms. Native retries and zero terminal errors checked independently.';ref='https://github.com/ydb-platform/ydb/blob/92bb02100ac7025660adf9c83d2cb2cfe3ef16b5/ydb/core/kqp/runtime/kqp_write_actor.cpp#L898'
 elif ' :KQP_COMPUTE ERROR:' in m and 'kqp_write_actor.cpp:' in m and 'statusCode=ABORTED. Issue=<main>: Error: Operation aborted., code: 2011' in m:
  actor=re.search(r'SessionActorId: (\[[^\]]+\])',m).group(1)
  matching=[v for v in a if v['entity']==x['entity'] and 'SessionActorId: '+actor in v['message'] and 'Got ABORTED for table' in v['message'] and re.search(r'DataShard [0-9]+ is splitting, code: 2011',v['message']) and 0<=(stamp-parse(v['message'].split()[0])).total_seconds()<.001];assert matching,x['message_sha256']
  x['correlation']={'split_report_hashes':[v['message_sha256'] for v in matching],'same_member_session_actor':True,'maximum_delay_seconds':.001}
  classification='ydb_tablet_split_abort';note='Generic write-abort status correlated with an explicit DataShard splitting report on the same member and session actor within 1ms. Native retries and zero terminal errors checked independently.';ref='https://ydb.tech/docs/en/reference/ydb-sdk/ydb-status-codes#400040-aborted'
 elif (' :KQP_COMPUTE ERROR:' in m and 'kqp_write_actor.cpp:' in m and 'Got ABORTED for table' in m or ' :KQP_EXECUTER ERROR:' in m and 'Runtime error Status# ABORTED Issues#' in m) and re.search(r'DataShard [0-9]+ is splitting, code: 2011',m):
  classification='ydb_tablet_split_abort';note='The write actor explicitly reports ABORTED because the named DataShard is splitting. Kept separate from overload and lock conflicts; native retries and zero terminal errors are required.';ref='https://ydb.tech/docs/en/reference/ydb-sdk/ydb-status-codes#400040-aborted'
 elif (' :KQP_EXECUTER ERROR:' in m and ('Runtime error Status# OVERLOADED Issues#' in m or 'marker=KQPEX status=OVERLOADED' in m) or ' :KQP_COMPUTE ERROR:' in m and re.search(r'\bstatusCode=OVERLOADED\b',m)) and 'is in process of split opId' in m and 'code: 2006' in m:
  classification='ydb_tablet_split_retry';note='Tablet rejects a transaction while splitting; native retries succeed with zero terminal errors.';ref='https://ydb.tech/docs/en/reference/ydb-sdk/ydb-status-codes#400060-overloaded'
 elif ' :KQP_COMPUTE ERROR:' in m and 'statusCode=OVERLOADED.' in m and 'code: 2006' in m:
  table=re.search(r'Table `([^`]+)`',m).group(1)
  matching=[v for v in a if ' :KQP_EXECUTER ERROR:' in v['message'] and 'Runtime error Status# OVERLOADED Issues#' in v['message'] and 'is in process of split opId' in v['message'] and v['entity']==x['entity'] and table in v['message'] and abs((parse(v['message'].split()[0])-stamp).total_seconds())<.001];assert len(matching)==1,x['message_sha256']
  classification='ydb_tablet_split_retry';note='Compute overload matches the exact executor tablet-split report within 1ms on the same database member and table; native retry counters retained.';ref='https://ydb.tech/docs/en/reference/ydb-sdk/ydb-status-codes#400060-overloaded'
 elif ' :TX_DATASHARD ERROR:' in m and 'Complete volatile write [' in m and 'Status: STATUS_ABORTED' in m and 'Distributed transaction aborted due to commit failure' in m and 'issue_code: 2011' in m:
  classification='ydb_aborted_volatile_write';note='Datashard reports aborted distributed commit; retained separately from successful logical transactions and native terminal errors.';ref='https://github.com/ydb-platform/ydb/blob/stable-25-4-1/ydb/core/tx/datashard/datashard.cpp#L755'
 else:raise RuntimeError('Unclassified message hash '+x['message_sha256'])
 x.update(classification=classification,note=note,source_timestamp=stamp.isoformat())
 if ref:x['reference']=ref
 out.append(x)
assert dict(counts)=={'simple':1,'tpcb-tx':4,'tpcc-tx':9},counts
proof={'status':'passed','run_id':state['run_id'],'checked_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'scope':'Exact log hashes only. Native zero terminal errors, native retries, downloaded setup logs and independent member health are required; bootstrap observations remain visible.','member_health_file':name+'.replication.json','native_measurements':{s['segment']:s['measurements'] for s in native['segments']},'drop_schema_windows':windows,'initial_missing_table_counts':dict(counts),'observation_counts':dict(collections.Counter(x['classification'] for x in out)),'observations':out}
(live/(name+'.probe-diagnostics.json')).write_text(json.dumps(proof,indent=2)+'\n');print(name,proof['observation_counts'])
