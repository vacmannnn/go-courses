# Занятие 7. SQL, миграции и PostgreSQL из Go

Небольшой каталог фильмов, пользователей и оценок из лекции. CLI создаёт два
фильма, читает фильм вручную через `Scan` и через sqlc, меняет оценку и выводит
средние оценки через `LEFT JOIN` и `GROUP BY`.

Нужны Go 1.23+, Docker с Compose и goose. Сгенерированный код sqlc включён
в репозиторий. Версии Go-зависимостей зафиксированы в `go.mod` и `go.sum`.

## Быстрый запуск

```sh
cd ~/code/go-courses/lecture7_sql
go mod download
go install github.com/pressly/goose/v3/cmd/goose@v3.26.0
export PATH="$(go env GOPATH)/bin:$PATH"

make up
make migrate
make run
```

Для новой БД начало вывода выглядит так

```text
created movie=1 and unrated movie=2
manual movie=1 title="Начало" year=2010 description=NULL
sqlc   movie=1 title="Начало" year=2010 description_valid=false
score=11 rejected by CHECK, SQLSTATE=23514
movie=0 not found, distinct from a NULL description
movie=1 title="Начало" ratings=2 average=8.5
movie=2 title="Интерстеллар" ratings=0 average=NULL
```

Каждый демонстрационный запуск добавляет два фильма. Названия фильмов могут
повторяться. Используем ID из вывода `RETURNING`, поэтому повторный запуск
работает с новыми строками. Пользователей находим по уникальному email.

По умолчанию `DATABASE_URL` равен
`postgres://lecture:lecture@localhost:5437/movies?sslmode=disable`.
Для другой БД передайте переменную окружения или значение в Make

```sh
DATABASE_URL='postgres://user:password@localhost:5432/movies?sslmode=disable' go run ./cmd -list
make migrate DATABASE_URL='postgres://user:password@localhost:5432/movies?sslmode=disable'
```

## Что открывать во время лекции

| Блок | Файл | Что показываем |
| --- | --- | --- |
| Таблицы и ограничения | `migrations/001_create_catalog.sql` | PK, FK, NOT NULL, UNIQUE, CHECK и связи |
| Изменение схемы | `migrations/002_add_description.sql` | Up, Down, новый столбец и NULL у старых строк |
| Пул и context | `cmd/main.go`, функция `run` | Один `pgxpool.Pool`, проверка через Ping, ограничение ожидания |
| Параметры и Scan | `internal/catalog/repository.go` | INSERT RETURNING, `$1`, соответствие столбцов и полей |
| Повторная оценка | `RateMovie` | ON CONFLICT обновляет ту же пару пользователя и фильма |
| JOIN и средний балл | `ListRatings` | Фильмы без оценок, COUNT, AVG, чтение строк и rows.Err |
| Генерация sqlc | `queries/movies.sql`, `internal/db/` | SQL превращается в методы и типы Go |

## Данные переживают перезапуск

Подставьте ID фильма из первого запуска

```sh
go run ./cmd -id 1
go run ./cmd -list
```

Обе команды запускают новый процесс. Фильм читается из PostgreSQL.
В `compose.yml` данные БД сохраняются в именованном томе. Проверить это можно
после остановки контейнера

```sh
make down
make up
go run ./cmd -id 1
```

## Миграции по шагам

```sh
make status
make rollback
make status
```

После отката `002` столбец `description` удалён. Приложение читает этот столбец,
поэтому запрос завершится ошибкой отсутствующего столбца. Восстановите схему

```sh
make migrate
go run ./cmd -id 1
```

Down удаляет и содержимое столбца. Повторный Up создаёт столбец со значением NULL
у существующих фильмов. Миграции запускаем перед использованием новой схемы.

## NULL, отсутствие строки и ограничения

У существующего фильма описание может быть NULL. Ручной код читает его
в `*string`, sqlc использует `pgtype.Text` с признаком `Valid`.
Запрос несуществующего фильма возвращает `pgx.ErrNoRows`, который хранилище
преобразует в `catalog.ErrMovieNotFound`

```sh
go run ./cmd -id 999999
```

Обычная демонстрация сначала записывает Анне оценку 7, затем меняет её на 9
через ON CONFLICT. Оценка Александра равна 8. В результате у фильма две оценки
со средним 8.5. Попытка записать 11 отклоняется CHECK, прежняя оценка сохраняется.
Фильм без оценок попадает в результат LEFT JOIN с количеством 0 и средним NULL.

## sqlc

```sh
go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0
make generate
make check
```

`sqlc.yaml` связывает схему из миграций, запросы из `queries/` и пакет `internal/db`.
Сравните `catalog.Repository.GetMovie` и сгенерированный `db.Queries.GetMovie`.
В `showMovie` оба метода читают один ID через тот же пул.

## Проверка

```sh
make check
make integration
```

`make check` проверяет сборку, гонки в тестах и `go vet`. Интеграционный тест
запускается при заданном `TEST_DATABASE_URL`, который устанавливает цель
`make integration`. Перед ним нужны работающая БД и применённые миграции.

Проверяются ручное и сгенерированное чтение, NULL, отсутствие строки, LEFT JOIN,
обновление оценки и отказ CHECK. Тест удаляет созданные им фильмы, пользователей
и оценки. Завершив показ, остановите контейнер командой `make down`.
