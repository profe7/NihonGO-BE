# NihonGO Backend

Go backend for learning Japanese through authenticated hiragana and katakana quizzes.

## Architecture

- `internal/kana` owns behavior shared by hiragana and katakana: cards, quiz and answer handlers, progress statistics, and repository algorithms.
- `internal/hiragana` and `internal/katakana` are thin feature entry points that select a fixed script configuration.
- `internal/study` contains content-independent study primitives such as option building, character-filter parsing, and accuracy calculation.
- `internal/auth` and `internal/user` own authentication, tokens, and user persistence.

Hiragana and katakana share a kana abstraction because their data and study flows are identical. Kanji and vocabulary should remain separate domains unless real implementation evidence shows a stable shared abstraction.

## Run locally

```bash
cp .env.example .env
docker compose up --build
```

The API listens on `http://localhost:8080` by default.

## Authentication

Public endpoints:

- `GET /health`
- `POST /auth/register`
- `POST /auth/login`
- `POST /auth/refresh`
- `POST /auth/logout`

Protected endpoints require an access token:

```http
Authorization: Bearer <access_token>
```

## Study endpoints

Hiragana:

- `GET /hiragana/quiz`
- `GET /hiragana/quiz?characters=あ,い,う`
- `POST /hiragana/quiz/answer`
- `GET /hiragana/stats`

Katakana:

- `GET /katakana/quiz`
- `GET /katakana/quiz?characters=ア,イ,ウ`
- `POST /katakana/quiz/answer`
- `GET /katakana/stats`

Answer request:

```json
{
  "id": 1,
  "answer": "a"
}
```

Statistics response:

```json
{
  "total_attempts": 3,
  "correct_attempts": 2,
  "accuracy_percent": 66.66666666666666
}
```

## Quality checks

Format, test, and vet all packages:

```bash
docker compose run --rm -T api go fmt ./...
docker compose run --rm -T api go test ./...
docker compose run --rm -T api go vet ./...
```

Integration tests use a separate PostgreSQL database. Create it once:

```bash
docker compose exec -T db sh -c 'createdb -U "$POSTGRES_USER" nihongo_test'
```

Then run the complete suite serially so packages sharing the test database cannot interfere with each other:

```bash
docker compose run --rm -T api sh -c 'TEST_DATABASE_URL="postgres://$DB_USER:$DB_PASSWORD@db:$DB_PORT/nihongo_test?sslmode=disable" go test ./... -p 1 -count=1'
```
