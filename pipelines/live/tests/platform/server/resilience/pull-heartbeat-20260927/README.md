# Heartbeat при молчащем Docker pull

MySQL run `22a00ddf-a390-4e89-b04a-a2eac80894bc` на proxy-1 повторял загрузку
ProxySQL из-за недоступности Docker Hub. В одной попытке сообщение стало
`decode docker progress: invalid activityID or activity already timed out`.

В `pipelines/internal/activities/docker.go` heartbeat зависел от поступления
JSON progress. Ожидание HTTP-ответа, скачивание или распаковка без progress могли
превысить минутный heartbeat timeout. Теперь heartbeat отправляется сразу и
каждые 15 секунд отдельной goroutine на всё время PullImage. Она останавливается
при завершении/отмене; общий deadline activity остаётся прежним.

Проверки: настоящий PullImage против HTTP-имитации Docker, которая отвечает
только после повторных heartbeat до ответа и в середине потока; разбор Docker
errors; workflow simulations; race detector; lint (0 issues). TestActivityEnvironment
пакетирует heartbeat раз в 30 секунд, поэтому регрессия занимает около минуты.

Собран и запущен локальный Compose
`0.0.0-dev-f658fd8b-pull-heartbeat-20260927`; [resync](resync.json) и
[synced status](status.json) получены через API сервера. Graphene не изменялся.
Образы реальных старых run не переписывались. Generated TPC-DS запускается
после публикации исправления; зеркала решают отдельную проблему доступа к registry.
