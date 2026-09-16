import subprocess,json,base64,datetime
from pathlib import Path
P=Path(__file__).parent;LIVE=Path('/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live');CLI=str(P/'bin/graphenectl')
def cli(*args):return subprocess.check_output([CLI,'-n','t-stroppy-live',*args,'--jq','.'],text=True,timeout=30)
d=json.loads(cli('get','stand/stroppy-run'));s=d['resource']['state'];hold=s['holdings'];before=json.loads((P/'stand-holdings-before212.json').read_text());assert set(before['expected'])<=set(hold)
for ref,h in before['initial'].items():assert hold[ref]==h,(ref,hold[ref],h)
for ref,h in before['expected'].items():
 assert hold[ref].get('from')==h.get('from')
 if 'retention_bounded' in h:assert bool(hold[ref].get('keepUntil'))==h['retention_bounded']
art=json.loads(cli('get','artifact','--owner','stand/stroppy-run'))['resources'];old=json.loads((P/'artifacts-before212.json').read_text())['resources'];assert {r['ref'] for r in old}<={r['ref'] for r in art}
events=[json.loads(x)['raw'] for x in cli('events','stand','stroppy-run').splitlines() if x]
start=events[0]['workflowExecutionStartedEventAttributes'];raw=base64.b64decode(start['input']['payloads'][0]['data']);envelope=json.loads(raw)
assert len(raw)<2097152
cache=envelope.get('completed',{});assert cache and not any('holdings' in r.get('result',{}) for r in cache.values())
report={'status':'passed','checked_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'holdings_before':len(before['expected']),'holdings_after':len(hold),'exact_initial_deadlines_preserved':len(before['initial']),'artifact_records_before':len(old),'artifact_records_after':len(art),'resume_payload_bytes':len(raw),'cached_responses':len(cache),'cached_response_bytes':len(json.dumps(cache).encode()),'retention':'all original holdings, origin refs and retention modes preserved; exact initial deadlines unchanged','stand_phase':d['resource']['phase']}
(LIVE/'graphene212-stand-recovery.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))
