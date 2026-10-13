# Architecture

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/architecture/architecture-dark.png">
  <img src="docs/architecture/architecture-light.png" alt="GeoProphet 的資料匯入、API 與地圖架構">
</picture>

[開啟互動圖](docs/architecture/README.md) · [資料匯入與告警](docs/DATA_IMPORT.md) · [API](docs/API.md)

<details>
<summary>資料表關係</summary>

下圖只列部分外鍵。完整的資料表定義在 [migrations](migrations/)。

```mermaid
erDiagram
    import_runs o|..o{ transactions : import_run_id
    import_runs o|..o{ rentals : import_run_id
    import_runs o|..o{ presales : import_run_id
    import_runs ||--o{ quality_alert_deliveries : run_id
    transit_stations ||--o{ transit_exits : station_id
    transit_stations ||--o{ transit_station_lines : station_id
    transit_lines ||--o{ transit_station_lines : line_id
    transit_stations ||--o{ transaction_station_distances : station_id
    import_runs {
        bigint id PK
        text source
        text dataset
        text season
        text status
    }
    transactions {
        text id PK
        bigint import_run_id FK
        geography location
        text geocode_precision
    }
    rentals {
        text id PK
        bigint import_run_id FK
    }
    presales {
        text id PK
        bigint import_run_id FK
        text project_key
    }
    quality_alert_deliveries {
        bigint run_id PK,FK
        text code PK
        integer attempts
        timestamptz next_attempt_at
        timestamptz sent_at
    }
```

交易的 `import_run_id` 可以是 NULL。車站距離表雖然有 `transaction_id`，但沒有對交易表設定外鍵，因此圖中不畫這條線。

</details>

<details>
<summary>H3 查詢流程</summary>

```mermaid
sequenceDiagram
    participant Web as React / Vite
    participant Gin as Gin handler
    participant UC as Use case
    participant Repo as Postgres repository
    participant DB as PostgreSQL / PostGIS
    Web->>Gin: GET /api/v1/aggregations/h3 with filters
    Gin->>Gin: Validate query and map filters
    Gin->>UC: Query with standard context.Context
    UC->>Repo: Stream matching coordinates and prices
    Repo->>DB: Spatial and attribute filtering
    DB-->>Repo: Matching rows
    Repo-->>UC: Stream rows
    UC->>UC: Aggregate H3 cells in Go
    UC-->>Gin: Aggregated result
    Gin-->>Web: JSON response
    Web->>Web: Render Deck.gl layer
```

這是使用資料庫時的 H3 查詢：Go 計算蜂巢統計，行情與區域比較則主要由 SQL 計算。本機 mock 不會走這條路徑。

</details>

## 目前沒做的事

資料庫只保存處理後的資料，原始下載檔沒有保存，所以還不能從原始檔重播匯入。`import_runs` 記錄批次結果，不追蹤每個欄位的來源與變更。

## 看圖與數據時要注意

- 圖中是程式之間的關係，不是正式部署配置。本機預設不啟動同步，管理端設定見 [存取說明](docs/ACCESS_BOUNDARY.md)。
- Webhook 可能重送；沒有匯入某個月份，也不代表當月沒有成交。見 [資料匯入](docs/DATA_IMPORT.md)。
- 行政區界線是近似值，車站距離是直線距離。歷史行情不能當作未來報酬。
