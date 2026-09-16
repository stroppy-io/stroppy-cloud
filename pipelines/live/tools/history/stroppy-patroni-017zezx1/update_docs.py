import pathlib,json
root=pathlib.Path("pipelines/live");matrix=json.loads((root/"patroni-matrix-check.json").read_text());metrics=json.loads((root/"graphene-metrics-read-check.json").read_text())
assert matrix["status"]=="passed" and matrix["validated"]==4 and metrics["deployment_verified"]
p=root/"README.md";s=p.read_text();start=s.index("PostgreSQL 15–18 ×");end=s.index("Входы и результаты",start)
s=s[:start]+"""PostgreSQL 15–18 × single / primary-replica / PgBouncer / Patroni HA:
**16/16 успешных функциональных smoke-прогонов** в YC с отдельными SSD.
Первые 12 ячеек описаны в `postgres-matrix-check.json`; четыре Patroni HA —
в `patroni-matrix-check.json`. Все тестовые YC-ресурсы этих матриц удалены;
проверены скачивание и SHA-256 24 + 8 артефактов. Конфиги — до явного удаления,
логи — 30 дней; общий retention не менялся.

Patroni HA проверен на Graphene 0.2.8: три PostgreSQL, три etcd, HAProxy и
runner. В каждой ячейке подтверждены SQL-репликация, отдельные диски, метрики
и логи всех 18 компонентов, pipeline traces и отсутствие переподключений
агентов во время наблюдения нагрузки. Переключение при отказе отдельно не
проверялось. Двухминутный simple — функциональный smoke, не полный workload.

Текущий Graphene server — **0.2.9**, worker — `stroppy-run:166453f8f98a639b`.
Сервер обновлён после завершения Patroni suite; исправления чтения метрик
проверены через опубликованную CLI и реальный backend в
`graphene-metrics-read-check.json`. Полные workload/baseline-матрицы, другие
СУБД, native TPS/OTLP-экспорт Stroppy и отказные сценарии остаются следующими
этапами. AWS исключён.

"""+s[end:]
a=s.index("- Graphene server **0.2.5**");b=s.index("- Pipeline SDK",a)
s=s[:a]+"- Graphene server **0.2.9** (`aa39645`), bundled agent **0.2.1**; Argo CD\n  Synced/Healthy, cloud commit `"+metrics["deployment"]["infra_commit"][:8]+"`. Image digest:\n  `sha256:40131d93afc55e981a2fcd858fd1d984439931f88d5fdc3b4c76ba62c4b82017`.\n"+s[b:]
s=s.replace("| stroppy-run | 5d4c94c13c882225 |","| stroppy-run | 166453f8f98a639b |")
s+="\nИтог Patroni suite `"+matrix["suite_run_id"]+"`: **4/4 Completed**.\n\n| PostgreSQL | Итерации Stroppy | Метрики / логи компонентов | Pipeline spans |\n|---|---:|---:|---:|\n"
for c in sorted(matrix["cells"],key=lambda x:x["cell"]):s+="| "+c["cell"][2:4]+" | "+str(c["iterations"])+" | 18 / 18 | "+str(c["spans"])+" |\n"
s+="""
SQL-доказательства находятся в `pg*-patroni-ha.replication.json`,
непрерывность соединений агентов — в `pg*-patroni-ha.agents.json`.
`patroni.artifacts.json` проверяет восемь артефактов после завершения;
`patroni.cleanup.json` проверяет отсутствие VM, boot/data disks, сетей,
подсетей и security groups, включая две отменённые подготовительные попытки.
На протяжении четырёх итоговых прогонов Graphene 0.2.8 не перезапускался.
"""
p.write_text(s);print("current deployment and completed matrix documentation updated")
