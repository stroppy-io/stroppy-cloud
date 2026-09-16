import pathlib,json,re
root=pathlib.Path("pipelines/live");proof=[]
for c in json.loads((root/"suite-postgres-patroni.json").read_text())["cells"]:
 name=c["id"];p=pathlib.Path(__file__).parent/"native-logs"/(c["run_spec"]["run_id"][:8]+"-stroppy-smoke-log.log");s=p.read_text();r=json.loads((root/(name+".result.json")).read_text());v=float(re.search(r"iterations_total\s+([0-9.]+)",s).group(1));assert v==r["metrics"]["smoke.iterations_total"]["value"]
 assert not re.search(r"(?i)level[=:]error|failed_iterations_total\s+[1-9]|iteration_errors_total\s+[1-9]",s)
 proof.append({"cell":name,"iterations_total_in_native_log":v,"matches_pipeline_result":True,"log_bytes":p.stat().st_size,"log_lines":len(s.splitlines()),"explicit_failed_iterations_counter_present":bool(re.search(r"failed_iterations_total",s)),"segment_exit_code":r["segments"][0]["exit_code"]})
(root/"patroni-native-summary-check.json").write_text(json.dumps({"checks":proof,"scope":"native log summary counters and pipeline exit status; no inferred TPS or inferred failure counter"},indent=2)+"\n")
print("native iteration counters match all four results")
