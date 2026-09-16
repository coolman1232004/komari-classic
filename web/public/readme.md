# 内嵌网页资源

Komari Classic 的固定网页源码位于本仓库的 [`frontend/`](../../frontend/) 目录。

在仓库根目录执行以下命令，将网页编译到服务端的内嵌目录：

```bash
npm --prefix frontend ci
npm --prefix frontend run build
mkdir -p web/public/defaultTheme/dist
cp -r frontend/dist/. web/public/defaultTheme/dist/
cp frontend/komari-theme.json web/public/defaultTheme/
cp frontend/preview.png web/public/defaultTheme/preview.png
cp frontend/preview.png web/public/defaultTheme/perview.png
```

完成后再编译服务端。GitHub Actions 自动执行相同步骤。

部署、更新和完整构建说明见 [仓库首页](../../README.md)。
