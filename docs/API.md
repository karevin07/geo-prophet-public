# GeoProphet HTTP API

GeoProphet API 由 Gin 提供，預設監聽 `:8080`。Docker 部署時瀏覽器應透過 web container 的同源 `/api` reverse proxy 存取，不需直接暴露 API port。

回到 [文件索引](README.md)。

## Base URL

- Vite（`make dev-web-api`）：`http://localhost:3100/api/v1`
- Docker（`make docker`）：`http://localhost:3100/api/v1`
- 直接呼叫 API：`http://localhost:8080/api/v1`

`make dev-web` 使用前端合成資料，不會呼叫 Go API。以下 cURL 範例需先啟動 Docker，或搭配執行 `make dev-api` 與 `make dev-web-api`；後者由 Go 提供合成資料。直接執行 `npm run dev` 的預設埠號則是 3000。

## 共用篩選參數

成交點位、H3 聚合、行情趨勢、市場摘要、行情分析、行政區比較、交易動態與 CSV 匯出端點都接受以下選填參數：

| 參數 | 格式與預設值 | 說明 |
| --- | --- | --- |
| `bbox` | `west,south,east,north` | WGS84 可視範圍；西、南必須小於東、北 |
| `min_price` | 非負整數，預設 `0` | 成交總價下限（新台幣） |
| `max_price` | 非負整數，預設無上限 | 成交總價上限（新台幣） |
| `min_area` | 非負數字，預設 `0` | 建物坪數下限，不含車位 |
| `max_area` | 非負數字，預設無上限 | 建物坪數上限，不含車位 |
| `types` | 逗號分隔 | `住宅大樓,華廈,公寓,透天厝,其他`；`其他` 代表前四類以外的全部 |
| `from` | `YYYY-MM` | 成交月份起點，預設為含本月的近 12 個月 |
| `to` | `YYYY-MM` | 成交月份終點，包含整個指定月份 |
| `include_special` | `true` 或 `false`，預設 `false` | 是否包含親友交易、分件登記與極端價格 |
| `exclude_new` | `true` 或 `false`，預設 `false` | 排除屋齡未滿 2 年（成交日距竣工日）的成交；竣工日未知者保留。適合跨時間比較，避免整批建案交屋改變成交組成 |

無效的數值、日期、bbox 或建物型態會回傳 `400 invalid_request`。

## 後台

後台 API 位於 `/api/v1/admin/*`，只接受 API 環境變數 `ADMIN_LOGINS` 中的 Tailscale 帳號。身分取自可信的 `tailscale serve` proxy 加上的 `Tailscale-User-Login` 標頭（見 [ACCESS_BOUNDARY.md](ACCESS_BOUNDARY.md#後台)）；其他人得到 `403 forbidden`，未使用資料庫時為 `503 not_available`。

公開 Compose 將 `ADMIN_LOGINS` 固定為空，也沒有啟動常駐 importer 或備份服務；只修改 `.env` 不會開啟後台。以下是另行部署可信管理端後的 API 行為，不是預設 Demo 的操作步驟。

### 目前身分

```http
GET /api/v1/admin/me
```

不需權限，回傳 `login`、`name`、`profilePic` 與 `admin`（是否在允許名單）。沒有經過 `tailscale serve` 時 `login` 為空字串。前端依此決定是否顯示「後台」入口與拒絕訊息。

### 系統狀態

```http
GET /api/v1/admin/system
```

回傳 `database`（資料庫大小、成交筆數與各資料表大小）、`disk`（備份目錄所在磁碟的總量、已用與可用位元組）、`backups`（完成的備份檔名、大小與時間，新到舊）、`backupKeep` 與 `backupIntervalHours`。API 未掛載備份目錄時 `disk` 為 `null`、`backups` 為空陣列。

### 工作佇列

```http
GET  /api/v1/admin/jobs
POST /api/v1/admin/jobs   {"kind": "sync"}
POST /api/v1/admin/jobs   {"kind": "seasons", "params": {"seasons": "113S1-114S4", "cities": ["a", "f"]}}
```

`POST` 排入一筆工作並回傳 `201`；`kind` 為 `sync`（完整同步）、`current_release`（重新匯入最新一期）、`seasons`（重新匯入指定季度）、`transit`（重新匯入軌道站點）或 `backup`（立即備份）。`backup` 需要另行啟動備份服務；其餘由以 `sync -interval` 啟動的常駐 importer 在同步間隔期間每 15 秒檢查並依序執行。沒有執行者時，工作只會留在佇列中。

`seasons` 需要 `params`：`seasons` 可為單季（`115S3`）、範圍（`113S1-114S4`）或以逗號分隔，最多 60 季；`cities` 為縣市代碼（`a`、`f`、`h`）且不可為空。本機手動匯入使用 `make import-sales SEASON=115S2 CITY=a`，也可改用 `import-rentals` 或 `import-presales`。不符時回傳 `400`。其他種類忽略 `params`。工作紀錄會帶回 `params`。同一種工作已在排隊或執行中時回傳 `409 conflict`；未知的 `kind` 回傳 `400`。

`GET` 列出最近 50 筆工作（新到舊），含 `status`（`queued`、`running`、`succeeded`、`failed`）、`requestedBy`（觸發者的 Tailscale 帳號）、排入／開始／結束時間、`result` 與失敗時的 `error`。匯入類工作的 `result` 為執行期間的 `runs`、`failedRuns`、`rowsUpserted`；備份為 `file` 與 `bytes`。執行者重啟時，原本執行中的工作會標為失敗。

### 通車日期

```http
GET /api/v1/admin/openings
PUT /api/v1/admin/openings   {"kind": "line", "targetId": "NTMC-Y", "openedOn": "2020-01-31", "source": "https://…"}
```

`GET` 回傳 `lines`（每條路線與其通車日期、來源、最後編輯者與時間；沒有紀錄代表 2015 年前通車）、`platforms`（另有通車日期的月台，例如比所屬路線晚通車的車站，附所屬路線的日期）與 `platformOptions`（所有月台，供新增時選擇）。

`PUT` 新增或修改一筆紀錄，`kind` 為 `line` 或 `platform`，`openedOn` 為 `YYYY-MM-DD`（1990–2100 年），`source` 最多 500 字；`openedOn` 為 `null` 時刪除該筆紀錄。成功時回傳更新後的完整清單，並立即重算路線、月台與車站的 `openedOn`、清除通車前後比較的快取；未知的路線或月台回傳 `404`。公開的車站 API 帶 `Cache-Control: max-age=3600`，瀏覽器最多一小時後才看到新日期。

### 跨站請求檢查

後台的 `POST` 與 `PUT` 另外檢查跨站請求偽造（CSRF）：`Sec-Fetch-Site` 必須為 `same-origin`、`Origin` 的主機須與請求相同，且內容須為 JSON（否則回傳 `415`）。`tailscale serve` 會替管理者瀏覽器發出的任何請求附上身分，這些檢查避免其他網站借用它。

### 資料同步狀態

```http
GET /api/v1/admin/data-status
```

回傳資料庫中的匯入概況，無查詢參數。`data.cities` 列出各縣市目前的成交總筆數、已有坐標的筆數、未來日期筆數（`futureDated`），以及不晚於資料庫今天日期的最新成交日；`recentRuns` 是最近 30 次匯入，`latestRuns` 是各資料集／縣市最後一次嘗試，`lastSuccessfulRuns` 是最後一次成功匯入。每筆紀錄有來源、資料集、期別、開始與結束時間、讀取／略過／upsert／定位失敗筆數；失敗的紀錄附錯誤全文（`error`），因此只開放給管理者。`lastSuccessfulRuns` 取每個資料集／縣市**期別最新**的成功紀錄，因此回補舊季度不會蓋過最新季度。

失敗的紀錄另有 `errorKind`，由錯誤訊息歸類：`rate_limited`（來源限流）、`unauthorized`（憑證被拒）、`unpublished`（季資料尚未公布）、`missing_input`（缺少前置資料，例如門牌或 TDX 設定）、`download`、`parse`、`database` 或 `other`。開始超過 6 小時仍未結束的 `running` 紀錄以 `interrupted` 回傳，代表匯入程序中途停止。

`data.coverage` 的 `missing` 只列出同一縣市**已成功匯入的最早與最新季度之間**缺少的季度。未公布的近期季度、早於最早成功季度的歷史資料，以及已從官方網站下架的十日一期都不會被推測為缺漏；空陣列不代表歷史資料完整。`rowsUpserted` 是本次寫入的筆數，包含原有交易的覆寫，**不可**解讀為新增筆數。

`data.quality` 供後台「資料品質」頁使用，包含 `metrics` 與 `alerts`。`metrics` 對每個縣市的 `sales`（季資料）及 `sales_release`（十日一期）各取期別最新的一次成功匯入；欄位有 `rowsRead`、`rowsSkipped`、`rowsUpserted`、`rowsGeocodeFailed`、`preciseCount`、`locatedCount`、`preciseRate`、`locatedRate`、`previousRowsRead`、`volumeChange`、`finishedAt`。比率分母是本次寫入筆數；`precise` 指門牌或同巷弄附近門牌，`located` 另含路段與行政區中心。舊紀錄若缺定位細項，比率為 `null`；沒有前一期時比較欄位為 `null`。比較只在同縣市、同資料集的不同期別之間進行，重新匯入同一期不會當成比較基準。

`alerts` 列出目前觸發的 `code`、`severity`、`cityCode`、`dataset`、`season`、`runId`。規則為：至少寫入 100 筆且定位率低於 90%（`low_location_rate`）或精確定位率低於 70%（`low_precise_rate`）；十日一期兩期各至少讀取 100 筆且本期少於前一期 65%（`volume_drop`）；最新一期登記期間結束已超過 21 天（`stale_release`，重匯舊資料不會重設時鐘）；或最近一次嘗試失敗（`import_failed`，來源尚未發布不計）。這是操作告警，不應直接解讀為房市變化。

此端點原本是公開的 `GET /api/v1/data/status`，現已移到後台。

## 查詢成交點位

```http
GET /api/v1/transactions?bbox=121.45,24.98,121.60,25.10&types=公寓,華廈&limit=500
```

額外參數：

| 參數 | 預設值 | 限制 |
| --- | --- | --- |
| `limit` | `500` | 最大 `2000`；非正數或超過上限時使用預設值 |

此端點只回傳定位精度為門牌、鄰近門牌或路段的成交，避免行政區中心點在地圖上形成錯誤熱區。

## H3 聚合

```http
GET /api/v1/aggregations/h3?bbox=121.45,24.98,121.60,25.10&resolution=9
```

| 參數 | 預設值 | 限制 |
| --- | --- | --- |
| `resolution` | `9` | `7`–`10` |

回傳欄位：

| 欄位 | 說明 |
| --- | --- |
| `hex` | H3 cell ID |
| `lat`、`lng` | cell 中心點 |
| `count` | 成交筆數 |
| `price` | 平均成交總價 |
| `avgArea` | 平均建物坪數，不含車位 |
| `avgPricePerPing` | 面積加權的每坪單價，車位價與車位坪數均排除 |

點位與 H3 都排除只定位到行政區中心的成交。H3 會逐筆串流所有符合篩選的成交並計算完整聚合，不再於 200,000 筆截斷；廣域、多年份查詢可能需要較長時間。

## H3 單格明細

```http
GET /api/v1/aggregations/h3/cell?hex=894ba0a4177ffff&bbox=121.45,24.98,121.62,25.2
```

地圖懸停或固定六角格時使用。`hex` 必填，須為 resolution 7–10 的 H3 cell；其餘套用共用篩選參數（`bbox` 不影響結果）。後端取出 cell 外接矩形內的成交，再以與 `/aggregations/h3` 相同的 H3 函式判斷是否落在 cell 內，因此數字與地圖上的六角格一致。

```json
{
  "data": {
    "hex": "894ba0a4177ffff",
    "count": 20,
    "avgPricePerPing": 1058000,
    "avgTotalPrice": 39580000,
    "avgArea": 39.1,
    "cityCode": "a",
    "district": "大安區",
    "road": "復興南路一段",
    "priceYoY": -0.011,
    "yoyThrough": "2026-02",
    "ageMix": [{"key":"0-5","count":7},{"key":"5-15","count":0},{"key":"15-30","count":4},{"key":"30+","count":9},{"key":"unknown","count":0}]
  }
}
```

- `district`、`road` 為 cell 內成交最常見的行政區與路段；地址無法解析路段時 `road` 為空字串。
- `priceYoY` 比較截至 `yoyThrough` 的 12 個月與前一年的面積加權單價；`yoyThrough` 為查詢結束月與最後完整登錄月份較早者，任一期少於 5 筆時為 `null`。
- `ageMix` 依成交時屋齡分為 `0-5`、`5-15`、`15-30`、`30+` 年與 `unknown`（無完工日，多為預售）。
- 無效或解析度不在 7–10 的 `hex` 回傳 `400 invalid_request`。

## 行情趨勢

```http
GET /api/v1/trends?bbox=121.45,24.98,121.60,25.10&from=2025-10&to=2026-09
```

趨勢由 PostgreSQL 按月份聚合，回傳面積加權的每坪單價與筆數。趨勢保留定位到行政區中心的成交，因為它們仍能代表區域行情。

每個月份另有 `provisional`：該月的成交可能仍在陸續登錄。判斷方式是由 `import_runs` 找出已匯入的季資料與其後連續的 10 日期資料，得到「登錄資料連續涵蓋到的日期」，再往前推 90 天；在那之前結束的月份才算完整。前端以虛線畫出 provisional 月份。

## 市場摘要

```http
GET /api/v1/summary?bbox=121.45,24.98,121.60,25.10&types=公寓&from=2026-01&to=2026-09
```

摘要套用與成交資料相同的價格、坪數、建物型態、日期、特殊交易與可視範圍篩選。和趨勢相同，摘要會保留只定位到行政區中心的成交。

回應不是集合，因此沒有 `meta`：

```json
{
  "data": {
    "count": 324,
    "avgPricePerPing": 712345.67,
    "medianTotalPrice": 16800000,
    "yoy": {
      "basis": "12m",
      "through": "2026-02",
      "count": -0.287,
      "avgPricePerPing": 0.018,
      "medianTotalPrice": 0.002
    }
  }
}
```

| 欄位 | 說明 |
| --- | --- |
| `count` | 符合條件的成交筆數 |
| `avgPricePerPing` | 面積加權的平均每坪單價（新台幣），排除車位價格與坪數 |
| `medianTotalPrice` | 成交總價中位數（新台幣） |
| `yoy` | 與去年同期相比的變化率，`0.018` 代表 +1.8% |

沒有符合條件的成交時，三個數值欄位都回傳 `0`。

年增率（`yoy`）的比較期間：

- 長度與查詢期間相同，至少 3 個月（`basis`，例如 `12m`；只選單一月份時為 `3m`）。
- 期間結束於查詢的最後一個月；若該月仍在登錄中，改為結束於最後一個完整月份（`through`，判斷方式同趨勢的 `provisional`）。因此近期資料不完整不會讓成交量年增率失真。
- 本期與去年同期都至少 5 筆成交時才回傳變化率，否則三個變化率為 `null`。
- 沒有 `from`/`to` 時 `yoy` 為 `null`；API 預設的 12 個月期間一定會帶 `from`/`to`。

## 行情分析

```http
GET /api/v1/analysis?from=2021-10&to=2026-09&types=公寓,華廈
```

使用共用篩選參數。若省略 `from`，此端點預設從含本月的近 60 個月起算（不同於其他端點的 12 個月）；省略 `to` 則截至本月。分析保留定位到行政區中心的成交，但排除沒有定位資料的成交。沒有符合條件的成交時，五個集合均為空陣列。

```json
{
  "data": {
    "monthly": [{"month":"2026-02","count":42,"avgPricePerPing":712345.67,"medianPricePerPing":700000,"medianTotalPrice":16800000,"avgArea":29.5,"provisional":false}],
    "categories": [{"category":"公寓","count":12,"avgPricePerPing":690000,"medianPricePerPing":680000,"medianTotalPrice":15000000}],
    "ageGroups": [{"key":"10_19","count":8,"avgPricePerPing":720000,"medianPricePerPing":700000,"medianTotalPrice":17500000}],
    "areaGroups": [{"key":"15_29","count":10,"avgPricePerPing":705000,"medianPricePerPing":695000,"medianTotalPrice":16000000}],
    "distribution": [{"minPricePerPing":600000,"maxPricePerPing":700000,"count":8}]
  }
}
```

`monthly` 依月份升冪排序，僅包含有成交的月份；`count` 為成交筆數，`avgPricePerPing` 是排除車位價與車位坪數的面積加權平均每坪單價，`medianPricePerPing` 是逐筆每坪單價中位數，`medianTotalPrice` 是含車位的成交總價中位數，`avgArea` 是不含車位的平均建物坪數。金額單位為新台幣、面積單位為坪。`provisional` 的判斷方式與行情趨勢相同。

`categories` 按建物型態分類；`ageGroups` 使用成交時屋齡，區間 key 為 `under5`、`5_9`、`10_19`、`20_29`、`30plus`、`unknown`；`areaGroups` 依建物坪數分組，key 為 `under15`、`15_29`、`30_44`、`45plus`。三種分組都提供筆數、面積加權平均單價、逐筆單價中位數及總價中位數，未出現的組別不回傳。`distribution` 以每 100,000 元／坪一組，區間包含下界、不包含上界；不回傳沒有成交的區間。

## 行政區比較

```http
GET /api/v1/districts/compare?city=a&districts=大安區,信義區&from=2023-10&to=2026-09
```

套用共用篩選參數。`city` 為 `a`（台北）、`f`（新北）或 `h`（桃園），預設 `a`；`districts` 為該縣市的行政區名稱，以逗號分隔，最多 4 個，不能重複。可從 `GET /api/v1/regions` 取得合法名稱。省略 `districts` 時仍回傳排行，但 `trends` 為空陣列。省略 `from` 時預設近 36 個月，省略 `to` 時截至本月。未知縣市、未知／重複行政區、超過 4 區或無效的共用篩選皆回傳 `400 invalid_request`。

```json
{
  "data": {
    "cityCode": "a",
    "yoyThrough": "2026-02",
    "ranking": [{"district":"大安區","count":3941,"avgPricePerPing":1092945.23,"medianPricePerPing":1036122,"medianTotalPrice":32200000,"priceYoY":0.05}],
    "trends": [{"district":"大安區","month":"2026-02","count":110,"avgPricePerPing":1050000,"provisional":false}]
  }
}
```

`ranking` 包含該縣市在所選期間內有符合條件成交的所有行政區，依逐筆每坪單價中位數降冪排序；`count` 為成交筆數，`avgPricePerPing` 是排除車位價與坪數的面積加權平均，`medianTotalPrice` 是含車位的成交總價中位數。`trends` 僅包含選定行政區有成交的月份，依月份升冪排序；`provisional` 與行情趨勢使用相同的登錄完整度判斷。

`priceYoY` 另外比較截至最後完整登錄月份（`yoyThrough`）的 12 個月，與前 12 個月的面積加權平均單價。此比較沿用可視範圍、價格、坪數、建物型態、特殊交易及縣市篩選，但不受排行期間的 `from`／`to` 限制；兩期各至少 5 筆且坪數總和大於零才回傳變化率，例如 `0.05` 為 +5%。資料涵蓋日期未知或樣本不足時，`priceYoY` 為 `null`；涵蓋日期未知時 `yoyThrough` 為空字串。排行與趨勢保留定位到行政區中心的成交，但排除未定位的成交。金額單位皆為新台幣，面積單位為坪。

## 區域投資觀察

```http
GET /api/v1/investment/areas?city=a&type=住宅大樓&min_area=15&max_area=45&min_age=5&max_age=30
```

這是獨立的同類住宅查詢，**不套用上方的共用篩選參數**。`city` 僅接受 `a`（臺北）、`f`（新北）、`h`（桃園），預設 `a`；`type` 只能指定一個 `住宅大樓`、`華廈`、`公寓` 或 `透天厝`，預設 `住宅大樓`。`min_area`／`max_area` 預設 15／45 坪，須滿足 `0 ≤ min_area < max_area ≤ 300`；`min_age`／`max_age` 預設 5／30 年，須滿足 `0 ≤ min_age < max_age ≤ 100`。兩個範圍均含下限、不含上限；未知屋齡不計。參數錯誤回傳 `400 invalid_request`。

只計住家用房地成交，排除特殊交易；行政區已知但尚未定位到門牌的成交仍計入。最近一期以**該縣市**成功匯入的季資料／十日一期推算最後完整登錄月份，再預留 90 天登錄延遲；`currentFrom` 至 `through` 為最近 12 個完整成交月份，`previousFrom` 起為前 12 個月。歷史匯入若有缺口，成交筆數仍可能偏低，請一併查閱後台涵蓋狀態。

```json
{
  "data": {
    "cityCode": "a", "through": "2026-02", "currentFrom": "2025-03", "previousFrom": "2024-03",
    "minimumSample": 30, "cityCurrentCount": 100, "cityPreviousCount": 95,
    "cityMedianPricePerPing": 800000, "cityVolumeChange": 0.0526, "cityPriceChange": 0.03,
    "areas": [{
      "district": "大安區", "currentCount": 40, "previousCount": 50,
      "medianPricePerPing": 900000, "volumeChange": -0.2, "priceChange": 0.0588,
      "relativeToCityMedian": 0.125,
      "sufficientCurrentSample": true, "sufficientComparisonSample": true
    }]
  }
}
```

上例為**格式示意，非實際行情**。`medianPricePerPing` 是符合條件成交的逐筆每坪單價中位數（新台幣／坪）；`relativeToCityMedian` 是相對同縣市、同類樣本中位數的比例，不是經屋況／位置調整的公允價值。`volumeChange` 與 `priceChange` 比較本期／前期，分別為筆數與中位單價變化。全市及各行政區本期至少 30 筆才顯示中位單價；兩期各至少 30 筆才顯示變化率，否則相關欄位為 `null`、樣本旗標為 `false`。`areas` 預設按本期筆數降冪排列，也保留前期有成交、本期為 0 的行政區。該縣市尚無成功匯入紀錄時，`through`、兩個起始月份為空字串，`areas` 為空陣列。

**解讀限制**：成交量不是待售去化天數；價格低於全市不代表被低估；歷史單價變化不是未來漲幅預測。本端點不含租金、空置、稅費或實際投資報酬；租金與毛報酬率見下節。

### 投資比較工作台

```http
GET /api/v1/investment/workbench?city=a&type=住宅大樓&min_area=15&max_area=45&min_age=5&max_age=30
```

參數、預設值與驗證規則同 `/investment/areas`，但期間取**成交與租賃兩者都完整**的最後月份。`currentFrom`–`through` 是同一組住宅條件的最近 12 個完整月份；`previousFrom` 起的前 12 個月只用於成交量與成交單價的歷史比較。兩項資料任一尚未匯入時，`through` 和起始月份為空字串，`areas` 為空陣列。無資料庫時回傳 `503 unavailable`。

```json
{
  "data": {
    "cityCode": "a", "through": "2026-02", "currentFrom": "2025-03", "previousFrom": "2024-03",
    "minimumSales": 30, "minimumRentals": 20,
    "areas": [{
      "district": "大安區", "currentCount": 40, "previousCount": 50,
      "medianPricePerPing": 900000, "volumeChange": -0.2, "priceChange": 0.0588,
      "relativeToCityMedian": null, "sufficientCurrentSample": true, "sufficientComparisonSample": true,
      "rentalCount": 25, "medianRentPerPing": 1200, "grossYield": 0.016
    }]
  }
}
```

上例僅示意格式，**不是實際行情**。成交只計住家用房地、非特殊交易；租金只計住家用整戶新簽租約，排除續租、特殊租約及社會住宅。兩者都依同縣市、屋型、坪數與屋齡篩選；坪數與屋齡含下限、不含上限。成交本期至少 30 筆才顯示中位單價，前後兩期各至少 30 筆才顯示變化率；租約至少 20 筆才顯示租金中位數。`grossYield = medianRentPerPing × 12 / medianPricePerPing`，任一中位數不足樣本則為 `null`。它是**不同房屋的行政區中位數之比**，不是任何個案的租金報酬率，也沒有扣稅、空置、費用或融資。租賃登錄不涵蓋所有自行出租案件，樣本不可視為整個租屋市場。成交筆數不等於去化天數，歷史價格變化不是預測。前端另有自行輸入成本的情境試算，未經 API 計算或保存。

### 區域流動性研究

```http
GET /api/v1/investment/liquidity?city=a&type=住宅大樓&min_area=15&max_area=45&min_age=5&max_age=30
```

沿用 `/investment/areas` 的縣市、單一屋型、坪數與屋齡參數、預設值及驗證規則，回傳 `{ "data": report }`。只計住家用房地、非特殊交易、屋齡已知且符合篩選條件的成交；不要求定位到門牌。屋齡為成交時屋齡，坪數與屋齡區間含下限、不含上限。需要資料庫；未設定時回傳 `503 unavailable`，不合法篩選回傳 `400 invalid_request`。

`through` 依該縣市成功匯入的買賣季資料與連續十日一期、再預留 90 天登錄延遲推算；`currentFrom`–`through` 是最近 12 個完整交易月份，`previousFrom` 起的前 12 個月為比較期。本端點僅使用買賣匯入紀錄，因此期間與租賃工作台可能不同。無可用季資料時，日期為空字串、`areas` 和 `incompleteMonths` 為空陣列，涵蓋旗標為 `false`。

回應的主要欄位：

| 欄位 | 意義 |
| --- | --- |
| `minimumSample` | 每期最低樣本數，固定 30 |
| `currentCoverageComplete`／`previousCoverageComplete` | 該期各月的登錄匯入區間是否均連續涵蓋 |
| `incompleteMonths` | 涵蓋不足的交易月份，按月份升冪 |
| `areas[].district` | 在這 24 個月內至少有一筆符合條件成交的行政區 |
| `currentCount`／`previousCount` | 兩期已取得的成交筆數；少量或匯入缺口仍回傳觀察值 |
| `activeMonths` | 本期至少有一筆符合條件成交的月份數，範圍 0–12 |
| `longestZeroRun` | 本期最長連續零筆月份數，包括期首、期末的連續零筆；不跨越本期起點 |
| `volumeChange` | 本期筆數 ÷ 前期筆數 − 1；例如 `0.1` 代表 +10% |
| `sufficientCurrentSample`／`sufficientComparisonSample` | 僅表示本期／兩期是否達 30 筆，與涵蓋旗標分開判斷 |
| `monthly` | 固定 24 筆、按月份升冪的 `{ month, count, coverageComplete }`，無已取得成交的月份補 0 |

`areas` 依本期筆數降冪，同筆數依行政區名稱排序。`activeMonths` 與 `longestZeroRun` 必須本期至少 30 筆且 `currentCoverageComplete=true` 才回傳數字，否則為 `null`；`volumeChange` 還要求前期至少 30 筆且 `previousCoverageComplete=true`。

**匯入缺口判斷**：逐月檢查「交易月首日」到「月底加 90 天」的登錄日期，能否被成功匯入的季資料與十日一期連續涵蓋；重複或重疊區間只算一次。無法完整涵蓋的月份仍回傳已取得筆數，但 `monthly[].coverageComplete=false`；0 筆不可視為該月實際沒有交易。旗標只確認匯入紀錄在上述延遲假設下連續，並不保證全部案件已揭露、解析或補登。

本功能研究**已揭露成交活動**。沒有待售庫存、上架日期、住宅存量或個別物件的銷售歷程，不能推算脫手天數、去化率、換手率或出售機率，也沒有投資評分。不同區域的筆數會受住宅存量影響。[內政部實價登錄 FAQ](https://lvr.land.moi.gov.tw/jsp/qa.jsp) 亦說明，揭露資訊僅描述發生交易的案件，且並非所有成交案件都能查到。

## 租金與毛報酬率

```http
GET /api/v1/rentals/districts?city=a&type=公寓&min_area=15&max_area=40&include_social=false
```

比較各行政區近 12 個完整月份的租金與同類住宅成交單價，同樣**不套用共用篩選參數**。`city` 為 `a`、`f`、`h`，預設 `a`；`type` 可省略（四種住宅類型合計）或指定 `住宅大樓`、`華廈`、`公寓`、`透天厝`；`min_area`／`max_area` 為坪數，含下限、不含上限，`max_area` 省略或為 0 代表不設上限，最大 300。`include_social=true` 時納入社會住宅包租代管租約，預設排除。需要資料庫；未連線時回傳 `503 not_available`。

租金只計**整戶出租的新簽租約**：排除套房與雅房、續租、特殊租約（例如部分面積出租、多個門牌合計租金、親友間租賃、期間內租金不同、極端租金與日期異常），預設也排除社會住宅（同區、同型態租金低約 15%）。112S4 以前沒有「出租型態」欄位，整戶出租由面積（至少 15 坪）與備註推定。成交只計同類型、同坪數的房地交易，排除特殊交易。

期間取買賣與租賃兩者都完整登錄的最後一個月往前 12 個月（`from`–`through`），規則與行情趨勢相同。

```json
{
  "data": {
    "cityCode": "a", "from": "2025-03", "through": "2026-02", "minimumSample": 20, "includeSocialHousing": false,
    "city": { "district": "", "rentals": 3200, "medianRentPerPing": 1180, "medianMonthlyRent": 32000,
              "sales": 6100, "medianPricePerPing": 720000, "grossYield": 0.0197 },
    "districts": [{
      "district": "大安區", "rentals": 410, "medianRentPerPing": 1420, "medianMonthlyRent": 42000,
      "sales": 520, "medianPricePerPing": 1050000, "grossYield": 0.0162
    }]
  }
}
```

上例為**格式示意**。`medianRentPerPing` 是月租金（不含另計的車位租金）除以租賃坪數的中位數；`grossYield` 為 `medianRentPerPing × 12 ÷ medianPricePerPing`，是毛報酬率，未扣除管理費、空置、稅費與修繕。租約或成交任一方少於 20 筆時，該方的中位數與 `grossYield` 為 `null`。`districts` 依租約筆數降冪排列。任一資料集尚未匯入時 `from`、`through` 為空字串、`districts` 為空陣列。

### 租金走勢

```http
GET /api/v1/rentals/trend?city=a&district=中山區&type=&min_area=&max_area=&include_social=false
```

篩選參數與上節相同；`district` 可省略。回傳全市（`city`）與指定行政區（`districtPoints`）的每月每坪月租中位數，每個月份合併當月與前兩個月的租約計算（3 個月滾動中位數），每點 `rentals` 為這 3 個月的租約數，少於 10 筆時 `medianRentPerPing` 為 `null`。走勢最長 60 個月，結束於租賃已登錄的最後一個月；晚於 `through`（最後完整月份）的點 `provisional` 為 `true`。

不含社宅（預設）時走勢從 **2023-12** 起：112S4 以前沒有「租賃住宅服務」欄位，社宅只能從備註辨識（新北約 43%，之後約 78%），沒被標出的社宅會讓「不含社宅」的租金在格式變更時假性跳升約 20%。`include_social=true` 時不受此限。

### 車站周邊租金

```http
GET /api/v1/transit/stations/TRTC-G14/rent?include_social=false
```

`rings` 依 300／500／800 公尺列出範圍內（到最近出入口的直線距離，只計定位到門牌者）近 12 個完整月份的租約數、每坪月租與月租中位數、同類住宅成交筆數與單價中位數、`grossYield`，以及每坪月租相對車站所在行政區（`districtMedianRentPerPing`）的差異 `relativeToDistrict`；`trend` 為該範圍的租金走勢，規則同上節。樣本門檻與行政區相同（20 筆）。不套用地圖的篩選條件，涵蓋四種住宅類型與所有坪數。未知站點回傳 `404`。

**解讀限制**：只有經包租代管業者或仲介的租約須登錄，房東自租的租約不在資料中；租金與房價取自不同物件的中位數，代表行政區整體水準，不是任何一間房子的報酬。許多租約的車位已含在租金內而未分開計價，會使每坪租金略為偏高。

## 預售屋建案

```http
GET /api/v1/presales/projects?city=a&district=中山區&months=24
GET /api/v1/presales/projects?lat=25.0526&lng=121.5204&radius=800&months=24
GET /api/v1/presales/projects?bbox=121.53,25.02,121.57,25.04&months=24
GET /api/v1/presales/project?key=a|文山區|元利四季莊園
```

預售屋以「建案」為單位：同縣市、同行政區、同建案名稱的交易合為一案（`key` 為 `city|district|name`），位置取該案最精確的一筆地址（`geocodePrecision` 為 `address`、`nearby`、`road` 或 `district`；約六成建案只登錄到路段）。108 年前的交易沒有建案名稱，不列入建案。需要資料庫（否則 `503`）。

`/presales/projects` 列出 `months`（預設 24、最多 240）個月內有簽約的建案，依最後簽約日新到舊，最多 1,000 案。每案含 `sales`（有效交易：房地交易、未解約、非特殊交易）、`cancelled`（解約筆數）、`recentSales`（近 12 個月有效交易）、首末簽約日、單價／總價／坪數中位數、最常見的建物型態與最高樓層。

帶 `lat`、`lng` 時改列出該點 `radius` 公尺內（預設 800、最多 3,000）定位到路段以上的建案，依距離由近到遠，最多 100 案，每案多一個 `distanceM`；此時忽略 `city` 與 `district`。車站卡片用它列出周邊預售建案。帶 `bbox`（經緯度範圍各不超過 1.2 度）時列出範圍內定位到路段以上的建案，依最後簽約日新到舊，最多 1,000 案；地圖的預售建案圖層使用。

`/presales/project` 另含 `lowPricePerPing`／`highPricePerPing`（有效交易單價的第 10／90 百分位）、`mainRooms`（最常見的房數）、`months`（每月有效交易數與單價中位數）、`floors`（各樓層單價中位數）、`units`（全部交易，含棟及號、樓層、總價、車位價、解約與特殊原因，最多 3,000 筆）與 `resale`：以建案最後 12 個月簽約的單價中位數，比較 1 公里內近 12 個完整月份的中古屋成交（全部屋齡，與 5 年內新成屋），各需至少 10 筆，`premium`／`newPremium` 為建案單價相對中古屋的溢價。建案只定位到行政區時 `resale` 為 `null`；只定位到路段時位置可能偏離數百公尺，請一併參考 `geocodePrecision`。未知的 `key` 回傳 `404`。

`transactedOn` 為簽約日。110S3（2021 年 7 月）以前，預售屋在代銷合約結束後才整批登錄，常延遲數年，舊建案的月份分布反映簽約時間，但當時的資料在簽約後很久才公開。

## 生活機能設施

```http
GET /api/v1/facilities?bbox=121.51,25.04,121.53,25.06&categories=park,convenience
GET /api/v1/facilities/nearby?lat=25.0526&lng=121.5204&radius=500
GET /api/v1/facilities/density?bbox=121.45,24.98,121.62,25.10&res=9
GET /api/v1/facilities/districts?city=a
```

設施類別與群組：

| 群組 | 類別 | 來源 |
| --- | --- | --- |
| `daily` 生活便利 | `convenience` 便利商店、`supermarket` 超市／量販、`market` 傳統市場 | OpenStreetMap |
| `education` 教育 | `elementary` 國小、`junior_high` 國中、`senior_high` 高中、`university` 大專 | 教育部學校名錄 |
| `medical` 醫療 | `hospital` 醫院、`clinic` 診所 | 健保特約醫事機構 |
| `leisure` 休閒 | `park` 公園、`library` 圖書館 | OpenStreetMap |
| `nuisance` 嫌惡設施 | `gas_station` 加油站、`substation` 變電所、`funeral` 殯儀館／火葬場、`cemetery` 墓地、`waste` 垃圾轉運站／掩埋場 | OpenStreetMap |

需要資料庫（否則 `503`）。`bbox` 的經緯度範圍各不得超過 1.2 度。

- `/facilities`：範圍內指定類別（省略時為全部）的設施，最多 5,000 筆，超過時 `truncated` 為 `true`。每筆含類別、名稱、品牌、縣市、行政區、地址（政府名單）、座標與面積（`areaM2`，公園等範圍）。
- `/facilities/nearby`：每個類別在 `radius` 公尺內（100–2,000，預設 500）的數量 `count`，以及 2 公里內最近一處的名稱與距離 `nearestM`（2 公里內沒有時為 `null`）。列出全部類別，依上表順序。公園、墓地等有範圍的設施量到範圍邊緣，位於範圍內為 0 公尺。距離為直線距離。
- `/facilities/density`：H3 格的機能熱度，`res` 為 8 或 9，`categories` 可限定計入的類別（省略時為全部）。每格的 `score` 是該格與周邊約 500 公尺內（第 9 層取 2 圈鄰格；第 8 層的格子本身半徑約 460 公尺，不取鄰格）出現的**正面類別數**（嫌惡設施以外的 11 類，0–11），`groups` 為各群組的類別數，`nuisances` 為同範圍的嫌惡設施數。只回傳中心點在 `bbox` 內且附近有設施的格子。
- `/facilities/districts`：縣市（`a`、`f`、`h`）各行政區的設施數（`counts`，依類別）與面積 `areaKm2`（行政區範圍的球面面積，含山區），以及全市合計 `city`，供計算每平方公里密度。行政區依名稱排序。

## 單一地址估價

```http
GET /api/v1/estimate?address=台北市大安區仁愛路四段27巷3號&lat=25.0389&lng=121.5453&type=華廈&area=32.5&age=53&floor=2&total_floors=6&parking=true
GET /api/v1/estimate/building?address=台北市大安區仁愛路四段27巷3號&lat=25.0389&lng=121.5453
```

以附近同類住宅的成交估算一戶住宅的行情，**不套用共用篩選參數**，需要資料庫（否則 `503`）。`lat`、`lng` 必填且須在台北、新北、桃園範圍內；`type` 為 `住宅大樓`、`華廈`、`公寓` 或 `透天厝`；`area` 為不含車位的建物坪數（3–300）。`address` 用來辨識同棟成交（取到「號」為止）與縣市、行政區，通常來自地點搜尋的門牌結果；`age`（0–100）、`floor`、`total_floors`、`parking` 可省略。

估價方式：

1. 取 1 公里內、近 36 個月、同建物型態、定位到門牌的房地成交，排除特殊交易；坪數為標的的 0.6–1/0.6 倍，屋齡（兩者都已知時）相差 10 年內。標的不在一樓時排除一樓成交，在一樓時只用一樓成交（透天厝除外），因為一樓常是店面。
2. 每筆單價依全市房價指數（3 個月滾動中位數）調整到最後完整登錄月份 `asOf`；大樓與華廈每差一層調整 0.5%。
3. 權重為距離（每 300 公尺）、坪數（對數比每 0.3）、成交時間（每 18 個月）、屋齡（每 8 年）各自的指數衰減相乘，同棟成交再乘 10；估價取加權中位數，`spread` 為加權四分位距除以估價。
4. 可比成交少於 5 筆時 `sufficient` 為 `false`，價格欄位為 `null`。

`confidence`：同棟至少 3 筆且 `spread` < 0.15 為 `high`；可比成交至少 20 筆且 `spread` < 0.25 為 `medium`；其餘為 `low`。`rangeLow`／`rangeHigh` 與 `backtestMedianError` 使用開發版本記錄的歷史回測校準值，不會依使用者匯入的資料重新回測。

以下成交與租金數字沿用開發版本的回測紀錄。本公開 repo 沒有附上當時的資料快照與重現腳本，因此無法在這裡獨立驗證，也不代表合成 Demo 或其他地區、期間的準確度。成交紀錄使用 2026 年 1–6 月 3,000 筆成交，每筆只用之前的成交估價：

| `confidence` | 占比 | 誤差中位數 | 80% 區間 |
| --- | --- | --- | --- |
| `high` | 16% | 3.9% | −11.4% ～ +7.9% |
| `medium` | 62% | 7.3% | −17.0% ～ +14.0% |
| `low` | 22% | 11.8% | −23.2% ～ +31.6% |

整體誤差中位數 7.4%（大樓 5.8%、華廈 8.0%、公寓 10.7%、透天厝 11.7%），同條件下行政區中位數為 16.0%。

`totalPrice` = `pricePerPing` × `area`，加上 `parking=true` 時的 `parkingPrice`：同行政區、同型態近 24 個月分開計價車位的中位數，不足 10 筆時改用全市。`lowPrice`／`highPrice` 只對建物部分套用區間。`comparables` 依權重列出最多 100 筆（`comparableSales` 為實際筆數），含原單價、調整後單價、距離、是否同棟與權重占比。

`rent` 為同一戶的租金估算（價格不足時為 `null`）：取 1.5 公里內、近 36 個月、同建物型態、定位到門牌的**整戶新簽租約**（不含續租、特殊租約與社宅），坪數與屋齡條件同上，權重相同但距離每 500 公尺衰減，依全市租金指數調整到最後完整登錄月份 `asOf`，取加權中位數。`rentPerPing` × `area` 四捨五入到百元為 `monthlyRent`；`grossYield` 為 `monthlyRent` × 12 ÷ `buildingPrice`（不含車位），`districtGrossYield` 為同行政區、同型態近 12 個月的毛報酬率（同 `/rentals/districts`）。`leases` 依權重列出最多 20 筆。透天厝登錄的租約太少，`supported` 為 `false`；可比租約少於 5 筆時 `sufficient` 為 `false`。

租金信心等級：同棟至少 2 筆且 `spread` < 0.15 為 `high`；至少 15 筆且 `spread` < 0.25 為 `medium`；其餘 `low`。以 2026 年 1–6 月 2,500 筆整戶租約回測（每筆只用之前的租約）：

| `confidence` | 占比 | 誤差中位數 | 80% 區間 |
| --- | --- | --- | --- |
| `high` | 16% | 5.0% | −10.3% ～ +10.6% |
| `medium` | 39% | 9.5% | −19.5% ～ +22.8% |
| `low` | 45% | 14.1% | −26.2% ～ +36.9% |

整體誤差中位數 9.8%（大樓 8.4%、華廈 11.2%、公寓 15.7%），3 公里內同類租金中位數為 17.8%。另外測試過以到車站、公園、嫌惡設施的距離與周邊超商數調整房價或作為權重，回測誤差都沒有改善（7.32% → 7.35–7.55%），因此估價不使用生活機能，只在頁面上列出。

`/estimate/building` 回傳門牌 150 公尺內、地址以該門牌開頭的成交筆數、最常見的建物型態與完工年份、最高總樓層與最近成交日，供表單自動帶入；地址沒有「號」時回傳 `400`。

**解讀限制**：資料不含裝潢、屋況、景觀、座向與增建；估價是參考行情，不是不動產估價師的鑑價。

## 行政區範圍

```http
GET /api/v1/districts/boundaries?city=a
```

回傳 `city`（預設 `a`）各行政區的近似範圍，格式為 GeoJSON `FeatureCollection`；每個 feature 的 `properties` 含 `district` 與 `cells`（組成的 H3 格數），`geometry` 為不含孔洞的 `MultiPolygon`。範圍由門牌坐標推算（見 [DATA_IMPORT.md](DATA_IMPORT.md#行政區範圍)），只在門牌更新時改變，因此回應帶 `Cache-Control: public, max-age=3600`。未知縣市回傳 `400 invalid_request`；尚未計算時回傳空的 `features`。

```json
{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": { "district": "大安區", "cells": 14 },
      "geometry": { "type": "MultiPolygon", "coordinates": [[[[121.53, 25.03], [121.54, 25.03], [121.54, 25.04], [121.53, 25.03]]]] }
    }
  ]
}
```

## 大眾運輸

站點與路線來自 TDX（見 [DATA_IMPORT.md](DATA_IMPORT.md#大眾運輸站點)），尚未匯入時各清單為空。

### 路線

```http
GET /api/v1/transit/lines
```

回傳 GeoJSON `FeatureCollection`，每條路線一個 feature。`properties` 含 `id`（如 `TRTC-BL`）、`operator`、`system`（`metro`、`lrt`、`tra`、`thsr`）、`name`、`nameEn`、`color` 與 `openedOn`；`geometry` 為簡化到約 5 公尺的 `MultiLineString`，台鐵與高鐵只保留涵蓋縣市附近的路段。回應帶 `Cache-Control: public, max-age=3600`。

`openedOn`（`YYYY-MM-DD`）只記錄 2015 年以後通車的路線與車站，由後台的「通車日期」頁維護；`null` 代表更早通車，早於實價登錄資料起點。

### 車站

```http
GET /api/v1/transit/stations?city=f
```

`city` 可省略，省略時回傳全部縣市。轉乘站合併為一站，例如板橋包含台鐵、高鐵、板南線與環狀線；站點 `id` 取第一個月台的 ID。`lines` 列出各月台所屬路線與該月台的通車日期，站點的 `openedOn` 是最早通車的月台，只要任一月台在 2015 年前通車就是 `null`。

```json
{
  "data": [
    {
      "id": "TRTC-BL07", "name": "板橋", "nameEn": "Banqiao", "cityCode": "f", "district": "板橋區",
      "lat": 25.013825, "lng": 121.462305, "systems": ["metro", "tra", "thsr"], "openedOn": null,
      "lines": [
        { "lineId": "NTMC-Y", "name": "環狀線", "color": "#fedb00", "platformId": "NTMC-Y16", "platformName": "板橋", "sequence": 10, "openedOn": "2020-01-31" },
        { "lineId": "TRTC-BL", "name": "板南線", "color": "#0a59ae", "platformId": "TRTC-BL07", "platformName": "板橋", "sequence": 7, "openedOn": null }
      ]
    }
  ],
  "meta": { "count": 104 }
}
```

### 車站周邊行情

```http
GET /api/v1/transit/stations/TRTC-BL07/trend
```

比較距離車站最近出入口 300、500、800 公尺內的成交（外圈包含內圈）。距離為直線距離，只計入定位到門牌（`address`、`nearby`）的交易。接受共用篩選參數，但忽略 `bbox`。

每個 `rings` 項目含 `count`、`avgPricePerPing`（面積加權）、`medianPricePerPing`（以上依 `from`／`to` 期間計算，預設近 12 個月）、`trends`（每月面積加權單價與筆數，**固定涵蓋全部年份**以跨過通車日期；登錄未完整的月份標記 `provisional`）與 `priceYoY`。年增率與行政區比較相同：比較截至 `yoyThrough` 的 12 個完整月份與前一年同期，任一期少於 5 筆時為 `null`。未知站點回傳 `404 not_found`。

```json
{
  "data": {
    "station": { "id": "TRTC-BL07", "name": "板橋", "...": "..." },
    "yoyThrough": "2026-02",
    "rings": [
      { "radius": 300, "count": 38, "avgPricePerPing": 690000, "medianPricePerPing": 668000, "priceYoY": 0.054,
        "trends": [{ "month": "2016-01", "avgPricePerPing": 512000, "count": 4, "provisional": false }] }
    ]
  }
}
```

### 各站年增率

```http
GET /api/v1/transit/stations/yoy?radius=500
```

一次回傳每個車站 `radius`（`300`、`500` 或 `800`，預設 `500`）公尺內的年增率，供地圖為車站上色。套用建物類型、價格、面積與特殊交易篩選，忽略 `bbox`、`from`、`to`；比較期間同上。`count` 是近 12 個完整月份的成交筆數；少於 5 筆時 `priceYoY` 為 `null`。沒有配對成交的車站不會列出。

```json
{
  "data": {
    "radius": 500,
    "yoyThrough": "2026-02",
    "stations": [{ "stationId": "TRTC-BL07", "count": 81, "priceYoY": 0.11 }]
  }
}
```

### 車站排行

```http
GET /api/v1/transit/stations/ranking?radius=500&from=2025-10
```

回傳每個車站 `radius`（`300`、`500`、`800`，預設 `500`）公尺內，在 `from`／`to` 期間（預設近 12 個月）的 `count`、`avgPricePerPing`、`medianPricePerPing` 與每月 `trend`，以及近 12 個完整月份的 `priceYoY`（任一期少於 5 筆時為 `null`）。套用共用篩選但忽略 `bbox`；期間內沒有成交的車站不列出。車站名稱、路線與行政區請與 `GET /api/v1/transit/stations` 對照，由前端篩選與排序。

### 距離與房價

```http
GET /api/v1/transit/distance-curve?from=2025-10&exclude_new=true
```

依縣市，將 `from`／`to` 期間（預設近 12 個月）定位到門牌的成交，按到最近車站出入口的直線距離分組：每 100 公尺一組至 800 公尺（`fromM`、`toM`），超過 800 公尺為最後一組（`toM` 為 `null`）。每組回傳 `count`、`avgPricePerPing`（面積加權）與 `medianPricePerPing`。套用共用篩選但忽略 `bbox`。

### 通車前後比較

```http
GET /api/v1/transit/openings?radius=500
```

列出 2015 年以後每條路線在每個車站的通車事件（既有車站新增路線也算，例如板橋的環狀線），比較通車月份前 12 個月與通車月份起 12 個月：

- `station`：車站 `radius` 公尺內的成交筆數、面積加權單價與變化率（`beforeCount`、`afterCount`、`beforePrice`、`afterPrice`、`change`）。
- `baseline`：同縣市同行政區、定位到門牌、但不在該車站 800 公尺內的成交，欄位相同。台北市幾乎每處都在某個車站 800 公尺內，因此基準只排除這個車站周邊，不排除其他車站。
- `relative`：`station.change − baseline.change`。
- `status`：`ok`；`too_recent`（通車後一年尚未完整登錄）；`insufficient`（任一期任一方少於 10 筆），後兩者 `relative` 為 `null`。
- `series`：通車前 36 個月至後 36 個月（不超過最後完整登錄月份）的每月車站與基準單價、筆數，`offset` 為距通車月份的月數。

套用建物型態、價格、面積、特殊交易與 `exclude_new` 篩選，忽略 `bbox`、`from`、`to`。車站頁預設帶 `exclude_new=true`：例如機場捷運體育大學站通車前一年 500 公尺內的 1,729 筆成交中有 1,728 筆是新成屋，未排除時會得到 +35 個百分點這類反映建案交屋而非房價的結果。結果只隨匯入改變，API 會快取 10 分鐘。這是觀察比較，不代表因果：通車前的預期可能已反映在房價，車站周邊的成交組成也可能改變。

## 地點搜尋

```http
GET /api/v1/places?q=台北市大安區敦化南路一段100號
```

| 參數 | 必填 | 限制 |
| --- | --- | --- |
| `q` | 是 | 正規化並移除空白後至少 2 個 Unicode 字元 |

搜尋會正規化臺／台、全形數字等台灣地址文字，可查詢行政區、道路或完整門牌。目前搜尋範圍為已支援門牌資料的台北市、新北市與桃園市，最多回傳 8 筆；完整門牌命中時會優先於道路結果。PostgreSQL 模式的結果取自 `address_points`，因此首次門牌資料匯入完成前可能回傳空集合。

```json
{
  "data": [
    {
      "kind": "address",
      "label": "台北市大安區敦化南路一段100號",
      "lat": 25.04,
      "lng": 121.55
    }
  ],
  "meta": {
    "count": 1
  }
}
```

`kind` 可能是 `district`、`road` 或 `address`。查詢不足 2 字時回傳 `400 invalid_request`。

## 交易動態

```http
GET /api/v1/feed?city=a&district=大安區&from=2026-01&to=2026-09&sort=date_desc
```

交易動態套用共用篩選參數，並提供以下額外參數：

| 參數 | 格式與預設值 | 說明 |
| --- | --- | --- |
| `city` | 縣市代碼 | 目前為 `a`（台北市）、`f`（新北市）或 `h`（桃園市） |
| `district` | 行政區名稱 | 必須同時指定 `city`，且行政區必須屬於該縣市 |
| `latest` | `true` 或 `false`，預設 `false` | 只回傳最新一期 10 日資料首次寫入的成交 |
| `sort` | 預設 `date_desc` | `date_desc`、`date_asc`、`price_per_ping_desc`、`price_per_ping_asc` 或 `total_price_desc` |
| `cursor` | opaque string | 載入下一頁時原樣帶回上一頁的 `meta.nextCursor` |

每頁最多 50 筆，使用 keyset pagination；游標分頁時應保留與第一頁相同的篩選及排序。第一頁範例：

```json
{
  "data": [
    {
      "id": "A123",
      "cityCode": "a",
      "district": "大安區",
      "address": "台北市大安區仁愛路四段1號",
      "totalPrice": 20000000,
      "buildingAreaPing": 30.5,
      "pricePerPing": 655738,
      "transactedAt": "2026-09-01T00:00:00Z",
      "isNew": true
    }
  ],
  "meta": {
    "total": 60,
    "newCount": 2,
    "nextCursor": "eyJ2IjoiMjAyNi0wOS0wMSIsImlkIjoiQTEyMyJ9",
    "latestRelease": {
      "label": "1150901-1150910",
      "registeredFrom": "2026-09-01T00:00:00Z",
      "registeredTo": "2026-09-10T00:00:00Z"
    }
  }
}
```

`data` 使用與成交點位相同的完整交易欄位，另以 `isNew` 表示該筆是否由最新一期匯入。`total` 與 `newCount` 只在第一頁計算；後續頁面省略。沒有下一頁時省略 `nextCursor`；尚未匯入 10 日資料時 `latestRelease` 為 `null`。

## CSV 匯出

```http
GET /api/v1/export.csv?city=a&district=大安區&from=2026-01&to=2026-09&sort=date_desc
```

CSV 匯出接受與交易動態相同的共用篩選、區域、`latest` 與 `sort` 參數，但不接受分頁游標。輸出依指定順序最多 50,000 筆，採串流寫入，不會先將全部資料載入記憶體。

- Content-Type：`text/csv; charset=utf-8`
- 檔名：`geoprophet-transactions-YYYYMMDD.csv`
- 編碼：UTF-8，檔案開頭包含 BOM，可直接用 Excel 開啟
- 欄位：交易編號、縣市／行政區、地址、成交資訊、價格、坪數、格局、樓層、備註、定位資料與本期新增標記

## 行政區清單

```http
GET /api/v1/regions
```

此端點不接受篩選參數，回傳交易動態可用的縣市代碼與行政區，行政區依政府代碼排序：

```json
{
  "data": [
    {
      "code": "a",
      "name": "台北市",
      "districts": ["松山區", "信義區", "大安區", "中山區", "中正區", "大同區", "萬華區", "文山區", "南港區", "內湖區", "士林區", "北投區"]
    },
    {
      "code": "f",
      "name": "新北市",
      "districts": ["板橋區", "三重區", "中和區", "永和區", "新莊區", "新店區", "樹林區", "鶯歌區", "三峽區", "淡水區", "汐止區", "瑞芳區", "土城區", "蘆洲區", "五股區", "泰山區", "林口區", "深坑區", "石碇區", "坪林區", "三芝區", "石門區", "八里區", "平溪區", "雙溪區", "貢寮區", "金山區", "萬里區", "烏來區"]
    },
    {
      "code": "h",
      "name": "桃園市",
      "districts": ["桃園區", "中壢區", "大溪區", "楊梅區", "蘆竹區", "大園區", "龜山區", "八德區", "龍潭區", "平鎮區", "新屋區", "觀音區", "復興區"]
    }
  ],
  "meta": {
    "count": 3
  }
}
```

## 回應格式

成功的集合回應：

```json
{
  "data": [],
  "meta": {
    "count": 0
  }
}
```

錯誤回應：

```json
{
  "error": {
    "code": "invalid_request",
    "message": "invalid input: bbox must contain west,south,east,north",
    "requestId": "..."
  }
}
```

## 健康檢查

| Endpoint | 用途 |
| --- | --- |
| `GET /healthz` | Process 存活檢查，不查外部依賴 |
| `GET /readyz` | Readiness 檢查；會驗證目前 repository／資料庫連線 |

## cURL 範例

```bash
curl --get --data-urlencode 'types=住宅大樓' --data-urlencode 'limit=10' 'http://localhost:3100/api/v1/transactions'
curl 'http://localhost:3100/api/v1/aggregations/h3?resolution=9'
curl 'http://localhost:3100/api/v1/trends?from=2025-10&to=2026-09'
curl --get --data-urlencode 'types=公寓' --data-urlencode 'from=2026-01' --data-urlencode 'to=2026-09' 'http://localhost:3100/api/v1/summary'
curl --get --data-urlencode 'q=台北市大安區敦化南路一段100號' 'http://localhost:3100/api/v1/places'
curl --get --data-urlencode 'district=大安區' 'http://localhost:3100/api/v1/feed?city=a&sort=date_desc'
curl --get --data-urlencode 'district=大安區' --output transactions.csv 'http://localhost:3100/api/v1/export.csv?city=a'
curl 'http://localhost:3100/api/v1/regions'
curl 'http://localhost:3100/readyz'
```
