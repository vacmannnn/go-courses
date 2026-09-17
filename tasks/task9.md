### Docker + Kubernetes

Нужно подготовить проект к запуску в контейнерах.

Docker:

- написать `Dockerfile` для `catalog-service`;
- написать `Dockerfile` для `downloader-service`;
- добавить `docker-compose.yml`, который поднимает оба сервиса и все зависимости: PostgreSQL, Redis, Kafka;
- сервис должен запускаться одной командой, например `docker compose up`.

Kubernetes:

- добавить манифесты для `catalog-service` и `downloader-service`;
- для каждого сервиса нужны `Deployment` и `Service`;
- конфигурацию вынести в `ConfigMap`, чувствительные значения — в `Secret`;
- добавить readiness/liveness probes для сервисов;
- указать базовые resource requests/limits;
- запустить `catalog-service` в нескольких репликах и проверить, что запросы продолжают работать.

PostgreSQL, Redis и Kafka можно поднять в Kubernetes простыми манифестами или использовать готовые образы без сложной production-настройки.

Helm, Ingress, autoscaling и production-grade настройка Kafka/PostgreSQL в этом ДЗ не требуются.