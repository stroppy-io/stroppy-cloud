import subprocess,json,pathlib,datetime
p=pathlib.Path('/tmp/stroppy-catalog-0z9hx668/ydb-parallel');s=json.loads((p/'state.json').read_text());cli='/tmp/stroppy-catalog-0z9hx668/bin/graphenectl';print(datetime.datetime.now(datetime.timezone.utc).isoformat())
for n,c in s['cells'].items():
 r=subprocess.check_output([cli,'-n','t-stroppy-live','events','run',c['run_id'],'--jq','.'],text=True,timeout=30);(p/(n+'.events.jsonl')).write_text(r);a=[json.loads(x).get('raw',{}) for x in r.splitlines()];names=[x['activityTaskScheduledEventAttributes']['activityType']['name'] for x in a if 'activityTaskScheduledEventAttributes' in x];print(n,names[-1], 'failed',sum('activityTaskFailedEventAttributes' in x for x in a),'member-probes',len(list(p.glob(n+'-*.sql.json'))))
print('VMs',len(json.loads((p/'vm-current.json').read_text())))
print('errors',[(x.name,json.loads(x.read_text())) for x in p.glob('*.check-error.json')])
