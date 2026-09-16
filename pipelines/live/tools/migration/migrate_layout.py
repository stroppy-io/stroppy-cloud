"""One-time offline migration of the flat live directory; never contacts YC."""
import argparse
import csv
import hashlib
import json
import os
import re
import shutil
from collections import defaultdict
from pathlib import Path

FIELDS = ('database', 'version', 'topology', 'workload', 'preset')
CHECKS = {
    'deployment': ['compile', 'infrastructure_provisioning'],
    'workload_check': ['smoke'],
    'native_metrics': ['native_tps', 'native_otlp_metrics'],
    'telemetry': ['component_metrics', 'component_logs', 'pipeline_traces', 'database_metric_distributions', 'managed_database_metrics'],
    'artifacts': ['artifacts'], 'replication': ['replication'], 'cleanup': ['cleanup'],
}

def write(path, obj):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(obj, indent=2, ensure_ascii=False) + '\n')

def slug(s):
    return re.sub(r'[^a-zA-Z0-9_.-]+', '-', str(s)).strip('-') or 'unspecified'

def case_path(row):
    return Path(*(slug(row[k].replace('2m0s', '2m')) for k in FIELDS))

def objects(value, pointer=''):
    if isinstance(value, dict):
        yield value, pointer
        for key, child in value.items():
            yield from objects(child, pointer + '/' + key.replace('~','~0').replace('/','~1'))
    elif isinstance(value, list):
        for i, child in enumerate(value):
            yield from objects(child, pointer + '/' + str(i))

def main(source, target):
    if target.exists():
        raise SystemExit('Output must be a new directory')
    target.mkdir(parents=True)
    rows = list(csv.DictReader((source/'progress.csv').open()))
    docs = {str(p.relative_to(source)): json.loads(p.read_text()) for p in source.rglob('*.json')}
    byrun = defaultdict(list)
    for row in rows:
        byrun[row['run_id']].append(row)
    owners = {rid: sorted(rs, key=lambda r:(r['workload']!='simple',str(case_path(r))))[0] for rid,rs in byrun.items()}
    prefix = {}
    record_by_case = {}
    for name, doc in docs.items():
        if not name.endswith('-check.json'):
            continue
        for d, ptr in objects(doc):
            rid = d.get('run_id')
            if rid not in byrun:
                continue
            if d.get('evidence_prefix'):
                prefix[d['evidence_prefix']] = rid
            for row in byrun[rid]:
                if name != row['evidence']:
                    continue
                if d.get('workload') and d['workload'] != row['workload']:
                    continue
                if d.get('database') and d['database'] != row['database']:
                    continue
                key = str(case_path(row))
                if key not in record_by_case or '/native_cells/' in ptr:
                    record_by_case[key] = (name, ptr, d)
    suffixes = ('.metrics.json','.logs.json','.traces.json','.native.json','.replication.json','.artifacts.json','.result.json','.components.json','.load-progress.json')
    for name,d in docs.items():
        if not isinstance(d,dict) or d.get('run_id') not in byrun:
            continue
        for suffix in suffixes:
            if name.endswith(suffix):prefix[name[:-len(suffix)]] = d['run_id']
    # Match the input UUID, cell suffix and accepted metadata, never list order.
    inputs = {}
    for row in rows:
        ref = row['input']; d = docs.get(ref, {})
        if 'cells' not in d and 'workload' in d:
            inputs[str(case_path(row))] = (ref, '', d)
        elif 'cells' in d:
            proof = record_by_case.get(str(case_path(row)), ('','',{}))[2]
            candidates = [(i,c) for i,c in enumerate(d['cells']) if row['run_id'].endswith('-'+c['id']) or (proof.get('uuid') and c['run_spec'].get('run_id')==proof['uuid'])]
            if len(candidates)==1:
                i,c = candidates[0];inputs[str(case_path(row))]=(ref,f'/cells/{i}/run_spec',c['run_spec'])
    mapping = {}; kinds = {}
    def assign(old, new, kind):
        mapping[old]=str(new);kinds[old]=kind
    for p in source.rglob('*'):
        if not p.is_file():continue
        old=str(p.relative_to(source));name=p.name
        if old=='progress.csv':continue
        if name=='README.md':assign(old,Path('platform/history/README.md'),'historical documentation');continue
        if p.suffix=='.py':assign(old,Path('tools')/name,'tool');continue
        if p.suffix=='.yaml':assign(old,Path('bootstrap')/name,'bootstrap manifest');continue
        if p.suffix=='.txt':assign(old,Path('tools/reference')/name,'source reference');continue
        matched = next((key for key in sorted(prefix,key=len,reverse=True) if old.startswith(key+'.')),None)
        if matched:
            rid=prefix[matched];base=case_path(owners[rid])/'runs'/rid
            assign(old,base/name[len(matched)+1:],'run evidence');continue
        if old.startswith('matrix/') and name!='inventory.json':
            found=[r for r in rows if (r['database'],r['version'],r['topology'])==next(((x['database'],x['version'],x['topology']) for x in docs['matrix/inventory.json'] if x.get('input')==name),None)]
            if found:
                assign(old,case_path(found[0])/'template.json','compiled template');continue
        d=docs.get(old,{})
        if isinstance(d,dict) and d.get('run_id') in owners:
            rid=d['run_id'];assign(old,case_path(owners[rid])/'runs'/rid/name,'run evidence');continue
        if name.startswith('suite-'):
            assign(old,Path('platform/suites')/name,'shared suite');continue
        category = ('catalog' if name in ('catalog-functional-check.json','mysql-family-check.json','mysql-clusters-check.json','postgres-matrix-check.json','patroni-matrix-check.json','native-export-check.json','current-campaigns.json') or old=='matrix/inventory.json' else
                    'credentials' if any(x in name for x in ('credential','token','kubeconfig','bootstrap','access','auth','verify-','missing-cred')) else
                    'quotas' if any(x in name for x in ('quota','provider')) else
                    'images' if any(x in name for x in ('image','release','rollout','worker')) else
                    'telemetry' if any(x in name for x in ('metric','otel','otlp','log','trace','native')) else
                    'lifecycle' if any(x in name for x in ('cleanup','retention','artifact','blob','recovery','keep','cancel','disk','ownership','provision','heartbeat')) else 'history')
        # Historical attempts outside the current CSV remain under their database
        # when an existing evidence prefix determines the configuration.
        normalized=name.replace('maria','mariadb',1) if name.startswith('maria') and not name.startswith('mariadb') else name
        family = next((k for k in sorted(prefix,key=len,reverse=True) if normalized.startswith(k+'-')),None)
        if family:
            owner=owners[prefix[family]]
            assign(old,case_path(owner)/'history'/name,'historical attempt (not current acceptance)')
        else:assign(old,Path('platform')/category/name,'shared or unclassified evidence')
    assert len(set(mapping.values()))==len(mapping),'destination collision'
    inventory=[]
    for old,new in mapping.items():
        dest=target/new;dest.parent.mkdir(parents=True,exist_ok=True);shutil.copy2(source/old,dest)
        inventory.append({'original':old,'path':new,'sha256':hashlib.sha256((source/old).read_bytes()).hexdigest(),'kind':kinds[old]})
    write(target/'tools/migration/paths.json',mapping)
    write(target/'tools/migration/files.json',inventory)
    write(target/'tools/migration/legacy-progress.json',rows)
    # The canonical payload is shared by all segment cases using relative symlinks.
    for rid,rs in byrun.items():
        base=case_path(owners[rid])/'runs'/rid
        (target/base).mkdir(parents=True,exist_ok=True)
        available=next((inputs[str(case_path(r))] for r in rs if str(case_path(r)) in inputs),None)
        manifest={'schema_version':1,'run_id':rid,'case_paths':[str(case_path(r)) for r in rs],
                  'execution':{'status':rs[0]['run_phase'].lower(),'observed_at':rs[0]['run_status_checked_at']},
                  'input_status':'public_snapshot' if available else 'unknown',
                  'note':'Public snapshot may omit private OTLP credentials. No credentials are restored during migration.'}
        if available:
            name,ptr,spec=available;write(target/base/'input.json',spec)
            manifest['input_source']={'path':mapping[name],'pointer':ptr}
        write(target/base/'manifest.json',manifest)
        for row in rs:
            alias=case_path(row)/'runs'/rid
            if alias!=base:
                (target/alias).parent.mkdir(parents=True,exist_ok=True)
                (target/alias).symlink_to(os.path.relpath(target/base,(target/alias).parent),target_is_directory=True)
    for row in rows:
        path=case_path(row); key=str(path);rid=row['run_id']
        name,ptr,record=record_by_case.get(key,(row['evidence'],'',{}))
        source_ref={'path':mapping[name],'pointer':ptr}
        checks=[]
        for group,names in CHECKS.items():
            for field in names:
                old=row.get(field,'');status={'pending':'not_run','not_recorded':'unknown','automatic_verified':'passed'}.get(old,old or 'unknown')
                reason='Imported accepted observation; see source record.'
                # A summary TPS placeholder must not suppress transactional checks.
                if field=='native_tps' and row['workload'] in ('tpcb/tx','tpcb/procs','tpcc/tx','tpcc/procs','baseline') and status=='not_applicable':status='unknown'
                if status=='not_applicable':reason='No logical transactions for this workload.' if field=='native_tps' else 'Not applicable to this database/topology; inherited scope is preserved.'
                if status=='not_run':reason='Not verified in the retained report.'
                if status=='unknown':reason='No explicit retained verification for this check.'
                if status=='passed' and field in ('component_logs','artifacts','replication','native_tps','native_otlp_metrics') and record.get(field)!='passed':
                    # Do not manufacture detailed evidence from a generic smoke success.
                    status='unknown';reason='Legacy CSV asserted passed; matching report does not explicitly confirm this field.'
                checks.append({'id':field,'group':group,'status':status,'reason':reason,'evidence':source_ref if status=='passed' else None,'legacy_status':old})
        write(target/path/'checks.json',{'schema_version':1,'run_id':rid,'checked_at':row['checked_at'],'checks':checks})
        case={'schema_version':1,'id':key,**{k:row[k].replace('2m0s','2m') if k=='preset' else row[k] for k in FIELDS},
              'provider':row['provider'],'zones':row['zone'].split(','),'acceptance_scope':'functional smoke and recorded telemetry checks; not full performance or fault readiness',
              'selected_run':rid,'execution':{'status':row['run_phase'].lower(),'observed_at':row['run_status_checked_at']},
              'checks':'checks.json','runs':['runs/'+rid], 'source':source_ref,
              'legacy_row':row,'input_status':'unknown'}
        if key in inputs:
            name,ptr,spec=inputs[key]
            write(target/path/'input.json',spec)
            case.update(input='input.json',input_status='public_snapshot',input_source={'path':mapping[name],'pointer':ptr})
            case['actual_segments']=[{'name':s['name'],'script':s.get('workload',{}).get('script'),'run':s.get('run',{}),'params':s.get('workload',{})} for s in spec.get('workload',{}).get('segments',[])]
            case['input_note']='Full original run input, potentially containing multiple workloads. Preset is the historical label; actual segment parameters take precedence.'
        write(target/path/'case.json',case)
    write(target/'platform/catalog/remaining-scope.json',{'status':'not_planned','required':['full workload parameter matrices','baseline matrix','segment matrix','fault campaigns'],'note':'Historical pending columns are retained here, not counted as completed or silently removed from full readiness.'})
    # Drafts have never run. Add explicit plan rows, without borrowed results.
    for filename in ('suite-pg-native-final.json','suite-pg-soak-5h.json'):
        doc=docs[filename]
        for i,cell in enumerate(doc['cells']):
            match=re.match(r'pg(\d+)-(.+)-(?:native|tpcc-5h)$',cell['id']);assert match,cell['id']
            version,topology=match.groups();spec=cell['run_spec'];seg=spec['workload']['segments'][0]
            row={'database':'postgres','version':version,'topology':topology,'workload':seg['workload']['script'],'preset':'soak-5h' if 'soak' in filename else 'native-final-2m-2vus'}
            path=case_path(row);ref={'path':mapping[filename],'pointer':f'/cells/{i}/run_spec'}
            assert not (target/path/'case.json').exists()
            write(target/path/'input.json',spec)
            checks=[{'id':n,'group':g,'status':'not_applicable' if n=='native_tps' and row['workload']=='simple' else 'not_run','reason':'Draft only; token fix preceded launch planning.','evidence':None} for g,ns in CHECKS.items() for n in ns]
            write(target/path/'checks.json',{'schema_version':1,'run_id':None,'checked_at':None,'checks':checks})
            write(target/path/'case.json',{'schema_version':1,'id':str(path),**row,'provider':'yandex','zones':sorted({m['location'] for m in spec['machines']}),'selected_run':None,'execution':{'status':'not_started','observed_at':None},'checks':'checks.json','runs':[],'input':'input.json','input_status':'draft','source':ref,'acceptance_scope':'draft; requires replanning before launch'})
    print(json.dumps({'cases':len(list(target.glob('*/*/*/*/*/case.json'))),'accepted_runs':len(byrun),'matched_case_inputs':len(inputs),'original_files':len(mapping)},indent=2))

if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source',type=Path,required=True);parser.add_argument('--output',type=Path,required=True)
    args=parser.parse_args();main(args.source,args.output)
