#!/usr/bin/env bash
# HTTP API + REST. Копируйте блоки по порядку из этого файла.
# Сначала в другом терминале: cd lecture4_http_api && go run .
# Начните с чистого запуска сервера. Перезапуск сбрасывает данные и ID.
BASE=http://localhost:8080

# 1. После слайда 9: запрос и ответ. 200, начальная книга ID 1.
curl -v "$BASE/books"

# 2. После слайда 15: жизненный цикл книги.
# Создание: 201, Location: /books/2.
curl -i "$BASE/books" \
  -H 'Content-Type: application/json' \
  -d '{"title":"Learning Go","author":"Jon Bodner"}'

# Чтение: 200.
curl -i "$BASE/books/2"

# Полная замена: 200.
curl -i -X PUT "$BASE/books/2" \
  -H 'Content-Type: application/json' \
  -d '{"title":"Learning Go, second edition","author":"Jon Bodner"}'

# Удаление: 204 без тела. Затем чтение: 404.
curl -i -X DELETE "$BASE/books/2"
curl -i "$BASE/books/2"

# 3. После слайда 18: идемпотентность.
# Одинаковые POST создают разные книги: ID 3 и 4.
curl -i "$BASE/books" \
  -H 'Content-Type: application/json' \
  -d '{"title":"Go","author":"A"}'

curl -i "$BASE/books" \
  -H 'Content-Type: application/json' \
  -d '{"title":"Go","author":"A"}'

# В списке книги 1, 3 и 4.
curl -s "$BASE/books"

# Одинаковые PUT: 200, одинаковое итоговое состояние.
curl -i -X PUT "$BASE/books/3" \
  -H 'Content-Type: application/json' \
  -d '{"title":"Updated","author":"A"}'

curl -i -X PUT "$BASE/books/3" \
  -H 'Content-Type: application/json' \
  -d '{"title":"Updated","author":"A"}'

# DELETE: 204, затем 404. Эффект одинаковый: книги нет.
curl -i -X DELETE "$BASE/books/3"
curl -i -X DELETE "$BASE/books/3"

# 4. Слайды 23-26: ресурс и представление.
curl -i "$BASE/books/4"

# 5. Слайды 31-34: middleware.
# После запроса покажите GET /books с длительностью в логе сервера.
curl -i "$BASE/books"

# 6. Слайды 37-40: JSON и ошибки.
# Корректный JSON, пустое название: 422, validation_error.
curl -i "$BASE/books" \
  -H 'Content-Type: application/json' \
  -d '{"title":"   ","author":"A"}'

# Сломанный JSON: 400, invalid_json.
curl -i "$BASE/books" \
  -H 'Content-Type: application/json' \
  -d '{'

# Неподходящий Content-Type: 415, unsupported_media_type.
curl -i "$BASE/books" \
  -H 'Content-Type: text/plain' \
  -d '{"title":"Go","author":"A"}'

# 7. После слайда 41: успешное создание, 201, ID 5.
curl -i "$BASE/books" \
  -H 'Content-Type: application/json' \
  -d '{"title":"HTTP in Go","author":"Demo"}'

# 8. Слайды 44-45: описание API.
curl -i "$BASE/openapi.json"
# Браузер: http://localhost:8080/docs/
# GET /books -> Try it out -> Execute.
# Затем POST /books -> создайте книгу -> снова GET /books.
