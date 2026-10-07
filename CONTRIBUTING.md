# Contributing

先依 README 啟動 mock，再執行 `make lint test build`。測試應使用合成 fixture 與本機 mock server，不需要正式資料庫或 API 金鑰。

修改 parser 時可更新 `scripts/generate-fixtures.mjs`，執行 `make fixtures`，並補充預期案例；禁止把下載的真實資料重新放入 testdata。文件須區分已實作與規劃中的能力。

公開版是受控快照。外部 PR 修正應先在主要開發流程整合，再由下一次公開發布納入，避免兩份程式碼各自演進。維護者可先啟用 GitHub PR／Issue 與私密漏洞通報，再邀請外部貢獻。
