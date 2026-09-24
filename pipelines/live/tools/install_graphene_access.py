#!/usr/bin/env python3
"""Install the Graphene worker identity. No tenant kubeconfig or static token."""
import argparse
import json
import subprocess
from pathlib import Path

import yaml

p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--kubeconfig', required=True)
p.add_argument('--apply', action='store_true')
a = p.parse_args()
k = ['kubectl', '--kubeconfig', a.kubeconfig, '--request-timeout=30s']
manifest = Path(__file__).parent / 'bootstrap/graphene-crossplane.yaml'
d = json.loads(subprocess.check_output(k + ['-n', 'graphene', 'get', 'deployment', 'graphene-server', '-o', 'json']))
tracking = d['metadata'].get('annotations', {}).get('argocd.argoproj.io/tracking-id')
if tracking and a.apply:
    raise SystemExit(f'ArgoCD owns this deployment ({tracking}); update its Helm chart in stroppy-io/cloud instead of patching it')
containers = d['spec']['template']['spec']['containers']
container = next(c for c in containers if any(e['name'] == 'GRAPHENE_MANAGED_POD_TEMPLATE' for e in c.get('env', [])))
entry = next(e for e in container['env'] if e['name'] == 'GRAPHENE_MANAGED_POD_TEMPLATE')
if 'valueFrom' in entry:
    raise SystemExit('pod template comes from a reference; update its configuration source')
template = yaml.safe_load(entry.get('value', '')) or {}
previous = template.get('serviceAccountName', 'default')
if previous not in ('default', 'graphene-crossplane'):
    raise SystemExit(f'worker already uses a different identity: {previous}; review before replacing it')
template['serviceAccountName'] = 'graphene-crossplane'
template['automountServiceAccountToken'] = True
value = yaml.safe_dump(template, sort_keys=False)
print(json.dumps({'worker_service_account_before': previous, 'worker_service_account_after': 'graphene-crossplane', 'credential_namespace': 'stroppy-provider-credentials', 'applied': a.apply}))
if a.apply:
    subprocess.run(k + ['apply', '-f', str(manifest)], check=True)
    # JSON Patch tests avoid replacing a pod template concurrently edited by an operator.
    ci = containers.index(container)
    ei = container['env'].index(entry)
    path = f'/spec/template/spec/containers/{ci}/env/{ei}/value'
    patch = [{'op': 'test', 'path': path, 'value': entry['value']}, {'op': 'replace', 'path': path, 'value': value}]
    subprocess.run(k + ['-n', 'graphene', 'patch', 'deployment', 'graphene-server', '--type=json', '-p', json.dumps(patch)], check=True)
