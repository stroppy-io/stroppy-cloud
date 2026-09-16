import subprocess,json,time,pathlib,datetime
root=pathlib.Path("pipelines/live");work=pathlib.Path(__file__).parent
cells=[x["id"] for x in json.loads((root/"suite-postgres-patroni.json").read_text())["cells"]]
checked=set()
for _ in range(240):
 for name in cells:
  if name in checked or not (root/(name+".result.json")).exists():continue
  try:
   subprocess.run(["python3",str(work/"collect_telemetry.py"),name],check=True,timeout=180)
   subprocess.run(["python3",str(root/"inspect_component_logs.py"),"matrix-patroni-96794a09-"+name,"--endpoint","http://127.0.0.1:19428","--output",str(root/(name+".logs.json"))],check=True,timeout=60)
   logs=json.loads((root/(name+".logs.json")).read_text())
   assert len(logs["component_logs"])==18,(name,"missing component logs")
   checked.add(name);print(datetime.datetime.now(datetime.timezone.utc).isoformat(),"verified",name,flush=True)
  except Exception as e:print(name,"verify retry",str(e),flush=True)
 if len(checked)==len(cells):break
 time.sleep(30)
