# Komari Classic：發佈、安全及 1.2.7 備份

更新：2026-09-18。

此頁保留 Security 1 的歷史結果。Security 2 已修補此頁列出的兩筆低危告警；最新安裝及驗證見 [Security 2](RELEASE-SECURITY-2.md)。下方 Security 1 的映像及掃描數字不代表目前最新版本。

## 合併及版本

PR #1 已合併至你的 GitHub 主分支 `main`。合併提交 `1e73d75fdcda74c7b24b99f7b0723850b15ef9e4` 與已通過測試的 `44c8f12a` 具有完全相同的程式碼樹。

發佈標籤：`v1.2.5-fix2-security.1`。

發佈頁：https://github.com/coolman1232004/komari-classic/releases/tag/v1.2.5-fix2-security.1

服務端和探針已發佈，AMD64／ARM64 均可匿名拉取。兩種架構的實際安裝、登入、探針上報、資料保留和密碼升級測試已通過，最終掃描結果見下方驗證紀錄。

## 安全結論

不能保證零漏洞。已完成主要入口的源碼檢查、權限回歸、Go/npm 依賴掃描、雙架構 Docker 測試及映像掃描；這不是逐行及完整供應鏈認證。

此前源碼建置的映像沒有 HIGH/CRITICAL 警示；探針總警示為 0。服務端有兩筆 UNKNOWN `GO-2026-5932`，分別來自 Komari 和 cloudflared 的 x/crypto 模組；源碼掃描顯示沒有匯入受影響的 OpenPGP 套件。

我沒有加入後門、固定秘密密碼、隱藏管理員或將密鑰傳回作者的程式。檢查範圍內未發現這類內容，但不能據此證明所有第三方程式或部署環境絕對無後門。初始管理密碼由使用者環境變數指定或隨機產生；Cloudflare Tunnel 需要使用者配置 token 才啟動；舊二進位自動更新器已停用。

原有遠端終端及任務執行仍是高權限管理功能；不需要時停用。IP/地理位置查詢和管理員選用的通知/OAuth/Tunnel 功能仍會聯絡相應外部服務；不等於完全離線運作。未進行所有第三方回應的惡意輸入及記憶體耗盡測試。

## 1.2.7 備份不能當作可直接無損降級

1.2.7 將監控歷史搬到新的 metric store，完成後刪除舊 `records`、`records_long_term`、`gpu_records`、`ping_records` 表。舊版 1.2.5-fix2 不會自動把新儲存還原成舊表。1.2.7 同時改用 UTC 的 `time.Time` 欄位。

- 官方發佈說明：https://github.com/komari-monitor/komari/releases/tag/1.2.7
- 官方遷移實作：https://github.com/komari-monitor/komari/blob/1.2.7/pkg/migrations/legacy_monitoring.go

備份 ZIP 被接受不代表內容可以完整使用。帳戶、節點設定、歷史數據及主題都需要逐項驗證；未取得你的備份，沒有做過你的資料的實際還原測試。

不要把 1.2.7 的 `data` 目錄直接掛到這個舊版容器；不要停止或覆蓋現有服務來嘗試。保留完整離線備份，在獨立資料目錄和端口測試。若需要保留 1.2.7 歷史數據，應另做並驗證遷移工具。

## 獨立測試安裝

以下使用不同容器名、資料目錄及端口，不使用現有 1.2.7 資料：

```sh
docker pull ghcr.io/coolman1232004/komari-classic:v1.2.5-fix2-security.1
mkdir -p komari-classic-security-data
docker run -d --name komari-classic-security --restart unless-stopped \
  --security-opt no-new-privileges:true \
  --log-opt max-size=10m --log-opt max-file=3 \
  -p 127.0.0.1:25775:25774 \
  -v "$(pwd)/komari-classic-security-data:/app/data" \
  ghcr.io/coolman1232004/komari-classic:v1.2.5-fix2-security.1
docker logs komari-classic-security
```

透過 HTTPS 反向代理或 SSH tunnel 存取主機的 `127.0.0.1:25775`。首次登入後設定獨立強密碼及 2FA。此處沒有部署到你的 VPS。

## 發佈與校驗紀錄

- 發佈流程：https://github.com/coolman1232004/komari-classic/actions/runs/35296847498 （25 個工作全部通過）。
- 直接拉取驗證：https://github.com/coolman1232004/komari-classic/actions/runs/35297950618 。
- 服務端多架構 digest：`sha256:c778e536445f262f967e1a699ca72fabef010f04c7fd5f6fafedff81d094e308`。
- 探針多架構 digest：`sha256:76c15a1ab4db4f9e1eb2b1f3331923b421b006ff28890f10ea573a9b6fdf6162`。
- 映像程式來自發佈標籤的提交 `1e73d75`；映像標籤中的 revision `1871157` 是執行發佈打包流程的提交，兩者應分開記錄。

## 最終發佈映像掃描

兩種架構均通過匿名拉取、啟動、登入、權限拒絕、上報、重啟資料保留與舊密碼遷移驗證。

| 映像 | 高危／嚴重 | 中危 | 低危 | 未分類 |
|---|---:|---:|---:|---:|
| 服務端（各架構） | 0 | 0 | 2 | 2 |
| 探針（各架構） | 0 | 0 | 0 | 0 |

服務端兩筆 LOW 均為 cloudflared 內的 `CVE-2026-81870`（OpenTelemetry exporter 與 SDK，修補版 1.45.0）。其影響是開啟特定內部診斷日誌後，可能把 exporter endpoint 寫到日誌；**這個發佈版本的依賴尚未升級至該修補版**。官方明確說明預設 logger 不會觸發，且需要取得日誌才可讀到披露資料。對 cloudflared 源碼的查找沒有發現啟用 `otel.SetLogger` 的呼叫，符合預設緩解條件；不是對所有未來設定的保證。

官方公告：https://github.com/open-telemetry/opentelemetry-go/security/advisories/GHSA-8wmf-6v46-5gfg

另兩筆 UNKNOWN 是前述未匯入 OpenPGP 套件的模組公告。沒有使用掃描忽略清單。此前源碼建置掃描的數字是歷史紀錄，不能取代本次發佈映像的完整結果。

GitHub 安裝配置 `compose.image.yaml` 已用服務端多架構 digest 固定映像，避免日後同名 tag 被改動。完整掃描 ZIP 已下載並核對 GitHub artifact SHA-256。
