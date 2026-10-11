# Interactive Architecture

[HTML 圖檔](index.html) · [JSON 原始檔](public.architecture.json) · [架構說明](../../ARCHITECTURE.md)

這張圖用 [Archify](https://github.com/tt-a1i/archify) 產生。下載 repo 後，用瀏覽器開啟 `index.html` 就能縮放和查看原始碼連結。圖中文字是繁體中文，操作介面是英文。

GitHub 檔案頁只能看 HTML 原始碼；目前沒有架設互動圖網站。

<details>
<summary>更新圖片</summary>

修改 `public.architecture.json`，將來源 revision 設為 GitHub 上已有的 commit。以下指令使用 Archify 2.17：

```bash
ARCHIFY_CLI=/path/to/archify/bin/archify.mjs node scripts/architecture.mjs check
ARCHIFY_CLI=/path/to/archify/bin/archify.mjs node scripts/architecture.mjs build
ARCHIFY_CLI=/path/to/archify/bin/archify.mjs node scripts/architecture.mjs visual
```

重新產生 HTML 後，在瀏覽器切換 Light／Dark，用 **Export → PNG** 分別更新 `architecture-light.png` 和 `architecture-dark.png`。`build` 不會替你更新 PNG。

確認文字和箭頭沒有重疊，再更新 [驗證紀錄](validation.json)。以上指令只產圖與檢查，不會 commit、push 或部署。

</details>
