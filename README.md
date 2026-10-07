# GeoProphet — Data Pipeline & Geospatial Analytics

[繁體中文](README.md) · [English](README.en.md)

台灣房產資料工程與地理分析作品：以 Go 建構多來源資料匯入流程，使用 PostgreSQL／PostGIS 保存處理後資料，再透過 Gin API 與 React 地圖呈現行情與資料品質。

這是開發專案的獨立公開展示版本，不包含原始開發歷史、正式環境設定、資料庫備份或下載的官方資料。公開版測試 CSV 與 TDX loader 測試使用合成案例；數值不是實際行情或投資建議。

## 工程亮點

| 主題 | 實作與閱讀入口 |
| --- | --- |
| Data Pipeline | 多來源解析、地址正規化、TWD97 → WGS84、定位精度、COPY staging＋upsert、匯入紀錄與重跑：[匯入流程](docs/DATA_IMPORT.md) |
| Data Quality | 筆數／定位率檢核、匯入涵蓋與樣本門檻、告警佇列、Webhook 重試：[架構](ARCHITECTURE.md) |
| Backend | Gin delivery／use case／repository 分層、PostGIS GiST、keyset pagination、CSV 串流：[API](docs/API.md) |
| Visualization | MapLibre＋Deck.gl＋H3、區域比較、月度趨勢、缺口視覺標示、React lazy loading 與 manual chunks：[操作指南](docs/MAP_GUIDE.md) |

## 功能與技術棧

- 地圖探索：成交位置、H3 蜂巢聚合、篩選與圖層設定。
- 交易動態：篩選與排序、分頁、明細抽屜、地圖定位與 API CSV 匯出。
- 行情與區域研究：多年趨勢、月度彙總、分組分析、單價分布與行政區比較。
- 投資研究：區域指標比較、比較工作台與流動性研究；研究指標不等於投資報酬預測。
- 資料營運：匯入紀錄、資料品質檢核與可設定的 Webhook 告警；管理功能需要額外的身分與資料庫設定。

前端使用 React、TypeScript、Vite、Tailwind CSS、MapLibre、Deck.gl、H3 與 Recharts；後端使用 Go、Gin 與 pgx；儲存層使用 PostgreSQL／PostGIS，搭配 SQL migrations、Docker Compose、Vitest 與 Go tests。

## 快速體驗：不需資料庫或 API 金鑰

需求：Node.js 24+、npm 與 Make。底圖仍需網路；mock 是本機產生的資料，不等於完全離線地圖。

```bash
npm ci
make dev-web
```

開啟 <http://localhost:3100>。此指令明確使用前端 mock 資料，網站不會呼叫正式資料服務。租金、預售、匯入狀態與部分投資功能需要資料庫資料，mock 可能顯示空狀態；CSV 在純前端 mock 模式停用。

若想查看 Go API 的 mock 實作，另開終端機執行 `make dev-api`（需要 Go 1.26.8+），以 `curl http://127.0.0.1:8080/healthz` 檢查。`make dev-web-api` 使用 Gin 的 `/api`；請勿與 `make dev-web` 同時占用同一埠號。

## PostgreSQL／PostGIS 本機環境

需求：Docker（含 Compose plugin）與 Make。

```bash
cp .env.example .env
# 先編輯 POSTGRES_PASSWORD：只使用 URL-safe 字元的本機測試密碼
make docker
```

Docker project 固定為 `geo-prophet-public`，web 僅綁 `127.0.0.1:3100`；API 與資料庫不發布主機埠號。資料卷與其他專案隔離，不會連到既有正式資料庫。首次啟動是空資料庫，不會自動下載資料或執行背景同步。停止但保留資料：`make docker-down`。

實際匯入須自行確認來源授權，並明確執行 [匯入指令](docs/DATA_IMPORT.md)。提供 importer CLI 供研究，不提供正式資料庫 dump。

## 資料來源與 TDX

保留內政部實價登錄、TDX 與 OSM 相關串接程式，但 repo 不附帶下載的官方資料或 API 憑證。

- 實價登錄：成交、租賃與預售資料解析；公開版 CSV fixtures 是手工建構的合成案例。
- TDX：保留 API client 與交通路網 loader；測試使用合成車站、出口與路線回應，沒有附帶官方 JSON fixtures。
- OSM：保留相關匯入能力；資料與底圖須依各自條款使用與標示來源。

若要使用真實 TDX 資料，需自行設定憑證並明確執行匯入；公開 Compose 預設不提供憑證或啟動同步。詳見 [資料來源與授權](docs/DATA_SOURCES.md) 與 [資料匯入](docs/DATA_IMPORT.md)。

## 開發驗證

```bash
make install
make lint
make test
make build
make help
```

前端執行 TypeScript 檢查、Vitest 與 Vite 建置；後端執行 vet、race tests 與 server／importer 建置。部分 Go 測試會啟動本機 HTTP 測試伺服器，需要允許 loopback socket。Go module path 保留既有的 package 身分，不代表外部依賴私有 repo。

## 已實作與尚未實作

- 已實作：資料解析與正規化、地理定位、批次寫入與去重、成功／失敗紀錄、品質告警、資料查詢與視覺化。
- 邊界：raw ZIP／CSV／TDX 回應尚未持久保存，分析以即時 SQL／use case 聚合為主；不是完整 raw／processed／analytical 三層或 lakehouse。
- 後續方向：原始來源版本與 checksum、可重播批次、增量分析彙總、可重現的效能 benchmark。

## 文件與公開邊界

[文件索引](docs/README.md) · [架構](ARCHITECTURE.md) · [資料來源](docs/DATA_SOURCES.md) · [發布政策](docs/PUBLIC_RELEASE.md) · [安全](SECURITY.md) · [貢獻](CONTRIBUTING.md)

本機展示環境不是正式部署範本。管理端預設無人可存取，不提供示範身分繞過；見 [存取邊界](docs/ACCESS_BOUNDARY.md)。

## 授權

專案原創程式碼與文件依 [MIT License](LICENSE) 提供。第三方套件、底圖與自行取得的資料仍依各自授權；本專案的 MIT 不重新授權政府、TDX 或 OSM 資料。
