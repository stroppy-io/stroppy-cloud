import json,datetime as dt
from pathlib import Path
P=Path(__file__).parent;LIVE=Path('/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live')
catalog={};native={};cleanup={}
entries=[(P,'single','mysql','8.4','single','suite-mysql84-single.json'),
 *[(P/'maria','maria'+v.replace('.','')+'-single','mariadb',v,'single','suite-mariadb-single.json') for v in ['11.8','11.4','10.11']],
 *[(P/'remaining','mysql'+v.replace('.','')+'-'+t,'mysql',v,t,'suite-mysql-remaining.json') for v,t in [('8.0','single'),('8.4','semi-sync'),('8.0','semi-sync')]],
 *[(P/'fixed-mysql','mysql'+v.replace('.','')+'-'+t,'mysql',v,t,'suite-mysql-fixed.json') for v,t in [('8.0','single'),('8.4','semi-sync'),('8.0','semi-sync')]],
 (P/'family211','mariadb1011-single','mariadb','10.11','single','suite-mysql-family-final.json'),
 *[(P/'family211','mysql'+v.replace('.','')+'-semi-sync','mysql',v,'semi-sync','suite-mysql-family-final.json') for v in ['8.4','8.0']],
 (P/'semisync2','mariadb1011-single','mariadb','10.11','single','suite-mysql-family-collision.json'),
 *[(P/'semisync2','mysql'+v.replace('.','')+'-semi-sync','mysql',v,'semi-sync','suite-mysql-family-collision.json') for v in ['8.4','8.0']],
 *[(P/'semisync','mysql'+v.replace('.','')+'-semi-sync','mysql',v,'semi-sync','suite-mysql-semisync.json') for v in ['8.4','8.0']],
 *[(P/'fixed-mariadb','mariadb'+v.replace('.','')+'-single','mariadb',v,'single','suite-mariadb-fixed.json') for v in ['11.8','11.4','10.11']]]
entries.append((P/'single-fixed','mysql84-single','mysql','8.4','single','suite-mysql84-single-fixed.json'))
for directory,name,db,v,topo,inputfile in entries:
 proof=directory/(name+'.verified.json')
 if not proof.exists():continue
 report=json.loads(proof.read_text());assert report['status']=='passed'
 if topo=='semi-sync':
  assert json.loads((LIVE/(report['name']+'.replication-metrics.json')).read_text())['status']=='passed'
  repl=json.loads((LIVE/(report['name']+'.replication.json')).read_text());assert repl['status']=='passed'
 state=json.loads((directory/'state.json').read_text());prefix=state['cells'][name]['uuid'][:8]
 ledger=json.loads((directory/'ledger.json').read_text());removed={}
 for k,items in ledger.items():
  ids={i for i,r in items.items() if prefix in r['name']}
  if k=='disk':
   for vm in ledger.get('vm',{}).values():
    if prefix in vm['name']:ids.add(vm['boot_disk_id'])
  current=json.loads((directory/(k+'-current.json')).read_text());assert not ids & {r['id'] for r in current},(name,k)
  removed[k]=sorted(ids)
 assert len(removed['vm'])>=2
 cleanup[report['run_id']]=removed
 runrecord=next(r for r in json.loads((directory/'runs.json').read_text()) if r['ref']=='run/'+report['run_id'])
 inputdata=json.loads((LIVE/inputfile).read_text());ins=next(c['run_spec'] for c in inputdata['cells'] if c['id']==name)
 for s in report['native']['segments']:
  workload=next(x['workload']['script'] for x in ins['workload']['segments'] if x['name']==s['segment'])
  metrics=s['measurements'];row={'status':'passed','database':db,'version':v,'topology':topo,'workload':workload,'segment':s['segment'],'run_id':report['run_id'],'iterations':metrics['iterations_total'],'native_tps':'passed' if 'tps' in metrics else 'not_applicable','native_otlp_metrics':'passed','component_metrics':'passed','component_logs':'passed','artifacts':'passed','replication':'passed' if topo=='semi-sync' else 'not_applicable','pipeline_traces':'passed','cleanup':'passed','input':inputfile,'checked_at':report['checked_at'],'image':ins['workload']['stroppy_image'],'evidence_prefix':report['name'],'started_at':runrecord['startedAt'],'finished_at':runrecord['finishedAt'],'database_actual_version':','.join(sorted({v['version'] for v in report['components']['database_versions'].values()}))}
  if workload=='simple':catalog[(db,v,topo)]=row
  else:
   row['preset']='native-otlp-20s-2vus-retry50' if workload.startswith('tpcc/') else 'native-otlp-20s-2vus'
   native[(db,v,topo,workload)]=row
fixed_image='docker.stroppy.io/stroppy-io/stroppy@sha256:118f585cd7ce621b35a44dae47f5897033f3462005ee0a9650bd5b4a21ea89da'
all_fixed=all(r['image']==fixed_image and (LIVE/(r['evidence_prefix']+'.load-progress.json')).exists() for r in catalog.values())
body={'status':'passed' if len(catalog)==7 and len(native)==28 and all_fixed else 'in_progress','checked_at':dt.datetime.now(dt.timezone.utc).isoformat(),'all_catalog_cells_on_corrected_native_image':all_fixed,'catalog_target':7,'catalog_validated':len(catalog),'catalog_cells':list(catalog.values()),'native_cells':list(native.values()),'cleanup_by_run':cleanup,'limitations':['Functional smoke: full workload, baseline and fault matrices remain pending.','Native Stroppy changes are uncommitted and use a pinned development image; no upstream PR or release yet.']}
(LIVE/'mysql-family-check.json').write_text(json.dumps(body,indent=2)+'\n')
print('Verified catalog',len(catalog),'native',len(native))
