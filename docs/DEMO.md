# 線上 Demo

回到 [文件索引](README.md)。

[線上 Demo](https://karevin07.github.io/geo-prophet-public/) 放在 GitHub Pages，使用台北市近 3 年實價登錄的快照，不連 API 或資料庫。公開 repo 的 Pages workflow 每週一及每次 main 更新時重新產生快照。

## 資料怎麼來

GitHub Actions 透過 `.github/workflows/pages.yml` 執行：

1. 用 Docker Compose 啟動一個只存在這次工作的 PostGIS。
2. `make import-city` 匯入台北市的門牌、生活機能，以及到兩季前為止 13 季的買賣、租賃和預售屋；再用 `importer sync` 補上最新一季和最新一期 10 日資料。
3. `make demo` 執行 `api/cmd/snapshot` 寫出快照，再用 `VITE_DATA_SOURCE=snapshot` 建置網站。
4. 上傳到 GitHub Pages。

匯入或建置失敗時不會部署，網站維持上一次成功的版本。官方資料只存在 Pages 的部署裡，不會進 git。

## 快照內容

`api/cmd/snapshot` 寫出三種檔案：

| 檔案 | 內容 |
| --- | --- |
| `meta.json` | 快照時間、成交日期範圍、最後完整登錄月份、最新一期 10 日資料、行政區清單；最後才寫，缺少它代表快照不完整 |
| `sales.json` | 近 3 年的買賣成交，一個欄位一個陣列（比逐筆存放小三分之一）；台北市約 4.6 萬筆，gzip 後約 1.9 MB |
| `api/<key>.json` | 參數固定的 API 回應：行政區邊界、各區租金與走勢、各區預售建案、生活機能統計 |

`<key>` 取請求路徑與排序後參數的 SHA-256 前 20 碼。Go（`snapshot.Key`）和前端（`snapshotKey`）使用相同算法；前端先組出 API 網址，再改讀對應的快照檔案。

## 瀏覽器端的計算

地圖、行情分析、行政區比較、區域總覽和交易動態的篩選條件可以任意組合，無法事先算好，改由 `src/lib/api/local-sales.ts` 在瀏覽器計算。

篩選條件、預設 12 個月的期間、年增率比較月份、仍在登錄中的暫定月份、六角格明細和交易動態排序，都沿用 Go handler、usecase 與 SQL 的規則。曾用同一份資料與正式 API 比對，結果相同。

唯一的差別是地圖範圍：API 用 PostGIS geography 的 `&&` 比對，會多包含畫面邊緣外一點點的成交；瀏覽器端只算畫面範圍內的。

開發時的 mock 模式（`VITE_DATA_SOURCE=mock`）也用同一套計算，只是資料是產生出來的。

## Demo 沒有的功能

| 功能 | 原因 |
| --- | --- |
| 估價 | 需要比對同棟與鄰近成交、門牌資料與回測，頁面改為說明 |
| 租金、預售、投資比較頁 | 篩選組合尚未放進快照，暫不顯示 |
| 車站與軌道圖層 | 需要 TDX 金鑰，公開 repo 沒有 |
| 生活機能與預售圖層 | 尚未放進快照 |
| CSV 匯出、後台 | 需要 API |

## 在本機建置

先照 README 匯入一個縣市，再執行：

```sh
make demo CITY=a
```

網站會寫到 `build/demo-site`，預設放在 `/geo-prophet-public/` 底下；用 `DEMO_BASE=/` 可以改放在根目錄。

## 啟用 GitHub Pages

在公開 repo 的 **Settings → Pages** 把 **Source** 設為 **GitHub Actions**，之後 workflow 就會自動部署。
