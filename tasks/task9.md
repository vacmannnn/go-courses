### Docker + Kubernetes

Нужно подготовить проект к запуску в контейнерах.

Docker:

- написать `Dockerfile` для `catalog-service`;
- написать `Dockerfile` для `downloader-service`;
- добавить `docker-compose.yml`, который поднимает оба сервиса и все зависимости: PostgreSQL, Redis, Kafka;
- сервис должен запускаться одной командой, например `docker compose up`.

`downloader-service` должен читать `MOVIE_BASE_URL` из окружения. Docker Compose должен позволять переопределить эту переменную для `downloader-service`; базовый URL фильм-стенда нельзя жёстко зашивать в код или Docker-образ. HTTP-порт `catalog-service` должен быть опубликован через Compose.

Kubernetes:

- добавить манифесты для `catalog-service` и `downloader-service`;
- для каждого сервиса нужны `Deployment` и `Service`;
- конфигурацию вынести в `ConfigMap`, чувствительные значения — в `Secret`;
- добавить readiness/liveness probes для сервисов;
- указать базовые resource requests/limits;
- запустить `catalog-service` в нескольких репликах и проверить, что запросы продолжают работать.

PostgreSQL, Redis и Kafka можно поднять в Kubernetes простыми манифестами или использовать готовые образы без сложной production-настройки.

Helm, Ingress, autoscaling и production-grade настройка Kafka/PostgreSQL в этом ДЗ не требуются.

### Критерии оценивания

6 баллов — Добавлены рабочие Dockerfile для обоих сервисов. Docker Compose одной командой поднимает catalog-service, downloader-service, PostgreSQL, Redis и Kafka; HTTP-порт опубликован, MOVIE_BASE_URL для downloader-service можно переопределить через окружение. В Kubernetes для обоих сервисов есть Deployment и Service, используются ConfigMap и Secret, настроены readiness/liveness probes и resource requests/limits. Проверена работа API при нескольких репликах catalog-service. В README описаны запуск через Compose и Kubernetes и изменение числа реплик.

5 баллов — Запуск через Compose и Kubernetes работает, все обязательные манифесты и настройки добавлены, несколько реплик обслуживают запросы. Есть небольшой недочёт в документации, например, не приведена команда изменения числа реплик.

4 балла — Оба сервиса и зависимости запускаются через Compose, базовое развёртывание в Kubernetes работает, но отсутствуют probes, resource requests/limits или проверка нескольких реплик. Либо конфигурация не полностью вынесена в ConfigMap/Secret или нельзя переопределить MOVIE_BASE_URL.

3 балла — Работает только один способ запуска: Compose или Kubernetes. Либо Docker-образы собираются, но для запуска всей системы нужны неописанные ручные шаги, часть зависимостей отсутствует или API недоступен снаружи Compose.

0–2 балла — Работа не сдана, Docker-образы не собираются или приложение нельзя запустить ни через Compose, ни через Kubernetes. Либо конфигурационные файлы добавлены формально и не позволяют запустить сервисы.

Отметка за задание снижается при более поздней сдаче задания (1 балл за каждые 2 недели опоздания).

Дедлайн: 1 неделя
