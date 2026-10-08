# Architecture

本頁記錄 public repo 已實作的責任邊界。總覽見 [中文 README](README.md)／[English README](README.en.md)；[Archify 互動圖](docs/architecture/README.md) 提供系統總覽、資料匯入、查詢分析、品質告警四個視角。圖示不是正式部署拓撲，也不包含 private 存取設定。

## Responsibility boundaries

```mermaid
flowchart LR
    Sources[Source adapters] --> Parser[Go parser / normalization]
    Parser --> Geo[Geocoding / quality flags]
    Geo --> Staging[COPY staging]
    Staging --> DB[(PostgreSQL / PostGIS)]
    Parser --> Runs[Import run statistics]
    Runs --> Quality[Quality rules / alert deliveries]
    DB --> Repo[Postgres repository]
    Repo --> Usecase[Framework-independent use cases]
    Usecase --> Gin[Gin HTTP API]
    Gin --> Web[React / Vite / MapLibre / Deck.gl]
```

來源下載與解析位於 `api/internal/opendata`、`addresspoint`、`lvr`、`transit`、`facility`；排程與工作執行位於 `importer`。delivery 不將 `gin.Context` 傳入 use case，repository 使用標準 `context.Context` 與 pgx，讓取消傳遞至資料庫。

## Data model and lineage

買賣、租賃與預售分別存於 `transactions`、`rentals`、`presales`；`address_points` 和 `transit_*` 提供定位基礎。`import_runs` 記錄批次、來源／期別、成功失敗與寫入統計，`admin_jobs` 是人工工作的佇列，`quality_alert_deliveries` 保留通知與重試狀態。這些紀錄不是完整欄位級 lineage。

目前持久層主要是 processed data；原始 ZIP／CSV／TDX 回應尚未持久保存。`presale_projects` 是匯入時重建的衍生建案資料，其他分析多為查詢時彙總。沒有宣稱完整 medallion 或 lakehouse 架構。

以下為核心外鍵關係節選，不是完整 ERD；欄位依 [migrations](migrations/) 定義。門牌比對、預售建案聚合與交易到車站距離的計算關係，不等於全部都有資料庫外鍵。

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

成交／租賃／預售的 `import_run_id` 可為 NULL，圖中顯示的是有批次關聯時的外鍵方向。`transaction_station_distances.transaction_id` 是邏輯關聯，migration 未宣告對 `transactions` 的外鍵，因此未畫成外鍵關係。此表保存直線距離，不是路網步行距離。

## Query lifecycle

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

此圖針對 H3 查詢，並非所有 endpoint 都在 Go 聚合。月份行情與區域比較主要以 SQL 彙總。取消過期請求透過標準 context 傳至 repository；mock 模式不走此資料庫路徑。

## Reliability and quality

- 實價登錄依來源「編號」upsert；COPY staging 和寫入使用交易，重跑不應增加重複主鍵。
- 定位精度與特殊交易原因保留在資料中，分析採用明確的排除條件。
- 已完成的同步工作依成功期別跳過，失敗留下匯入紀錄並於後續同步重試。
- 工作佇列以 `FOR UPDATE SKIP LOCKED` 認領；部分唯一索引約束待執行工作。
- Webhook 採持久化送達狀態與退避重試，可能重送，接收端應去重，不宣稱 exactly-once。
- 流動性研究補齊月序列並檢查登錄匯入涵蓋；缺口的零筆不是實際市場零交易。

## Backend and visualization

PostGIS geography＋GiST 用於可視範圍和附近查詢，交易清單使用 keyset cursor，CSV 逐列串流且限制最多 50,000 筆。H3 目前在 Go 聚合；區域排行與趨勢主要在 PostgreSQL 彙總。

React 頁面和圖表以 lazy loading 載入，Vite manual chunks 分隔地圖、Deck.gl 與圖表依賴。前端取消過期請求、保留 URL 篩選狀態，並以樣本門檻、缺口斜線與空狀態避免誤讀資料。

## Tradeoffs and next steps

- Go 常駐同步與 PostgreSQL 佇列降低部署成本，沒有引入 Airflow／Kafka；更複雜的依賴 DAG 或高吞吐需求才重新評估。
- 即時 SQL 與 Go H3 聚合容易追溯口徑，但大範圍查詢需掃描較多資料；未來依 benchmark 選擇預聚合表或 tiles。
- raw 檔案保存、來源 checksum、版本化轉換與可重播，屬於下一階段，不能用目前的 upsert 取代。
- 近似行政區邊界不是法定界線，地理距離不是步行距離；投資數據不代表出售機率或未來漲幅。

## Deployment and security

公開 Compose 僅供本機展示：獨立 project／volume、web 綁 loopback、資料庫不發布埠號、預設不啟動背景匯入。管理 API 的可信身分邊界見 [存取邊界](docs/ACCESS_BOUNDARY.md)。CI 僅讀取權限與測試／建置，不持有正式部署或跨 repo 發布憑證。
