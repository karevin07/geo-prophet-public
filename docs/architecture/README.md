# Interactive Architecture

[互動 HTML](index.html) · [JSON 原始檔](public.architecture.json) · [深入架構](../../ARCHITECTURE.md)

README 顯示 [Archify](https://github.com/tt-a1i/archify) 亮／暗圖片；互動版請下載後以瀏覽器開啟 `index.html`。GitHub 不執行 HTML，目前未部署互動網站。

圖文為繁體中文，固定操作介面為英文。

<details>
<summary>維護與重新匯出</summary>

編輯 `public.architecture.json`，來源 revision 指向已公開 commit。使用 Archify 2.17：

```bash
ARCHIFY_CLI=/path/to/archify/bin/archify.mjs node scripts/architecture.mjs check
ARCHIFY_CLI=/path/to/archify/bin/archify.mjs node scripts/architecture.mjs build
ARCHIFY_CLI=/path/to/archify/bin/archify.mjs node scripts/architecture.mjs visual
```

切換 Light／Dark，以 **Export → PNG** 更新 `architecture-light.png`／`architecture-dark.png`；`build` 不會自動更新圖片。

重建後更新 [驗證紀錄](validation.json)，並另行視覺審查。以上指令不會 commit、push 或部署。

</details>
