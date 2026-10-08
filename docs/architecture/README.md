# Interactive Architecture

[回文件索引](../README.md) · [架構與 Mermaid](../../ARCHITECTURE.md) · [JSON 原始檔](public.architecture.json) · [互動 HTML](index.html)

互動圖由 [Archify](https://github.com/tt-a1i/archify) 2.17 產生，包含系統總覽、資料匯入、查詢分析、品質告警四個視角。節點可追溯至 public repo 的指定 commit；不引用 private repo、內網服務或 architecture-hub。

圖中文字為繁體中文；Archify 固定 Viewer 操作介面與 HTML 語言標記使用英文 fallback。這是功能責任圖，不是正式部署拓撲。Mock 快速體驗不走圖中的資料庫查詢路徑；外部來源與 Webhook 均需額外設定，公開 Compose 不預設啟動同步。

## 開啟

下載 repo 後直接以瀏覽器開啟 `docs/architecture/index.html`，不需 API 或資料庫。GitHub 檔案頁不會執行互動 HTML；README 的 Mermaid 則可直接閱讀。

若需要本機 HTTP 預覽，從 repo 根目錄執行：

```bash
npx --no-install vite --host 127.0.0.1 --port 3101 --strictPort
```

開啟 <http://localhost:3101/docs/architecture/index.html>，使用完畢按 Ctrl-C 關閉。此預覽需先依根目錄 README 安裝 npm dependencies。

## 重新產生與驗證

只編輯 `public.architecture.json`，不要手改產生的 HTML。另行取得 Archify 2.17，指定其 CLI 路徑：

```bash
ARCHIFY_CLI=/path/to/archify/bin/archify.mjs node scripts/architecture.mjs check
ARCHIFY_CLI=/path/to/archify/bin/archify.mjs node scripts/architecture.mjs build
ARCHIFY_CLI=/path/to/archify/bin/archify.mjs node scripts/architecture.mjs visual
```

`check` 驗證 showcase 與 pinned commit 的來源引用；`build` 通過相同檢查後以 atomic delivery 更新 HTML 與 hash receipt；`visual` 需要 Chrome／Chromium，檢查四種桌面尺寸並產生亮暗截圖與 browser receipt。自動檢查不等於人工視覺審查。

`meta.repository.revision` 是已公開的程式版本。更新引用時，先確保程式已提交且存在 public repo，再更新 revision 與來源路徑、重新產生。產圖不會自動 commit 或 push；本地 Archify 安裝路徑不會寫入 HTML。

交付證據見 [validation.json](validation.json)。瀏覽器截圖及詳細暫存輸出不納入 repo，避免擴大版本庫。

## 未部署的 Pages 方案

目前沒有公開互動網站，也沒有新增 Pages workflow。之後可用只讀 checkout 與 Pages 專用權限，僅上傳此目錄的 `index.html`，預期路徑為 `https://karevin07.github.io/geo-prophet-public/`；啟用 Pages 並完成部署前，不把它當作有效展示連結。

不沿用 private 的跨 repo dispatch、bot 自動 commit 或部署憑證。Mermaid 與 JSON 不是同一套生成來源，修改架構時應同步檢查兩者；此階段不宣稱已自動檢查語意一致性。
