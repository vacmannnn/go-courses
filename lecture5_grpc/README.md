# Занятие 5: gRPC и Protobuf

Один локальный сервис фильмов и Go-клиент для демонстраций из презентации.
Фильмы хранятся в памяти: IDs `1–5` и `42`. Внешний стенд и база данных не нужны.
`GetMovie(42)` возвращает `Inception`, 2010. Сервер ничего не изменяет в каталоге.

Нужен Go **1.25+**. Версии зависимостей зафиксированы в `go.mod` и `go.sum`.
Сгенерированные файлы включены в репозиторий: **для запуска protoc не нужен**.
Первый запуск скачивает Go-зависимости; перед занятием выполните `go mod download`.

## Быстрый запуск: два терминала

В первом терминале:

```sh
cd ~/code/go-courses/lecture5_grpc
go run ./cmd/server
```

Во втором:

```sh
cd ~/code/go-courses/lecture5_grpc
go run ./cmd/client
```

Ответ:

```text
movie=42 title="Inception" year=2010
```

Сервер слушает `127.0.0.1:50051`. Для локального примера TLS отключён.
`Ctrl+C` останавливает сервер; активным вызовам даётся до двух секунд на завершение.

## 1. Контракт и генерация

Откройте `proto/movie.proto`: сообщения `GetMovieRequest`, `Movie`,
`LoadMoviesRequest`, `LoadProgress` и сервис `MovieService`.

Для повторной генерации установите `protoc` (пример для macOS: `brew install protobuf`)
и Go-плагины:

```sh
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2
export PATH="$(go env GOPATH)/bin:$PATH"
make generate
```

В `gen/moviesv1/` появятся:

- `movie.pb.go`: типы сообщений, getters и поддержка сериализации;
- `movie_grpc.pb.go`: клиент, интерфейс сервера и регистрация обработчиков.

`package movies.v1` задаёт пространство имён Protobuf; `go_package` — путь Go-пакета.
Бизнес-логику генератор не пишет. Сгенерированные файлы вручную не редактируем.

В слайдах показаны короткие фрагменты; здесь они собраны в запускаемый проект.
Вместо заглушки из первого примера обработчик ищет фильм в небольшом каталоге.

## 2. Unary и коды ошибок

Сервер из быстрого запуска продолжает работать.

```sh
# OK
go run ./cmd/client -id 42

# InvalidArgument: id должен быть положительным
go run ./cmd/client -id 0

# NotFound: id корректен, фильма нет
go run ./cmd/client -id 999
```

При ошибке клиент выводит `code=... message=...` и завершается с кодом 1.
Код выбирает обработчик через `status.Error`; клиент читает его через `status.Code`.

## 3. Deadline и явная отмена

Остановите сервер и запустите с задержкой:

```sh
go run ./cmd/server -unary-delay 2s
```

Во втором терминале:

```sh
# Ждём только секунду: DeadlineExceeded
go run ./cmd/client -id 42 -timeout 1s

# Даём пять секунд: успешный ответ примерно через две секунды
go run ./cmd/client -id 42 -timeout 5s

# Отменяем сами раньше deadline: Canceled
go run ./cmd/client -id 42 -timeout 5s -cancel-after 300ms
```

В серверных логах видно, что работа прекращается до окончания двухсекундной задержки.
Код на клиенте и сервере при истечении deadline может различаться:
клиент видит `DeadlineExceeded`, сервер может наблюдать `Canceled`.
Таймаут не доказывает, что операция не выполнилась; отмена не откатывает изменения.
Здесь операция только читает данные.

## 4. Metadata и interceptors

```sh
go run ./cmd/client -id 42 -timeout 5s -request-id lecture-42
go run ./cmd/client -id 0 -request-id bad-input
```

В логах сервера появятся имя RPC, `request_id`, код результата и длительность.
`x-request-id` передаётся через metadata, не через поле `GetMovieRequest`.
Сервер читает metadata из контекста. В `internal/service/interceptors.go` есть
два отдельных interceptor: unary и streaming. Один unary-interceptor поток не оборачивает.

## 5. Server streaming

Остановите сервер с задержкой unary и вернитесь к обычному запуску:

```sh
go run ./cmd/server -stream-delay 300ms
```

Во втором терминале:

```sh
go run ./cmd/client -mode stream -from 1 -to 5 -timeout 5s
```

Сообщения появляются по одному:

```text
movie=1 loaded=1
movie=2 loaded=2
movie=3 loaded=3
movie=4 loaded=4
movie=5 loaded=5
stream completed: OK
```

Теперь два сценария остановки:

```sh
# Несколько сообщений, затем явная отмена
go run ./cmd/client -mode stream -timeout 5s -cancel-after 750ms

# Несколько сообщений, затем истечение deadline
go run ./cmd/client -mode stream -timeout 750ms
```

Число успевших сообщений зависит от времени запуска и планирования.
Сервер реагирует на `stream.Context()`. Успешное завершение клиент читает как `io.EOF`.

Ошибка после части прогресса:

```sh
# Есть фильмы 4 и 5; фильма 6 нет
go run ./cmd/client -mode stream -from 4 -to 6 -timeout 5s
```

Клиент получает два сообщения, затем `NotFound`. Это не `io.EOF`.
Полученные сообщения не исчезают и не доказывают успешность всего RPC.
Счётчик означает обработку фильма в учебном примере; постоянного сохранения в БД нет.

Диапазон должен быть положительным, упорядоченным и содержать не более 100 IDs.
Обычный `LoadMovies` из ДЗ остаётся unary: `LoadMoviesStream` — дополнительная демонстрация.
Client streaming и bidirectional streaming разобраны на слайдах; в этом проекте
подробно реализован server streaming.

## 6. Старый и новый контракт: почему year = 0

```sh
go run ./cmd/evolution
```

Пример использует реальные байты Protobuf и старую схему `Movie` без `year`:

- старый сервер → новый клиент: ID и название читаются, `year` равен `0` по умолчанию;
- новый сервер → старый клиент: ID и название читаются, байты неизвестного поля сохраняются.

Это сериализация в одном процессе, отдельный сервер для неё не нужен.
Для обычного `int32 year` отсутствие значения и переданный ноль не различаются.
Если различие нужно, используйте `optional int32 year = 3` и проверяйте наличие.

После удаления `year` добавьте `reserved 3;` и `reserved "year";` в сообщение.
`protoc` запретит повторно использовать номер или имя. Резервирование не исправляет
бизнес-логику старого клиента, который рассчитывал получать год.

## Вызовы через grpcurl

Необязательный инструмент; сервер включает reflection, поэтому `.proto` передавать не нужно.

```sh
grpcurl -plaintext localhost:50051 list
grpcurl -plaintext localhost:50051 describe movies.v1.MovieService
grpcurl -plaintext -d '{"id":42}' localhost:50051 movies.v1.MovieService/GetMovie
grpcurl -plaintext -H 'x-request-id: grpcurl-demo' \
  -d '{"from":1,"to":5}' localhost:50051 movies.v1.MovieService/LoadMoviesStream
```

JSON здесь — формат ввода и вывода инструмента, сетевые сообщения gRPC остаются Protobuf.

## Что открыть во время занятия

| Файл | Что показываем |
| --- | --- |
| `proto/movie.proto` | Контракт, номера полей, `stream` |
| `gen/moviesv1/movie.pb.go` | Типы сообщений |
| `gen/moviesv1/movie_grpc.pb.go` | Клиент, серверный интерфейс, регистрация |
| `internal/service/movies.go` | Unary, статусы, context, Send |
| `internal/service/interceptors.go` | Metadata и логирование |
| `cmd/server/main.go` | Запуск, регистрация, reflection |
| `cmd/client/main.go` | NewClient, deadline, cancel, metadata, Recv |
| `cmd/evolution/main.go` | Старый и новый контракт |

## Проверка

```sh
make check
```

Тесты используют настоящий gRPC через in-memory соединение: проверяют сериализацию,
статусы, прекращение серверной работы, порядок потока, EOF, частичную ошибку и metadata.
