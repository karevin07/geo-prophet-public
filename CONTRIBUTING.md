# Contributing

## 開發

需要 Node.js 24+、Go 1.26.8+ 和 Make。Go 測試會用到 C 編譯器，也會啟動本機 HTTP server。

```bash
make install
make lint test build
```

不使用資料庫時，先執行 `make dev-api`，再另開終端機執行 `make dev-web-api`。API 的健康檢查是 `curl http://127.0.0.1:8080/healthz`。

前端使用 3100，不能和 `make dev-web` 同時啟動。

## 本機資料庫

先安裝 Docker 和 Compose plugin：

```bash
cp .env.example .env
# 編輯 POSTGRES_PASSWORD，使用 URL-safe 的本機測試密碼
make docker
```

開啟 <http://localhost:3100>。如果前端 dev server 還在執行，先關掉它。這組容器使用自己的資料卷，只有 web 綁定本機埠號，API 和資料庫不直接對外。

第一次啟動是空資料庫，不會自動下載或同步資料。接著依 [匯入指令](docs/DATA_IMPORT.md) 加入資料。管理頁預設無法存取，設定方式見 [存取說明](docs/ACCESS_BOUNDARY.md)。

執行 `make docker-down` 可以關閉容器，資料會保留。

## 修改與測試

測試使用合成資料和本機 mock server，不需要正式資料庫或 API 金鑰。

修改 parser 時，更新 `scripts/generate-fixtures.mjs` 並執行 `make fixtures`，再補上對應測試。不要把下載的真實資料放進 testdata。文件只描述已完成的功能；還沒做的部分要另外註明。

這份 repo 是從開發版本整理出的快照，不會自動同步。修正先合併到開發版本，再帶到這裡，避免兩份程式碼不同步。
