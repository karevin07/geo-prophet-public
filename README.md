# GeoProphet

[繁體中文](README.md) · [English](README.en.md)

把實價登錄、門牌座標和軌道站點放在同一張地圖，查成交、看行情、比較行政區。

Go 負責下載、整理和匯入資料，PostgreSQL／PostGIS 儲存與查詢，React 呈現地圖和圖表。

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

## 文件

[資料匯入](docs/DATA_IMPORT.md) · [API](docs/API.md) · [操作指南](docs/MAP_GUIDE.md) · [開發與資料庫設定](CONTRIBUTING.md) · [其他文件](docs/README.md)

## 資料與授權

Repo 只提供匯入程式和合成測試資料，不包含下載的官方資料或 API 憑證。使用 TDX 等來源時，需要自行設定憑證並確認[資料授權](docs/DATA_SOURCES.md)。頁面上的比較指標不代表未來漲幅。

程式與文件採 [MIT](LICENSE)；第三方資料和底圖依各自授權使用。
