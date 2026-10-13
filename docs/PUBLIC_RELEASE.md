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

## 發布方式

開發版本打上 release tag（例如 `v1.2.3`）後，自動化流程會：

1. 依白名單匯出該版本，套用公開版文件，不使用 `git push --mirror`。
2. 檢查禁止內容、文件中的連結與 `make` 指令，再用 gitleaks 掃描秘密。
3. 在匯出結果上執行 Go 與前端的 lint、測試和建置。
4. 全部通過後才 commit 到這份 repo 的 `main`（`chore: release v1.2.3`），並打上同名 tag。

任何一步失敗都不會推送。每個 release tag 對應這份 repo 的一個 commit；`.env`、備份、真實 fixture 與發布權限不在匯出範圍內。

首次公開前，建議用專業 secret scanner 檢查工作樹與要發布的 Git 歷史。白名單匯出和簡單樣式檢查不能取代完整安全稽核。

外部修正先帶回主要開發版本，再重新匯出。匯出工具與來源 commit 對照可留在非公開維運流程，不讓公開內容依賴內部路徑或 repo。
