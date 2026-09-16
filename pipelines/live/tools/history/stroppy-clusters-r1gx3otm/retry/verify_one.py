import sys,os,json,datetime as dt,subprocess,importlib.util
from pathlib import Path
P=Path(sys.argv[1]);name=sys.argv[2];outname=sys.argv[3]
LIVE=Path('/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live')
CLI='/tmp/stroppy-clusters-r1gx3otm/bin/graphenectl'
os.environ['PATH']=str(Path(CLI).parent)+os.pathsep+os.environ['PATH']
sys.dont_write_bytecode=True
S=json.loads((P/'state.json').read_text());cell=S['cells'][name]
def mod(name):
 spec=importlib.util.spec_from_file_location(name,LIVE/(name+'.py'));m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m);return m
def write(path,d):
 temp=path.with_suffix(path.suffix+'.tmp');temp.write_text(json.dumps(d,indent=2)+'\n');os.replace(temp,path)
result=json.loads((P/(name+'.result.json')).read_text())
inputcell=next(x for x in json.loads((P/'suite.json').read_text())['cells'] if x['id']==name)['run_spec']
end=dt.datetime.now(dt.timezone.utc).isoformat()
diagnostic='--allow-workload-errors' in sys.argv
n=mod('inspect_native_metrics').inspect(CLI,cell['run_id'],result,require_zero_errors=not diagnostic)
m=mod('inspect_metrics').inspect(cell['run_id'],'t-stroppy-live',start=S['started_at'],end=end,entities=['docker/'+cell['uuid']+'-'+c['name'] for c in inputcell['containers'] if c.get('scrape')])
expected_exporters=sum(bool(c.get('scrape')) for c in inputcell['containers'])
expected_disks=sum(len(c.get('disks',[])) for c in inputcell['machines'])
assert len(m['exporters'])==expected_exporters,(len(m['exporters']),expected_exporters)
assert len(m['data_filesystems'])==expected_disks
assert m['exporter_runs_observed']==[cell['run_id']]
health_end=(dt.datetime.fromisoformat(result['segments'][-1]['finished_at'].replace('Z','+00:00'))+dt.timedelta(seconds=15)).isoformat()
workload_health=mod('inspect_metrics').inspect(cell['run_id'],'t-stroppy-live',start=result['segments'][0]['started_at'],end=health_end,entities=['docker/'+cell['uuid']+'-'+c['name'] for c in inputcell['containers'] if c.get('scrape') and c['name'].endswith(('-mysqld-exporter','-proxysql'))])
m['workload_observation']=workload_health
collectors=workload_health['mysql_exporter_collectors']
assert collectors and all(row['min']==1 for row in collectors),collectors
m['health_scope_note']='Health assertions cover workload start through 15 seconds after its last segment. Full-run samples also include planned database shutdown during cleanup.'
health=[v['mysql_up'] for v in workload_health['component_health'].values() if 'mysql_up' in v]
expected_dbs=sum(c['name'].endswith(('-mysql','-mariadb')) for c in inputcell['containers'])
assert len(health)==expected_dbs and all(h['min']==1 for h in health),health
l=mod('inspect_component_logs').inspect(cell['run_id'],'t-stroppy-live','http://127.0.0.1:19428')
assert len(l['component_logs'])==len(inputcell['containers']),l['component_logs']
t=mod('inspect_traces').inspect(cell['run_id'],'t-stroppy-live','http://127.0.0.1:18043/select/jaeger',S['started_at'],end)
assert t['trace_count']>0 and t['matching_spans']>0
artifacts=[]
now=dt.datetime.now(dt.timezone.utc)
for ref in result['artifacts']:
 r=json.loads(subprocess.check_output([CLI,'-n','t-stroppy-live','get',ref,'--jq','.resource'],text=True,timeout=30))
 assert r['phase']=='ready' and r['owner']=='stand/stroppy-run'
 state=r['state'];keep=state.get('keepUntil')
 if ref.endswith('-config'):assert not keep or keep.startswith('0001-')
 else:
  assert ref.endswith('-log') and keep
  assert 29*86400<(dt.datetime.fromisoformat(keep.replace('Z','+00:00'))-now).total_seconds()<31*86400
 artifacts.append({'ref':ref,'owner':r['owner'],'keepUntil':keep,**state['blob']})
assert len(artifacts)==len(result['segments'])*2
write(P/(name+'.blobs.json'),artifacts)
logsdir=P/(name+'-artifacts');logsdir.mkdir(mode=0o700,exist_ok=True)
r=subprocess.run(['/tmp/stroppy-patroni-017zezx1/download-logs',str(P/(name+'.blobs.json')),str(logsdir)],capture_output=True,text=True,check=True)
(P/(name+'.downloads.json')).write_text(r.stdout)
for suffix,body in [('result',result),('metrics',m),('native',n),('logs',l),('traces',t),('artifacts',artifacts)]:write(LIVE/(outname+'.'+suffix+'.json'),body)
write(LIVE/(outname+'.replication-metrics.json'),mod('inspect_mysql_cluster_metrics').inspect(m))
write(P/(name+('.diagnostic.json' if diagnostic else '.verified.json')),{'status':'export_verified_with_workload_errors' if diagnostic else 'passed','name':outname,'run_id':cell['run_id'],'checked_at':end,'native':n,'components':m,'logs':l,'traces':t,'artifacts':artifacts})
print(outname,len(n['segments']),'native segments,',len(artifacts),'downloaded artifacts, metrics/logs/traces passed')

if not diagnostic:
 subprocess.run(['python3','/tmp/stroppy-mysql-vnafdcv2/verify_progress.py',cell['run_id'],str(LIVE/(outname+'.load-progress.json'))],check=True,timeout=40)
 subprocess.run(['python3','/tmp/stroppy-clusters-r1gx3otm/aggregate.py',str(P)],check=True)
