# Architecture

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/architecture/architecture-dark.png">
  <img src="docs/architecture/architecture-light.png" alt="GeoProphet 資料工程、後端與前端責任邊界">
</picture>

[互動版](docs/architecture/README.md) · [匯入與告警流程](docs/DATA_IMPORT.md) · [API](docs/API.md)

## 核心資料關係

外鍵節選；完整結構見 [migrations](migrations/)。

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

交易的 `import_run_id` 可為 NULL；交易與車站距離表的 `transaction_id` 僅是邏輯關聯，未畫成外鍵。

## H3 查詢流程

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

H3 在 Go 聚合；行情與區域比較主要由 SQL 彙總。Mock 不走此流程。

## 必要限制

- 已持久保存 processed data；raw 保存、重播與完整欄位級 lineage 尚未實作。
- 總覽是功能責任圖，非正式部署拓撲；公開環境預設不啟動同步。見 [存取邊界](docs/ACCESS_BOUNDARY.md)。
- Webhook 可能重送；資料缺口不等於零交易。見 [品質與匯入說明](docs/DATA_IMPORT.md)。
- 區界為近似值、距離為直線距離；研究指標不是未來報酬預測。
