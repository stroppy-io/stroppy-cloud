import json,pathlib,datetime,importlib.util,time,sys
root=pathlib.Path('pipelines/live')
def module(name):
 s=importlib.util.spec_from_file_location(name,root/('inspect_'+name+'.py'));m=importlib.util.module_from_spec(s);s.loader.exec_module(m);return m
metrics=module('metrics');traces=module('traces');checks=[]
for cell in json.loads((root/'suite-postgres-versions.json').read_text())['cells']:
 name=cell['id'];result_path=root/(name+'.result.json')
 if len(sys.argv)>1 and name not in sys.argv[1:]:continue
 if not result_path.exists():continue
 if name in ['pg18-pgbouncer','pg15-single','pg17-single']:parent='matrix-pg-versions-ed460e6f'
 elif name in ['pg16-single','pg18-single','pg15-primary-replica']:parent='matrix-pg-retry-daaf3554'
 else:parent='matrix-pg-final-951a642e'
 rid=parent+'-'+name;result=json.loads(result_path.read_text());segment=result['segments'][0]
 assert segment['status']=='completed' and segment['exit_code']==0,(name,segment['status'])
 def date(s):return datetime.datetime.fromisoformat(s.replace('Z','+00:00'))
 start=(date(segment['started_at'])-datetime.timedelta(minutes=15)).isoformat();end=min(date(segment['finished_at'])+datetime.timedelta(minutes=20),datetime.datetime.now(datetime.timezone.utc)).isoformat()
 for attempt in range(3):
  try:
   m=metrics.inspect(rid,'t-stroppy-live',endpoint='http://127.0.0.1:18428',start=start,end=end)
   t=traces.inspect(rid,'t-stroppy-live','http://127.0.0.1:18043/select/jaeger',start,end);break
  except Exception:
   if attempt==2:raise
   time.sleep(2)
 db_count=2 if 'replica' in name else 1;exporters=5 if 'replica' in name else 6 if 'pgbouncer' in name else 3
 assert m['exporter_runs_observed']==[rid],(name,'run isolation',m['exporter_runs_observed'])
 assert len(m['exporters'])==exporters,(name,'exporters',len(m['exporters']))
 assert len(m['data_filesystems'])==db_count,(name,'data disks',m['data_filesystems'])
 assert all(f['size_bytes']>90*2**30 for f in m['data_filesystems']),(name,'disk size')
 assert len(m['database_versions'])==db_count,(name,'database versions')
 assert all(v['short_version'].startswith(name[2:4]+'.') for v in m['database_versions'].values()),(name,'wrong version')
 health=[v for values in m['component_health'].values() for v in values.values()]
 assert len(health)==db_count+int('pgbouncer' in name),(name,'health samples')
 assert all(v['min']==1 and v['max']==1 for v in health),(name,'health',health)
 assert len(m['postgres_exporter_scrape_error_max'])==db_count and all(v==0 for _,v in m['postgres_exporter_scrape_error_max']),(name,'scrape errors')
 assert t['matching_spans']>0,(name,'traces missing')
 for kind,data in [('metrics',m),('traces',t)]: (root/(name+'.'+kind+'.json')).write_text(json.dumps(data,indent=2)+'\n')
 checks.append({'cell':name,'run_id':rid,'iterations':result['metrics']['smoke.iterations_total']['value'],'exporters':exporters,'data_disks':db_count,'database_versions':sorted({v['short_version'] for v in m['database_versions'].values()}),'traces':t['trace_count'],'spans':t['matching_spans'],'error_spans':t['error_spans']})
 print(name,'verified',checks[-1],flush=True)
p=pathlib.Path('/tmp/stroppy-pg-matrix-czazco7o/telemetry-validation.json');previous=json.loads(p.read_text()) if p.exists() else [];combined={x['cell']:x for x in previous};combined.update({x['cell']:x for x in checks});p.write_text(json.dumps(list(combined.values()),indent=2))
