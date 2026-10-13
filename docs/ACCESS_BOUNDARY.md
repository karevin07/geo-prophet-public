# Access boundary

Repo 公開原始碼，但不附正式環境的帳號、主機位址、憑證或存取設定，也不代表提供公開的正式服務。

## 後台

開發系統使用可信的 Tailscale reverse proxy 提供身分標頭，API 再以 `ADMIN_LOGINS` 允許名單判斷管理權限。一般 nginx port 80 會移除外部送入的身分標頭；port 81 才保留，只能給可信 proxy 使用，不應對外公開。

公開 Compose 不發布 port 81，`ADMIN_LOGINS` 也為空，所以預設沒有人能使用 `/admin`。不要為了展示而接受可偽造的管理者標頭，或關閉身分驗證。

若要部署管理端，必須另外設定可信 proxy、網路存取限制、允許名單與工作執行服務；本機 Demo 不包含這些設定。

## 本機 Demo

web 僅綁 `127.0.0.1:3100`，PostgreSQL 與 API 只位於 Compose 內部網路。DB 密碼由 `.env` 提供，Demo DSN 請使用 URL-safe 字元。不要把 Docker socket、DB 或可信管理端直接發布到網際網路。

## 原則

- API 金鑰不進前端 `VITE_*` 變數或 Git。
- 不公開包含操作者／帳號資訊的後台截圖。
- 正式環境應另外配置 HTTPS、存取控制、備份、密碼管理與資源限制。
