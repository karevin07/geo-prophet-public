# Data sources and licensing boundary

程式碼的 MIT 授權不取代資料、底圖或第三方套件的授權。

| 項目 | 公開 repo 的處理方式 |
| --- | --- |
| 實價登錄 | 保留 downloader／parser；未複製官方 CSV。三個測試 CSV 由 `scripts/generate-fixtures.mjs` 建構合成案例 |
| TDX | 保留 client／network loader；未複製官方 JSON fixture。loader 測試使用手工合成回應 |
| 政府門牌、健保、教育名單 | 保留 adapter；正式下載資料由使用者自行取得，不隨 repo 散布 |
| OpenStreetMap | 保留 OSM adapter；不提供 Taiwan PBF 或衍生資料庫 |
| CARTO／OSM 底圖 | 前端仍使用 CARTO 提供的樣式；需要網路並遵守服務與資料署名條件 |
| Mock 行情 | 程式生成的展示數據，不是官方或實際成交行情 |
| 線上 Demo | GitHub Pages 部署時從上述官方來源產生台北市近 3 年的成交、租金與預售統計和生活機能統計，只存在部署裡、不進 git；頁面標示實價登錄（政府資料開放授權）及 OpenStreetMap、健保署、教育部來源。見 [線上 Demo](DEMO.md) |

合成道路、編號與建案只供測試，不應用來定位真實物件。部分單元測試使用公開地名來驗證字串規則；這不代表包含完整官方資料集。

若自行匯入真實資料，請逐一查閱資料集的來源頁與再散布條款，記錄版本、取得時間及必要署名。此文件不宣稱已完成所有來源的再散布授權審查；公開 repo 刻意不包含下載資料。

參考：[政府資料開放授權](https://data.gov.tw/license)、[內政部實價登錄](https://lvr.land.moi.gov.tw/)、[TDX](https://tdx.transportdata.tw/)、[OSM copyright / ODbL](https://www.openstreetmap.org/copyright)、[CARTO attribution](https://carto.com/attributions)。
