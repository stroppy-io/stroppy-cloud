import json,pathlib,subprocess,base64,sys
root=pathlib.Path("pipelines/live");exe=sys.argv[1];s=json.loads((root/"pg17-patroni-ha.result.json").read_text())["segments"][0]
flags=["--start",s["started_at"],"--end",s["finished_at"],"-o","json"]
m=json.loads((root/"pg17-patroni-ha.metrics.json").read_text());entities=sorted(m["exporters"]);proof=[]
for entity in entities:
 r=subprocess.run([exe,"-n","t-stroppy-live","metrics",entity,*flags],capture_output=True,text=True,timeout=45)
 assert r.returncode==0,r.stderr[:500]
 data=json.loads(base64.b64decode(json.loads(r.stdout)["snapshot"]))
 rows=data["data"]["result"];assert rows,(entity,"empty metrics")
 assert {x["metric"].get("graphene.entity") for x in rows}=={entity}
 assert {x["metric"].get("graphene.run") for x in rows}=={"matrix-patroni-96794a09-pg17-patroni-ha"}
 proof.append({"entity":entity,"series":len(rows),"snapshot_bytes":len(base64.b64decode(json.loads(r.stdout)["snapshot"]))})
 print(entity,"verified",len(rows),flush=True)
r=subprocess.run([exe,"-n","t-stroppy-live","metrics","run/matrix-patroni-96794a09-pg17-patroni-ha",*flags],capture_output=True,text=True,timeout=45)
assert r.returncode!=0 and "exceeds 8 MiB" in r.stderr and not r.stdout,(r.returncode,r.stderr[:200],len(r.stdout))
p=root/"graphene-metrics-read-check.json";report=json.loads(p.read_text());report.update(deployment_verified=True,resource_queries=proof,oversize_live_check={"exit_code":r.returncode,"stdout_bytes":len(r.stdout),"error":"metrics backend response exceeds 8 MiB"});p.write_text(json.dumps(report,indent=2)+"\n")
print("All 18 component queries complete; oversized run returns explicit error")
