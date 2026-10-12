# Занятие 4: HTTP API и REST

Простой сервер книг для примеров из презентации. Только стандартная библиотека
Go: `net/http`, `encoding/json`, `embed`. Нужен Go 1.22 или новее.

## Запуск

```sh
cd lecture4_http_api
go run .
```

- API: http://localhost:8080/books
- Swagger UI: http://localhost:8080/docs/
- OpenAPI: http://localhost:8080/openapi.json

Swagger UI загружает JS/CSS с CDN (нужен интернет в браузере).
В UI откройте метод → **Try it out** → заполните параметры → **Execute**.
Запрос пойдёт в этот же локальный сервер, а не во внешний Petstore.
Спецификация — описание API, она сама не запускает сервер.

## Маршруты

| Метод | Адрес | Результат |
| --- | --- | --- |
| GET | /books | 200, массив книг |
| GET | /books/{id} | 200, книга; 404, если её нет |
| POST | /books | 201, новая книга и заголовок Location |
| PUT | /books/{id} | 200, полная замена существующей книги |
| DELETE | /books/{id} | 204, без тела |

`HEAD` автоматически поддерживается маршрутами `GET` в ServeMux.
`PATCH`, `OPTIONS`, `CONNECT`, `TRACE` в этом примере не реализованы.
Неизвестный маршрут и неподдерживаемый метод обрабатывает сам ServeMux
(404/405, текстовое тело; для 405 также заголовок Allow).

## Попробовать через curl

```sh
curl -i http://localhost:8080/books

curl -i -X POST http://localhost:8080/books \
  -H 'Content-Type: application/json' \
  -d '{"title":"Learning Go","author":"Jon Bodner"}'

curl -i http://localhost:8080/books/2

curl -i -X PUT http://localhost:8080/books/2 \
  -H 'Content-Type: application/json' \
  -d '{"title":"Learning Go, second edition","author":"Jon Bodner"}'

curl -i -X DELETE http://localhost:8080/books/2

# Ошибка валидации: 422
curl -i -X POST http://localhost:8080/books \
  -H 'Content-Type: application/json' \
  -d '{"title":" ","author":"Jon Bodner"}'
```

## Что посмотреть в коде

- `main.go`: запуск HTTP-сервера и таймауты.
- `server.go`: маршруты, request/response, JSON, валидация, middleware.
- `docs.go`: раздача встроенных в бинарник OpenAPI и HTML.
- `openapi.json`: контракт API: параметры, тело, статусы и схемы.

Сначала декодируем JSON, затем проверяем смысл данных. Неверный JSON → 400,
пустые поля → 422, неверный Content-Type → 415. ID назначает сервер.
Тело ограничено 1 MiB; неизвестные поля и несколько JSON-объектов запрещены.
Ошибки обработчиков имеют общий формат:

```json
{"error":{"code":"book_not_found","message":"Book not found"}}
```

Повторный PUT с тем же телом оставит то же состояние (идемпотентность).
Повторный DELETE даст 404 вместо 204, но книга всё равно будет отсутствовать:
идемпотентность относится к эффекту, а не к одинаковым статусам.
Повторный POST создаст ещё одну книгу — он не идемпотентен.

## Проверка

```sh
go test -race ./...
go vet ./...
```

Это учебный пример, не production-сервис. Данные в памяти сбрасываются при
перезапуске; map защищён mutex, потому что запросы обрабатываются параллельно.
Нет БД, авторизации, пагинации, graceful shutdown и генерации спецификации:
при изменении обработчиков нужно вручную обновить `openapi.json`.
