# mesa-ads — мини-движок видеорекламы

`mesa-ads` — это небольшой HTTP-сервис на Go, который подбирает видеорекламу под контекст запроса, учитывает бюджет кампаний и собирает базовую статистику по показам и кликам.  
Проект реализован как тестовое задание и построен по принципам hexagonal (ports & adapters) архитектуры.

---

## Возможности

- Подбор релевантного ролика по:
  - языку (`language`),
  - гео (`geo`),
  - категории контента (`category`),
  - интересам (`interests`),
  - плейсменту (`placement`).
- Поддержка кампаний с моделью:
  - **CPM** (списание при показе),
  - **CPC** (списание при клике).
- Учёт **дневного** и **общего** бюджета кампании.
- Фиксация событий:
  - **Impression** (показ),
  - **Click** (клик).
- Идемпотентная обработка кликов по токену (повторный клик не списывает бюджет повторно).
- Эндпойнт для агрегации статистики по периодам.

---

## Структура проекта

Основная логика спрятана в `internal/` и разделена на домен, порты и адаптеры.

```text
mesa-ads/
├── cmd/
│   └── main.go               # Точка входа приложения
├── internal/
│   ├── core/
│   │   ├── domain/           # Доменные сущности (Campaign, Creative, Impression, Click, Targeting, UserContext)
│   │   └── port/             # Порты (интерфейсы AdUseCase, AdRepository, DTO для статистики)
│   ├── adapter/
│   │   ├── http/             # HTTP-адаптер (обработчики /ad/request, /ad/click, /stats/overview)
│   │   ├── postgres/         # Postgres-репозиторий, реализация AdRepository
│   │   └── usecase/          # Реализация бизнес-логики (AdUseCase)
│   ├── config/               # Конфиг агрегатор и структуры для env-переменных
│   │   └── configs/          # Части конфига: HTTP, Logger, PostgreSQL
│   └── db/                   # Подключение к Postgres, миграции, сидирование
├── migrations/               # SQL-миграции (встроены через embed)
├── Dockerfile                # Образ приложения
├── docker-compose.yml        # Локальный стенд: Postgres + сервис
├── Makefile                  # Сборка, тесты, линтер, генерация моков
├── go.mod / go.sum           # Модуль и зависимости
├── .golangci.yml             # Конфиг линтера
└── .mockery.yml              # Конфиг генерации моков
```

---

## Архитектура

Проект следует hexagonal / ports & adapters подходу:

* **Domain (`internal/core/domain`)**
  Чистые структуры без зависимостей от инфраструктуры:

  * `Campaign` — кампания (бюджеты, ставки, даты, статус),
  * `Creative` — конкретный видеоролик (video URL, landing URL, duration),
  * `Targeting` — настройки таргета (языки, гео, категории, интересы, плейсменты),
  * `Impression`, `Click` — события,
  * `UserContext` — контекст входящего запроса.

* **Ports (`internal/core/port`)**

  * `AdUseCase` — интерфейс бизнес-операций:

    * `RequestAd` — подбор рекламы и запись показа,
    * `RegisterClick` — регистрация клика,
    * `GetStats` — агрегированная статистика.
  * `AdRepository` — интерфейс доступа к хранилищу:

    * выбор кандидатов (`GetEligibleCreatives`),
    * атомарная запись показов/кликов с изменением бюджета,
    * выбор статистики.
  * DTO `StatsReq`, `StatsResp`, `CreativeCandidate`.

* **Use case слой (`internal/adapter/usecase`)**
  Реализация `AdUseCase`:

  * строит список кандидатов по таргету и бюджету,
  * считает **eCPM** для ранжирования:

    * CPM: `eCPM = bid_cpm`,
    * CPC: `eCPM = bid_cpc × CTR_estimate × 1000` (CTR по умолчанию 1 %),
  * создаёт `Impression` и списывает CPM-бюджет,
  * регистрирует `Click` и списывает CPC-бюджет,
  * обеспечивает идемпотентность кликов через токены.

* **Inbound адаптер (HTTP, `internal/adapter/http`)**

  * роутинг на `chi`:

    * `POST /api/v1/ad/request` — запрос показа,
    * `GET /api/v1/ad/click/{token}` — редирект + запись клика,
    * `GET /api/v1/stats/overview` — статистика.
  * конвертация HTTP запросов/ответов в доменные модели и обратно,
  * логирование ошибок.

* **Outbound адаптер (Postgres, `internal/adapter/postgres`)**

  * хранение кампаний, креативов, таргета, показов и кликов,
  * `GetEligibleCreatives` фильтрует по таргету и остаткам бюджета,
  * `CreateImpressionAndDeductBudget` и `CreateClickAndDeductBudget` выполняют:

    * транзакцию,
    * проверку дневного и общего бюджета,
    * обновление `remaining_*_budget`,
    * вставку события.

* **Инфраструктура**

  * `internal/config` — загрузка конфигурации из env через `caarlos0/env`,
  * `internal/db` — построение пула `pgxpool`, прогон миграций через `golang-migrate`, сидирование тестовыми данными,
  * `migrations` — SQL-схема (таблицы `campaigns`, `creatives`, `targetings`, `impressions`, `clicks`).

---

## Переменные окружения

Конфиг собирается из нескольких секций:

### Общие

| Переменная | Тип    | Значение по умолчанию | Описание                                   |
|------------|--------|-----------------------|--------------------------------------------|
| `ENV`      | string | `prod`                | Название окружения (prod/dev/local и т.п.) |

### Логгер (`LOG_` префикс → `configs.Logger`)

| Переменная   | Тип    | По умолчанию | Описание                                        |
|--------------|--------|--------------|-------------------------------------------------|
| `LOG_LEVEL`  | string | `info`       | Уровень логов: `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | string | `text`       | Формат логов: `text` или `json`                 |

### HTTP-сервер (`HTTP_` префикс → `configs.HTTP`)

| Переменная  | Тип    | По умолчанию | Описание              |
|-------------|--------|--------------|-----------------------|
| `HTTP_PORT` | uint16 | `8080`       | TCP-порт HTTP-сервера |

В docker-compose значение порта пробрасывается наружу как `8080:8080`.

### PostgreSQL (`PSQL_` префикс → `configs.Postgres`)

| Переменная            | Тип  | По умолчанию                                                           | Описание                                            |
|-----------------------|------|------------------------------------------------------------------------|-----------------------------------------------------|
| `PSQL_ADDR`           | URL  | `postgres://postgres:password@localhost:5432/postgres?sslmode=disable` | Строка подключения к Postgres                       |
| `PSQL_RUN_MIGRATIONS` | bool | `false`                                                                | Запуск миграций при старте (`true`/`false`)         |
| `PSQL_RUN_SEED`       | bool | `false`                                                                | Заполнение демо-данными при старте (`true`/`false`) |

[Пример `.env` файла](docs/.env)

---

## Быстрый старт

### Вариант 1: Docker Compose

Требуется Docker и docker-compose.

```bash
# из корня репозитория
docker-compose up --build
```

Будет поднято:

* `postgres` на `localhost:5435`,
* `mesa-ads` на `localhost:8080`.

Приложение при старте:

* подключится к Postgres по `PSQL_ADDR`,
* при `PSQL_RUN_MIGRATIONS=true` применит миграции,
* при `PSQL_RUN_SEED=true` добавит демо-кампании и события.

### Вариант 2: Локальный запуск без Docker

1. Подними Postgres локально и создай БД (по умолчанию — `postgres`).

2. Установи переменные окружения, например:

   ```bash
   export PSQL_ADDR="postgres://postgres:password@localhost:5432/postgres?sslmode=disable"
   export PSQL_RUN_MIGRATIONS=true
   export PSQL_RUN_SEED=true
   ```

3. Запуск через Go:

   ```bash
   go run ./cmd
   ```

   или через `make`:

   ```bash
   make build
   ./mesa-ads
   ```

---

## HTTP API

### 1. Подбор рекламы — `POST api/v1/ad/request`

**Запрос**

```json
{
  "userID": "123",
  "language": "ru",
  "geo": "Russia",
  "category": "music",
  "interests": [
    "gaming"
  ],
  "placement": "pre-roll"
}
```

Поля:

* `user_id` — идентификатор зрителя (используется для связи событий и frequency-capping, если включён),
* `language`, `geo`, `category`, `placement` — контекст,
* `interests` — список интересов пользователя.

**Ответы**

* `200 OK` — найден креатив:

  ```json
  {
    "CreativeID": 2,
    "Duration": 42,
    "VideoURL": "https://example.com/video/2.mp4",
    "ClickURL": "api/v1/ad/click/fbee64a8-adab-447d-a3ea-79add27f8a86"
  }
  ```

  `click_url` — относительный URL, по которому нужно отправлять пользователя при клике.

* `204 No Content` — подходящего объявления нет (таргет / бюджет не позволяют отдать ролик).

* `400 Bad Request` — некорректный JSON.

* `500 Internal Server Error` — внутренняя ошибка.

### 2. Клик по объявлению — `GET api/v1/ad/click/{token}`

* `token` — токен, выданный в `click_url` из `/ad/request`.

Поведение:

* При валидном токене:

  * записывается событие `Click`,
  * для CPC-кампании списывается бюджет,
  * выполняется `302 Found` редирект на `landing_url` креатива.
* При неизвестном токене:

  * `404 Not Found`.
* При внутренних ошибках:

  * ошибка логируется,
  * наружу отдаётся `404` (чтобы не раскрывать детали).

Идемпотентность:

* Повторный запрос с тем же `token` **не создаёт второй клик** и **не списывает бюджет второй раз**.

### 3. Статистика — `GET api/v1/stats/overview`

Параметры query-строки:

* `from` (опционально) — начало периода, RFC3339 (`2025-01-01T00:00:00Z`),
* `to` (опционально) — конец периода, RFC3339,
* `campaign_id` (опционально) — фильтр по кампании (int).

Если `from`/`to` не заданы — берётся последний 1 день (`now() - 24h .. now()`).

**Ответ**

```json
{
  "impressions": 1234,
  "clicks": 56,
  "cost": 78900
}
```

Где:

* `impressions` — количество показов,
* `clicks` — количество кликов,
* `cost` — суммарный расход в минимальных денежных единицах (например, копейки/центы).

CTR можно посчитать на клиенте как `clicks / impressions`.

---

## Модель данных и бюджеты

* `campaigns`

  * `daily_budget`, `total_budget` — лимиты,
  * `remaining_daily_budget`, `remaining_total_budget` — остатки,
  * `cpm_bid`, `cpc_bid` — ставки для CPM / CPC,
  * `status` — состояние кампании.

* `creatives`

  * принадлежат кампании (`campaign_id`),
  * содержат `video_url`, `landing_url`, `duration`, `language`, `category`, `placement`.

* `targetings`

  * JSON-поля с массивами языков, гео, категорий, интересов и плейсментов.

* `impressions`, `clicks`

  * события с полями `token`, `creative_id`, `campaign_id`, `user_id`, `cost`, `created_at`.

Списание бюджета:

* CPM:

  * при показе — списывается `cpm_bid / 1000 * cost_factor`,
  * в коде используется целочисленное представление (например, 1.00 = 100).
* CPC:

  * при клике — списывается `cpc_bid`,
  * повторные клики по тому же токену не списывают бюджет повторно.

---

## Разработка

Репозиторий содержит `Makefile` с часто используемыми командами:

* `make generate` — запускает `go generate` для генерации моков.
* `make lint` — проверяет код линтером `golangci-lint`.
* `make fix` — пытается автоматически исправить найденные проблемы.
* `make test` — прогоняет unit‑тесты.
* `make build` — собирает бинарник `mesa-ads` из каталога `cmd`.
* `make clean` — удаляет артефакты сборки и файлы покрытия.
* `make install` — устанавливает инструменты для разработчика (`golangci-lint`, `mockery`).


---

## Import Postman Collection
Импортировать запросы, с помощью которых тестировалось приложение можно [тут](docs/mesa-ads.json)
