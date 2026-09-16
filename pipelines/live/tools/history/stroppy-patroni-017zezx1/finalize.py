import json,pathlib,subprocess,datetime
root=pathlib.Path("pipelines/live");work=pathlib.Path(__file__).parent
suite=json.loads((root/"suite-postgres-patroni.json").read_text());parent=(work/"run-id").read_text();result=json.loads((root/"suite-postgres-patroni.result.json").read_text())
assert result["total"]==result["done"]==4 and result["failed"]==0,result
checks={x["cell"]:x for x in json.loads((work/"telemetry-validation.json").read_text())};cells=[]
for c in suite["cells"]:
 name=c["id"];rid=parent+"-"+name;spec=c["run_spec"];expected={"docker/"+spec["run_id"]+"-"+x["name"] for x in spec["containers"]}
 def read(kind):return json.loads((root/(name+"."+kind+".json")).read_text())
 metrics,logs,sql,agents=read("metrics"),read("logs"),read("replication"),read("agents")
 assert expected==set(metrics["exporters"])==set(logs["component_logs"]),(name,"incomplete component coverage")
 assert logs["runs_observed"]==[rid] and logs["namespaces_observed"]==["t-stroppy-live"]
 assert len(agents)==8 and all(x["observed_connection_seconds"]>120 and x["disconnect_events"]==0 for x in agents.values())
 assert len(sql["nodes"])==3 and all(n[1]["rows"]==[["100"]] for n in sql["nodes"].values())
 tree=json.loads(subprocess.check_output(["graphenectl","-n","t-stroppy-live","tree","run/"+rid,"--jq","."],timeout=30));assert tree=={},(name,tree)
 cells.append(dict(checks[name],logs=logs["records"],log_components=len(logs["component_logs"]),sql_replication_verified=True,agents=len(agents),agent_reconnections=0,ownership_tree=tree))
cleanup=json.loads((root/"patroni.cleanup.json").read_text());assert not any(cleanup["remaining_test_resources"].values())
artifacts=json.loads((root/"patroni.artifacts.json").read_text());assert len(artifacts["download_checks"])==8 and all(x["download_verified"] for x in artifacts["download_checks"])
report={"checked_at":datetime.datetime.now(datetime.timezone.utc).isoformat(),"status":"passed","scope":"PostgreSQL 15-18 Patroni HA, simple smoke, 2m, 2 VUs, real YC ru-central1-a","suite_run_id":parent,"total":4,"validated":4,"failed":0,"server_during_runs":"0.2.8","topology":{"postgresql":3,"etcd":3,"haproxy":1,"runner":1,"postgres_data_disk_gib":100,"etcd_data_disk_gib":20},"cells":cells,"artifact_downloads_verified":8,"cleanup_verified":True,"retention":{"configs":"until explicit deletion","artifact_logs":"30d","shared_metrics":"90d","shared_logs":"30d","shared_traces":"14d"},"tps":"not computed by pipeline; native Stroppy TPS export still pending","traces_scope":"pipeline spans; workload OTLP is not validated","remaining_scope":["MySQL Group Replication, MariaDB Galera, Managed YDB implementation","other catalog version/topology smoke matrices excluding AWS","all workload, baseline and segment matrices","native Stroppy TPS and workload OTLP exports","fault campaigns after successful positive matrices"]}
pods=json.loads(subprocess.check_output(["kubectl","--kubeconfig","/home/yaroher/devel/github/stroppy-io/cloud/.config/yc/kubeconfig","-n","graphene","get","pods","-l","app=graphene-server","-o","json"],timeout=30))
servers=[]
for pod in pods["items"]:
 for c in pod["status"].get("containerStatuses",[]):
  if c["name"]=="server":servers.append({"pod":pod["metadata"]["name"],"image":c["image"],"image_id":c["imageID"],"ready":c["ready"],"restarts":c["restartCount"]})
assert len(servers)==1 and servers[0]["ready"] and servers[0]["restarts"]==0 and servers[0]["image"].endswith(":0.2.8"),servers
report["server_before_post_matrix_rollout"]=servers[0]
report["local_checks"]=["root go test ./...", "pipelines go test ./...", "Graphene make test and make lint", "local Patroni SQL through RW and RO HAProxy ports"]
(root/"patroni-matrix-check.json").write_text(json.dumps(report,indent=2)+"\n");print("Patroni live matrix verified 4/4")
