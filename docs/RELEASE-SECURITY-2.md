# Komari Classic Security 2

固定功能版本：服務端／前端 1.2.5-fix2、探針 1.2.60。安全發佈標籤：`v1.2.5-fix2-security.2`。

## 本次修補

Security 1 映像有兩筆 LOW `CVE-2026-81870`，來自 cloudflared 的 OpenTelemetry SDK 和 trace exporter。本次將相關模組升級至官方修補版 **1.45.0**，同步鎖定必要的間接依賴，消除診斷日誌可能披露 exporter 設定的問題。監控功能及資料庫結構沒有改動。

- [官方漏洞公告](https://github.com/open-telemetry/opentelemetry-go/security/advisories/GHSA-8wmf-6v46-5gfg)
- [修補提交](https://github.com/coolman1232004/komari-classic/commit/3b45d99e173874cb611bf1c8e53bcb65268f7d89)
- [發佈頁](https://github.com/coolman1232004/komari-classic/releases/tag/v1.2.5-fix2-security.2)

兩種 Docker 建置方式都會先執行 cloudflared 原有 tracing 測試，再編譯。cloudflared 本身仍固定官方 2026.9.1 的源碼提交與校驗值。

## 驗證紀錄

[AMD64／ARM64 源碼 Docker 回歸測試](https://github.com/coolman1232004/komari-classic/actions/runs/35360585428) 全部通過，包括服務端測試、登入、權限拒絕、探針上報、重啟資料保留及舊密碼遷移；AMD64 亦通過並行存取的 race 測試。源碼建置映像在兩種架構均沒有 CRITICAL、HIGH、MEDIUM 或 LOW 告警。服務端有兩筆 UNKNOWN，探針總告警為 0。

本機 Linux AMD64 目標的 Go 源碼掃描（Go 1.26.8，CGO 關閉）顯示 Komari 與 cloudflared 均沒有已知受影響呼叫或匯入套件。正式 Docker 服務端使用 CGO SQLite，並另有上述實際容器測試及映像掃描。

[正式發佈流程](https://github.com/coolman1232004/komari-classic/actions/runs/35361583842) 的 25 項工作全部通過；[2026-09-19 直接拉取正式映像驗證](https://github.com/coolman1232004/komari-classic/actions/runs/35410429991) 的 AMD64／ARM64 工作亦全部通過。正式映像可匿名拉取，啟動、登入、權限拒絕、探針上報、重啟資料保留與密碼遷移均通過。

| 最終發佈映像（每種架構） | 嚴重／高危 | 中危 | 低危 | 未分類 |
|---|---:|---:|---:|---:|
| 服務端 | 0 | 0 | 0 | 2 |
| 探針 | 0 | 0 | 0 | 0 |

完整掃描報告已下載保存並核對 GitHub artifact 的 SHA-256。報告中的 OpenTelemetry SDK、exporter、API、trace 及 metric 模組均確認為 1.45.0。

- 服務端多架構 digest：`sha256:34fab72ca869f7f58895e6916cc490cbccf9eb1bef40af771dff58be163a1701`。
- 探針多架構 digest：`sha256:8c9dd1c5254de9672f28d77f9c4e146985d8160315fce6d90fbb5295382a186c`。
- `compose.image.yaml` 已固定服務端 digest。
- 程式源碼及發佈工作均來自提交 `3b45d99e173874cb611bf1c8e53bcb65268f7d89`。

## 剩餘告警及限制

兩筆 UNKNOWN 是 `GO-2026-5932`，分別由 Komari 與 cloudflared 引用的 x/crypto 模組觸發。公告針對不再維護的 `x/crypto/openpgp` 套件，**沒有可升級的修補版本**；源碼掃描確認沒有匯入受影響套件。x/crypto 仍用於其他密碼學功能，所以沒有為清除掃描數字而移除整個模組，也沒有加入掃描忽略清單。

[Go 官方公告](https://pkg.go.dev/vuln/GO-2026-5932)。以上不等於零漏洞保證；未知漏洞、主機、代理及完整第三方供應鏈仍不在認證範圍。實際 Cloudflare Tunnel 連線需要使用者憑證，本次未驗證。

## Docker 安裝與升級

```sh
docker pull ghcr.io/coolman1232004/komari-classic:v1.2.5-fix2-security.2
docker pull ghcr.io/coolman1232004/komari-classic-agent:v1.2.5-fix2-security.2
```

完整安裝請依照 [README](../README.md)，使用 `compose.image.yaml`。該配置使用獨立 volume 和 localhost 端口 25775。首次登入後設定強密碼及 2FA，透過 HTTPS 反向代理或 SSH tunnel 存取。

從 Security 1 升級：先停止服務並備份完整資料 volume，保存舊映像 digest；更新 Compose 的映像版本，再執行 `docker compose pull` 及 `docker compose up -d`。保留原有 volume，不執行 `docker compose down -v`。本次沒有資料庫結構變更；出現問題時使用舊映像及與之配對的資料備份回退。

**上游 1.2.7 使用者仍不可直接覆蓋降級。** 本次沒有新增 1.2.7 資料轉換功能。先保留現有服務及離線備份，使用獨立資料目錄和端口試行；你的實際備份尚未做還原驗證。詳見 [1.2.7 相容性分析](RELEASE-SECURITY-1.md)。

維護原則是固定功能、按需要更新安全依賴，並在通過測試後更換固定 digest 的映像；不是永久停止安全更新。
