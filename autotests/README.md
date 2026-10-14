# Автотесты курса: ДЗ 1–10

Основная инструкция подключения — в [корневом README](../README.md).
Студент один раз добавляет `ci-example.yml` в `.github/workflows/course.yml`.
На каждом ДЗ создаёт ветку `1`–`10`; `task1`–`task10` тоже поддерживаются.
Результат находится в Checks его PR. Параметры CI менять не нужно.

## Запуск

```bash
cd ~/code/go-courses/autotests
python3 -m pip install -r requirements.txt

# Автоматически определить ДЗ и структуру:
python3 scripts/ci.py --repo ~/code/student-homework --branch 7

# Явный запуск локально (принимается цифра и taskN):
./scripts/check.sh 3 ~/code/student-homework --package cmd/server
./scripts/check.sh 7 ~/code/student-homework --report reports/task7.json

# Проверка проверяющего инструмента:
make check
make test
```

Зависимости: Go 1.26.4+, Python 3.9+, Git, PyYAML из `requirements.txt`.
Для ДЗ 5–10 обязательны работающие Docker и Docker Compose. Если Docker недоступен,
проверка красная, контейнерные проверки не пропускаются ради зелёного статуса.

## Наборы

| ДЗ | Реализация | Что делает |
| --- | --- | --- |
| 1 | `task1/twosum_test.go.txt` | Table cases и генерируемые входы с единственным решением; порядок индексов свободный |
| 2 | `task2/cli_test.go` | CLI и worker pool с управляемым HTTP-стендом |
| 3 | `task3/http_test.go` | 5 HTTP-тестов без привязки к внутренним пакетам |
| 4 | `scripts/advanced.py` | Слои, импорты domain/usecase, HTTP-регрессия, студенческие тесты |
| 5 | `scripts/advanced.py` | PostgreSQL, SQL-миграции, sync, повторный запуск и чтение без стенда |
| 6 | `scripts/advanced.py` | Поиск по полям, Redis TTL и cache hit без PostgreSQL |
| 7 | `scripts/advanced.py` | Kafka download tasks / downloaded, повторная доставка, два сервиса |
| 8 | `scripts/advanced.py` | Покрытие студенческими тестами >70% и HTML-поиск |
| 9 | `scripts/advanced.py` | Compose build/up/API/search и структурная проверка Kubernetes |
| 10 | `scripts/advanced.py` | Compose observability, метрики, JSON-логи, provisioning и 6 дашбордов |

ДЗ 5–8 используют временные контейнеры `postgres:16-alpine`, `redis:7-alpine`,
`docker.redpanda.com/redpandadata/redpanda:v26.2.4` (Kafka-совместимый брокер).
Каждое решение запускается с чистыми данными. Созданные проверкой контейнеры удаляются после запуска.
ДЗ 9–10 используют студенческий Compose, отдельное имя Compose project и свои fixtures.
Данные Compose удаляются при завершении проверки.

## Договор запуска

- `MOVIE_BASE_URL` — базовый URL фильм-стенда, путь запроса `/{id}/info.0.json`.
- `HTTP_ADDR` — адрес HTTP-сервера (ДЗ 3–8).
- `DATABASE_URL` — PostgreSQL DSN (ДЗ 5–8).
- `REDIS_ADDR` — `host:port`, без пароля (ДЗ 6–8).
- `KAFKA_BROKERS` — адреса брокеров через запятую (ДЗ 7–8).
- С ДЗ 7 точки входа располагаются в путях с `catalog`/`downloader` в названии.
  Поддерживаются как один общий Go-модуль, так и два отдельных.
- Запуск без обязательных CLI-аргументов; относительные конфиги читаются из каталога Go-модуля.
- С ДЗ 5 сервис должен применить SQL-миграции/подготовить схему при старте до синхронизации.
  SQL-миграции должны быть в репозитории. Таблица фильмов содержит `title`,
  актёры и жанры представлены отдельными таблицами или колонками с `actor`/`genre` в названии.
- Поиск: `GET /search?q=...` возвращает JSON-массив фильмов. Проверяются отдельные слова
  из title/description/director/actors/genres; AND/OR многословного поиска не навязывается.
- ДЗ 8: `GET /` возвращает HTML-форму с полем `q`; метод GET, action указывает на HTML-результаты.
- Compose в корне решения: `compose.yaml`, `compose.yml`, `docker-compose.yml` или `.yaml`.
  В именах двух сервисов есть `catalog`/`downloader`; они собираются через `build`.
  У catalog опубликован HTTP-порт. Адрес fixtures задаётся проверкой через `MOVIE_BASE_URL`.
- ДЗ 9: Deployment для каждого сервиса; имена содержат `catalog`/`downloader`, catalog replicas >=2.
- ДЗ 10: `/metrics` отдаёт Prometheus text format, семейства с именами, содержащими HTTP/gRPC,
  search/movie, redis, kafka, postgres/pg_/database. JSON-логи: `level`, `msg`, request/correlation id.

HTTP-контракт: `/load-movies?from=N&to=M` → JSON-массив; `/last-movie` → JSON-объект.
Поля: `id` (число), `title` (строка), `year` (число), `director` (строка).
Неверные параметры HTTP дают 400 с непустым описанием. Swagger доступен на `/swagger/index.html`.
ДЗ 2: `--from`, `--to`, `--workers` (10), `--timeout` (`5s`, Go duration).
Вывод успешного фильма: `id — title — year — director`, порядок строк свободный.

Эти переменные/пути — публичный договор автопроверки. Его нужно включить в условия для студентов,
чтобы CI не требовал ручных настроек на каждом задании.

## Что означает зелёный результат

Пройден перечисленный набор. Это не полный автоматический зачёт всех критериев:
не проверяются gRPC, все частичные ошибки, Kafka retry/DLQ и offset при ошибке БД,
реальный запуск Kubernetes и качество панелей Grafana. Smoke-проверка Swagger проверяет только страницу.
Направления импортов дают базовую проверку слоёв, а не доказательство чистой архитектуры.

Для ДЗ 8 преподавательские тесты не добавляются к студенческому профилю покрытия.
Покрытие агрегируется по числу Go statements во всех модулях решения, строго >70%.
Generated-код пока не исключается из общего покрытия.

Исходное решение копируется во временную папку. Это не sandbox: запускайте чужой код
в одноразовом окружении без секретов. Итоговую приёмку делайте на конкретном SHA студента и версии тестов.
JSON-отчёт сохраняет результаты и revisions. Незакоммиченные изменения в SHA не отражаются.
