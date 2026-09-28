# Нативный JSON-отчёт Stroppy

Контракт интеграции описан в [HANDOFF.md](HANDOFF.md#результаты).
Результаты доступны серверу через Graphene; прямое подключение сервера
к Docker, Kubernetes или хранилищам телеметрии не требуется.

## Локальная приёмка

Проверены штатное завершение и SIGINT настоящим бинарником Stroppy,
а также полная activity RunSegment с официальным контейнером 6.1.0:

```
ghcr.io/stroppy-io/stroppy:v6.1.0.63
ghcr.io/stroppy-io/stroppy@sha256:12e6064f6682024002f561414e3cebf28b5c47db9e0fab797c7e17ac110d41a7
```

Docker-проверка запускает execute_sql на noop: три итерации, метрики из
JSON, полный файл совпадает с inline-отчётом. UID/GID процесса совпадает с
агентом, поэтому файл 0600 доступен и агенту без root. Контейнеры теста
удаляются после проверки. Облачные ресурсы не создаются.

В модуле `pipelines`:

```sh
STROPPY_REPORT_TEST_BINARY=/path/to/stroppy go test ./internal/activities -run TestReportCLICompatibility -count=1 -v
STROPPY_REPORT_TEST_IMAGE=ghcr.io/stroppy-io/stroppy@sha256:12e6064f6682024002f561414e3cebf28b5c47db9e0fab797c7e17ac110d41a7 go test ./internal/activities -run TestReportDockerActivity -count=1 -v
```

Обычные тесты без внешнего бинарника проверяют native fixture, отсутствие и
повреждение отчёта, неподдерживаемую версию, failed/canceled, nonzero exit,
thresholds, точные uint64, неизвестные payloads и лимит inline.
Симуляция Graphene проверяет загрузку/отказ загрузки артефакта, отсутствие
несуществующих ссылок и TTL 30 дней независимо от удаления стенда.
HTTP-контракт сохраняет report/report_artifact/report_omitted и нормализует
только статус сегмента, не native report.status.

`stroppycfg/testdata/run-report.json` получен командой `stroppy run 'SELECT 1'
-d noop --executor shared-iterations --iterations 3 --report-file ...` из
локальной сборки ветки PR Stroppy #173 на базе 6.1.0; envelope schema 1.
Полные buckets и series сохранены, значения времени не являются нормативом
производительности. Совместимость с официальным релизом проверяет Docker-тест.

Реальный прогон через развёрнутый сервер и YC после обновления pipeline ещё
нужен для приёмки развёртывания; локальные проверки его не заменяют.

## Результат проверок реализации

- Сборка сервера и пяти пайплайнов; manifest run/suite.
- Линтер сервера и пайплайнов без замечаний.
- Полный набор пайплайнов без race: passed; `spec` — 114.877 s.
- Полный набор сервера с race: passed.
- HTTP-интеграция с PostgreSQL и подставным Graphene: passed, 534.344 s.
- Профильные race-тесты parser/activity/run/suite: passed.
- `make contract-check`: версия 2.1.0, 219 артефактов и браузерные JSON-тесты.

Дополнительный полный race-прогон пайплайнов не завершился: тест
`TestStructuredLiveInputsFitProductSchema` превысил общий timeout 10 минут
при повторной компиляции схем для live-фикстур. Race detector ошибок не
сообщил; этот прогон не считается passed. Штатный набор без race и отдельные
race-проверки изменённых компонентов завершились успешно.
