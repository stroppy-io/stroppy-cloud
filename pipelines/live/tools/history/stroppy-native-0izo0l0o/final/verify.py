import datetime as dt
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys

P = Path(__file__).parent
LIVE = Path('/home/yaroher/devel/github/stroppy-io/stroppy-cloud/pipelines/live')
CLI = '/tmp/stroppy-native-0izo0l0o/release-0210/graphenectl'
os.environ['PATH'] = str(Path(CLI).parent) + os.pathsep + os.environ['PATH']
S = json.loads((P / 'state.json').read_text())

def module(name):
    spec = importlib.util.spec_from_file_location(name, LIVE / (name + '.py'))
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod

def write(path, data):
    path.write_text(json.dumps(data, indent=2) + '\n')

native = module('inspect_native_metrics')
metrics = module('inspect_metrics')
traces = module('inspect_traces')
logs = module('inspect_component_logs')
now = dt.datetime.now(dt.timezone.utc)
end = now.isoformat()
reports = {}
instances = set()
artifact_refs = []
for name, cell in S['cells'].items():
    result = json.loads((P / (name + '.result.json')).read_text())
    assert 'segments' in result, list(result)
    n = native.inspect(CLI, cell['run_id'], result, require_zero_errors=True)
    for seg in n['segments']:
        assert seg['instance'] not in instances
        instances.add(seg['instance'])
    write(LIVE / ('native-' + name + '.metrics.json'), n)
    print(name, 'native metrics passed', len(n['segments']), flush=True)
    m = metrics.inspect(cell['run_id'], 't-stroppy-live', start=S['started_at'], end=end)
    assert len(m['exporters']) == 3, m['exporters']
    assert len(m['data_filesystems']) == 1, m['data_filesystems']
    assert m['exporter_runs_observed'] == [cell['run_id']]
    assert all(value == 0 for _, value in m['postgres_exporter_scrape_error_max'])
    assert any(v.get('pg_up', {}).get('min') == 1 for v in m['component_health'].values())
    write(LIVE / ('native-' + name + '.components.json'), m)
    t = traces.inspect(cell['run_id'], 't-stroppy-live', 'http://127.0.0.1:18043/select/jaeger', S['started_at'], end)
    assert t['trace_count'] > 0 and t['matching_spans'] > 0
    write(LIVE / ('native-' + name + '.traces.json'), t)
    l = logs.inspect(cell['run_id'], 't-stroppy-live', 'http://127.0.0.1:19428')
    assert len(l['component_logs']) == 4, l['component_logs']
    write(LIVE / ('native-' + name + '.logs.json'), l)
    write(LIVE / ('native-' + name + '.result.json'), result)
    artifact_refs.extend(result['artifacts'])
    reports[name] = {'native': n, 'components': m, 'traces': t, 'logs': l}
    print(name, 'component metrics/traces/logs passed', flush=True)

artifacts = []
for ref in artifact_refs:
    r = json.loads(subprocess.check_output([CLI, '-n', 't-stroppy-live', 'get', ref, '--jq', '.resource'], text=True))
    assert r['phase'] == 'ready' and r['owner'] == 'stand/stroppy-run'
    state = r['state']
    keep = state.get('keepUntil')
    if ref.endswith('-config'):
        assert not keep or keep.startswith('0001-'), keep
    else:
        assert ref.endswith('-log') and keep
        expiry = dt.datetime.fromisoformat(keep.replace('Z', '+00:00'))
        assert 29 * 86400 < (expiry - now).total_seconds() < 31 * 86400
    artifacts.append({'ref': ref, 'owner': r['owner'], 'keepUntil': keep, **state['blob']})
assert len(artifacts) == 4 and len(set(artifact_refs)) == 4
write(P / 'blobs.json', artifacts)
(P / 'logs').mkdir(mode=0o700, exist_ok=True)
r = subprocess.run(['/tmp/stroppy-patroni-017zezx1/download-logs', str(P / 'blobs.json'), str(P / 'logs')], capture_output=True, text=True, check=True)
(P / 'blob-download-output.json').write_text(r.stdout)
print('Artifact downloads verified', len(artifacts), flush=True)

ledger = json.loads((P / 'ledger.json').read_text())
baseline = json.loads((P / 'baseline.json').read_text())
cleanup = {}
for kind, args in {'vm': ['compute', 'instance'], 'disk': ['compute', 'disk'], 'network': ['vpc', 'network'], 'subnet': ['vpc', 'subnet'], 'sg': ['vpc', 'security-group']}.items():
    current = json.loads(subprocess.check_output(['yc', *args, 'list', '--folder-id', 'b1ghttqg66t14ldkvfcq', '--format', 'json'], text=True))
    actual = {r['id'] for r in current}
    created = set(ledger[kind])
    if kind == 'disk':
        created.update(r['boot_disk_id'] for r in ledger['vm'].values())
    original = {r['id'] for r in baseline[kind]}
    assert not created & actual, (kind, 'test resources remain')
    assert actual == original, (kind, 'baseline changed')
    cleanup[kind] = {'created_ids': sorted(created), 'remaining_ids': [], 'baseline_count': len(original), 'current_count': len(actual), 'baseline_ids_unchanged': True}
write(P / 'verified.json', {'reports': reports, 'artifacts': artifacts, 'cleanup': cleanup, 'checked_at': end})
print('Native export, artifacts and exact YC cleanup verified', flush=True)
