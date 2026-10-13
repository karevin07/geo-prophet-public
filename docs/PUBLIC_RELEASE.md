# Public release policy

這份 repo 是從開發版本整理出的公開快照，不是完整 mirror，也不包含開發 repo 的 Git 歷史。修改先在開發版本完成，再帶到這裡；不要讓兩份 repo 各自開發不同的版本。

## Included

前後端、來源 adapter、匯入流程、migration、通用 nginx／Postgres 工具、唯讀 CI、公開文件及合成測試資料。

## Excluded or adapted

- 原始 Git 歷史、遠端設定、環境檔、下載資料、備份、cache 與 build output 不匯出。
- 架構發布 workflow、跨 repo GitHub App 設定、部署主機資訊、私有架構圖來源連結不匯出。
- 政府 CSV 與 TDX JSON fixture 不匯出；CSV parser／importer 改用合成資料，TDX loader 改用合成 getter。
- README、架構、操作指令與 Compose 已按公開用途調整，測試覆蓋與部署能力不一定和開發版相同。
- 前端 mock 模式略過管理身分探測，不呼叫本機或正式管理 API。

## Publication checklist

1. 依白名單匯出單一已驗證版本，不使用 `git push --mirror`。
2. 審查文件、測試、圖片及 generated assets；不只依賴 `.gitignore`。
3. 執行秘密掃描與人工檢查，review 新增／修改的檔案。
4. 在乾淨環境執行 `npm ci`、`make lint test build` 和 mock 啟動驗證。
5. 確認 `.env`、備份、真實 fixture、私有發布權限未出現在 commit。
6. 人工確認後再建立公開 commit、設定 GitHub remote 與推送。

首次公開前，建議用專業 secret scanner 檢查工作樹與要發布的 Git 歷史。白名單匯出和簡單樣式檢查不能取代完整安全稽核。

外部修正先帶回主要開發版本，再重新匯出。匯出工具與來源 commit 對照可留在非公開維運流程，不讓公開內容依賴內部路徑或 repo。
