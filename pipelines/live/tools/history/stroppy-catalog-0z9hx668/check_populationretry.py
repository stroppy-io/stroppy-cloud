from pathlib import Path
import subprocess
p=Path(__file__).parent;r='/home/yaroher/devel/github/stroppy-io/stroppy'
for target in ['linter_fix','linter','tests']:
 with (p/('populationretry-'+target+'.log')).open('w') as log:
  rc=subprocess.run(['make',target],cwd=r,stdout=log,stderr=subprocess.STDOUT).returncode
 print(target,rc,flush=True)
 if rc:raise SystemExit(rc)
subprocess.run(['python3',str(p/'build_native.py')],check=True)
compose=['docker','compose','-f',str(p/'stroppy-native/compose.yml'),'-p','stroppy-catalog-native']
try:
 subprocess.run(compose+['up','-d','--wait'],check=True)
 with (p/'stroppy-native/populationretry-integration.log').open('w') as log:rc=subprocess.run(['make','integration'],cwd=r,stdout=log,stderr=subprocess.STDOUT).returncode
 print('integration',rc,flush=True)
 if rc:raise SystemExit(rc)
finally:subprocess.run(compose+['down','--volumes'],check=True)
import json
m=json.loads((p/'stroppy-native/release-source.json').read_text());tag='registry.stroppy.io/stroppy-io/stroppy:dev-native-'+m['binary_sha256'][:12]
with (p/'stroppy-native/populationretry-image-build.log').open('w') as log:subprocess.run(['docker','build','-t',tag,str(p/'stroppy-native/image-build')],check=True,stdout=log,stderr=subprocess.STDOUT)
subprocess.run(['python3',str(p/'publish_native.py'),'populationretry'],check=True)
