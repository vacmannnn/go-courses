# HTTP API + REST: материалы занятия

## Материалы

- [curl.sh](curl.sh) — команды по порядку лекции с ожидаемыми ответами.
- [README.md](README.md) — запуск и описание учебного API.

## Слайды и исходники

- [Презентация в Google Slides](https://docs.google.com/presentation/d/19TJWywCLAhV1o4nGXv7VYB9zV6_AtgzN5NDfLZ4LPvA/edit)
- [Примеры lecture4_http_api](https://github.com/vacmannnn/go-courses/tree/main/lecture4_http_api)
- [README проекта](https://github.com/vacmannnn/go-courses/blob/main/lecture4_http_api/README.md)
- [Запуск сервера: main.go](https://github.com/vacmannnn/go-courses/blob/main/lecture4_http_api/main.go)
- [Обработчики и middleware: server.go](https://github.com/vacmannnn/go-courses/blob/main/lecture4_http_api/server.go)
- [Контракт API: openapi.json](https://github.com/vacmannnn/go-courses/blob/main/lecture4_http_api/openapi.json)
- [Тесты: server_test.go](https://github.com/vacmannnn/go-courses/blob/main/lecture4_http_api/server_test.go)

Привязка к слайдам проверена 15 октября 2026 года.

## Подготовка

Нужен Go 1.22 или новее.

~~~sh
git clone https://github.com/vacmannnn/go-courses.git
cd go-courses/lecture4_http_api
go run .
~~~

Если репозиторий уже скачан, перейдите в его каталог lecture4_http_api и выполните go run .

Во втором терминале:

~~~sh
BASE=http://localhost:8080
curl -i "$BASE/books"
~~~

Локальные адреса после запуска:

- API: http://localhost:8080/books
- OpenAPI: http://localhost:8080/openapi.json
- Swagger UI: http://localhost:8080/docs/

Swagger UI загружает JS/CSS с CDN: нужен интернет в браузере.

## Как пользоваться командами

Откройте curl.sh как обычный текст. Копируйте блоки по порядку, включая BASE в начале сессии терминала. Файл рассчитан на последовательный показ, а не на запуск всех демонстраций одной командой.

Начните с чистого запуска сервера. Сначала существует книга 1, новая получит ID 2. Дополнительные POST через curl или Swagger UI сдвигают следующие ID — смотрите поле id и заголовок Location.

Перезапуск сбрасывает данные и ID. PATCH не реализован и вернёт 405 для /books/1. PUT обновляет только существующую книгу.

## Привязка к слайдам

Номера соответствуют версии из 48 слайдов; если порядок изменится, ориентируйтесь на заголовки.

| Слайды | Демонстрация |
| --- | --- |
| 9 — Тело сообщения | Запрос и ответ через curl -v |
| 15 — Статус-коды | Создание, чтение, замена и удаление |
| 18 — Идемпотентность | Повтор POST, PUT и DELETE |
| 23–26 — REST | Ресурс, представление и stateless |
| 31–34 — Проект на Go | main, маршруты, list, writeJSON, logging |
| 37–40 — JSON и ошибки | BookInput, readInput, ответы 400/415/422 |
| 41 — Успешное создание | create и корректный POST |
| 44–45 — OpenAPI / Swagger | Контракт и запросы через UI |

## Проверка примера

В каталоге lecture4_http_api:

~~~sh
go test -v -run TestBookLifecycle .
go test -race ./...
go vet ./...
~~~

Тесты httptest работают без отдельного запуска HTTP-сервера.
