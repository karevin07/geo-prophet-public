# GeoProphet

[![CI](https://github.com/karevin07/geo-prophet-public/actions/workflows/ci.yml/badge.svg)](https://github.com/karevin07/geo-prophet-public/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
![Go 1.26](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)
![PostgreSQL 16 + PostGIS](https://img.shields.io/badge/PostgreSQL_16-PostGIS-4169E1?logo=postgresql&logoColor=white)
![React 19](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=black)
![TypeScript](https://img.shields.io/badge/TypeScript-5.8-3178C6?logo=typescript&logoColor=white)
![MapLibre + deck.gl](https://img.shields.io/badge/MapLibre-deck.gl-396CB2?logo=maplibre&logoColor=white)
![Docker Compose](https://img.shields.io/badge/Docker-Compose-2496ED?logo=docker&logoColor=white)

[繁體中文](README.md) · [English](README.en.md)

把實價登錄的買賣、租賃和預售屋成交，連同門牌座標、軌道站點和生活機能放在同一張地圖，查成交、看行情、比較行政區，也能估算單一地址的價格和租金。

Go 負責下載、整理和匯入資料，PostgreSQL／PostGIS 儲存與查詢，React 呈現地圖和圖表。

![地圖：台北市的 H3 蜂巢網格依平均單價分色，疊上捷運路線與車站年增率](docs/screenshots/map.jpg)

| | |
| --- | --- |
| ![行情分析：多年單價走勢與每月成交量](docs/screenshots/analysis.jpg) | ![區域總覽：行政區的單價、租金、預售與生活機能](docs/screenshots/area.jpg) |
| **行情分析**：依建物型態、總價和坪數篩選，看多年單價與成交量。 | **區域總覽**：一個行政區的單價排名、租金報酬率、預售溢價和每平方公里設施數。 |
| ![租金：各行政區毛報酬率地圖與租金走勢](docs/screenshots/rents.jpg) | ![估價：以附近成交估算總價與月租](docs/screenshots/estimate.jpg) |
| **租金**：各區整戶出租租金、毛報酬率和走勢。 | **估價**：用附近同類成交與租約估總價和月租，附回測過的 80% 區間。 |
| ![預售建案：建案分布、成交統計與周邊中古屋比較](docs/screenshots/presales.jpg) | ![車站：周邊 500 公尺成交單價排行](docs/screenshots/stations.jpg) |
| **預售建案**：建案的銷售節奏、樓層價差和與周邊中古屋的價差。 | **車站**：捷運、台鐵、高鐵站周邊的單價排行與通車前後比較。 |

截圖來自匯入官方資料的本機部署。下面的試用模式使用合成資料，畫面上的數字會不同。

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/architecture/architecture-dark.png">
  <img src="docs/architecture/architecture-light.png" alt="GeoProphet：Go 資料匯入、PostGIS、Gin API、React 地圖與品質告警">
</picture>

[架構](ARCHITECTURE.md) · [開啟互動圖](docs/architecture/README.md)

## 在本機試用

先安裝 Node.js 24+、npm 和 Make，再執行：

```bash
npm ci
make dev-web
```

開啟 <http://localhost:3100>。這個模式使用合成資料，不需要資料庫或 API 金鑰。底圖仍需要網路；租金、預售等頁面可能沒有資料，CSV 匯出也未啟用。

### 使用真實資料

需要 Docker 和 Compose plugin。下面的指令會啟動本機 PostGIS，下載台北市（`CITY=a`）近一年的實價登錄與生活機能資料：

```bash
cp .env.example .env    # 把 POSTGRES_PASSWORD 改成自己的本機密碼
make docker
make import-city CITY=a SEASON=114S3-115S2
```

完成後開啟 <http://localhost:3100>。新北市是 `f`、桃園市是 `h`，可以寫成 `CITY=a,f,h`。`SEASON` 可以填一季（`115S2`）或區間；季數越多，下載和匯入越久。一年的資料（上例，台北市約 5 分鐘）足夠看地圖和行情；估價要用近 3 年的成交，建議填 `SEASON=112S3-115S2`。軌道站點需要 TDX 金鑰，另見[資料匯入](docs/DATA_IMPORT.md#大眾運輸站點)。下載前請先看[資料授權](docs/DATA_SOURCES.md)。

## 文件

[資料匯入](docs/DATA_IMPORT.md) · [API](docs/API.md) · [操作指南](docs/MAP_GUIDE.md) · [開發與資料庫設定](CONTRIBUTING.md) · [其他文件](docs/README.md)

## 資料與授權

Repo 只提供匯入程式和合成測試資料，不包含下載的官方資料或 API 憑證。使用 TDX 等來源時，需要自行設定憑證並確認[資料授權](docs/DATA_SOURCES.md)。頁面上的比較指標不代表未來漲幅。

程式與文件採 [MIT](LICENSE)；第三方資料和底圖依各自授權使用。
