# Отмена multi-machine deploy на Graphene 0.2.27

**Пройдено:** run `421dd494-143c-43b0-8d37-62b0a4811c73` отменён через сервер
в фазе `deploying`, после готовности всех пяти машин MySQL Group Replication
+ ProxySQL. Завершился `cancelled / done`; сегменты нагрузки не запускались.
Начало 2026-09-27 19:04:29 UTC, завершение 19:13:47 UTC.

[Состояние перед отменой](before-cancel.json),
[дерево готовой инфраструктуры](tree-before-cancel.json),
[проверка жизненного цикла](verification.json), [cleanup](cleanup.json).
Проверяется именно отмена развёртывания, а не SQL или failover.

После удаления инфраструктуры server admin/status показывает
running=0, pending=0, kept_stands=0. Свежие квоты вернулись к исходным
16 VM / 3 сети: [статус](final-status.json), [квоты](final-quotas.json).
Все операции и наблюдения выполнены через API Stroppy Cloud; прямого
обращения к YC, Kubernetes или хранилищу телеметрии не было.

Запуск воспроизводится `live/tools/server_cancel_deploy.py`; доказательства
проверяются `live/tools/record_cancel_deploy_checks.py` и
`live/tools/verify_server_cleanup.py`. Запись CSV находится в
`mysql/8.4/group-proxysql/lifecycle/server-cancel-deploy`.
