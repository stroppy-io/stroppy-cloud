# Live-проверки пайплайнов

- [tests/progress.csv](tests/progress.csv) — единственная таблица прогресса.
- [tests/README.md](tests/README.md) — структура случаев, статусы и границы покрытия.
- [tools/README.md](tools/README.md) — все утилиты: подготовка, bootstrap, наблюдение, проверка и восстановленные скрипты прошлых прогонов.

Данные организованы как `tests/<база>/<версия>/<топология>/<тест>/<preset>/`.
Код и bootstrap-манифесты находятся только в `tools/`.

Из корня репозитория:

```sh
python3 pipelines/live/tools/live.py validate
python3 pipelines/live/tools/live.py progress
```

Обе команды работают с сохранёнными файлами, без обращения к кластеру и YC.
