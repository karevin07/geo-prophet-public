# Access boundary

公開原始碼與公開正式服務是兩件事。這個 repo 不附正式環境的帳號、主機位址、憑證或存取設定。

## 後台

開發系統以可信的 Tailscale reverse proxy 身分標頭判斷管理者。API 以 `ADMIN_LOGINS` allowlist 判斷權限；一般 nginx port 80 移除外部送入的身分標頭。只有由可信 proxy 使用的 port 81 可保留身分標頭，該埠不應對外公開。

公開 Compose 不發布 port 81，且 `ADMIN_LOGINS` 為空，沒有任何管理者，因此 `/admin` 功能不屬於可直接操作的 Demo。不要為了展示而對外提供可偽造的管理者標頭或關閉身分驗證。需要部署管理端時，必須自行設定可信 proxy、網路邊界、allowlist 與工作執行服務；不在本機 Demo 的支援範圍內。

## 本機 Demo

web 僅綁 `127.0.0.1:3100`，PostgreSQL 與 API 只位於 Compose 內部網路。DB 密碼由 `.env` 提供，Demo DSN 請使用 URL-safe 字元。不要把 Docker socket、DB 或可信管理端直接發布到網際網路。

## 原則

- API 金鑰不進前端 `VITE_*` 變數或 Git。
- 不公開包含操作者／帳號資訊的後台截圖。
- 正式環境應另外配置 HTTPS、存取控制、備份、密碼管理與資源限制。
