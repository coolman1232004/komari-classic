# Komari Classic Security

基於服務端／網頁端 **1.2.5-fix2**、探針 **1.2.60** 的安全加固版本。保留既有監控功能，更新有安全修正的依賴。原版保存於標籤 `v1.2.5-fix2`；已審查的安全修正已合併到 `main`。

**1.2.7 使用者：不要直接降級覆蓋資料。** 1.2.7 已改用新的歷史指標儲存，備份不能視為可直接無損還原到此舊版。先保留現有服務及離線備份，使用獨立資料目錄／Docker volume 測試。[相容性及安全說明](docs/RELEASE-SECURITY-1.md)。

## Docker 安裝

固定版本：`v1.2.5-fix2-security.1`。AMD64／ARM64 共用相同映像地址。只需要 Docker Engine 和 Compose，主機不需要 Go 或 Node。

```sh
mkdir komari-classic-security
cd komari-classic-security
curl -fsSL https://raw.githubusercontent.com/coolman1232004/komari-classic/main/compose.image.yaml -o compose.yaml
docker compose pull
docker compose up -d
docker compose logs komari
```

這份 Compose 使用獨立的 Docker volume、容器名 `komari-classic-security` 及 `127.0.0.1:25775`，方便與現有 1.2.7 並行測試。不要掛載現有 1.2.7 資料。透過 HTTPS 反向代理或 SSH tunnel 存取；首次登入後設定獨立強密碼及 2FA。刪除容器不會自動刪除 volume；不要執行 `docker compose down -v`，除非確實要刪除資料。

服務端映像：`ghcr.io/coolman1232004/komari-classic:v1.2.5-fix2-security.1`。

探針映像：`ghcr.io/coolman1232004/komari-classic-agent:v1.2.5-fix2-security.1`。探針需要你自己的服務端地址及節點 token，宿主機監控範圍視掛載與命名空間設定而定。不需要遠端終端時加入 `--disable-web-ssh`。

[發佈與驗證結果](https://github.com/coolman1232004/komari-classic/releases/tag/v1.2.5-fix2-security.1) · [詳細 Docker／備份說明](docs/DOCKER.md) · [安全檢查範圍及限制](docs/SECURITY-AUDIT-2026-09.md)

## 安全結論

修正涵蓋探針身份及欄位隔離、Nezha 相容接口驗證、Session／RPC／終端權限重驗、Argon2id 密碼升級、登入與 TOTP 限流、OAuth state、ZIP 解壓、主題下載、秘密日誌、cloudflared 和 OpenSSL 依賴。

實際發佈映像在 AMD64／ARM64 都通過匿名拉取與安裝驗證。最新掃描：服務端 0 高危／嚴重、0 中危、2 低危、2 未分類；探針 0。兩筆低危是 cloudflared 的條件式診斷日誌公告，依賴尚未升至公告修補版，預設 logger 不觸發；完整說明見發佈紀錄。

安全掃描通過不等於零漏洞。實際 VPS、防火牆、代理、主題與憑證仍需要妥善管理。功能版本可以固定，安全依賴及 Docker 基底仍需定期檢視。二進位自動更新器已停用，更新需更換經驗證的映像。

## 源碼建置與來源

需要自行建置時，使用倉庫中的 `compose.yaml`；預製映像安裝使用 `compose.image.yaml`。

感謝 [komari-monitor](https://github.com/komari-monitor) 及 [kadidalax/komari-classic](https://github.com/kadidalax/komari-classic)。原始來源說明保存在 [歷史 README](docs/UPSTREAM-CLASSIC-README.md)，原有授權檔案保留於倉庫。
