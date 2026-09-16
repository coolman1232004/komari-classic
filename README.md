# Komari Classic

将以下三个固定版本合并为一个独立仓库，保留原有界面、功能、协议和依赖版本。

| 组件 | 固定源码版本 | 目录 | 原项目 |
| --- | --- | --- | --- |
| 服务端 | 1.2.5-fix2 | 仓库根目录 | [komari](https://github.com/komari-monitor/komari) |
| 网页端 | 1.2.5-fix2 | `frontend/` | [komari-web](https://github.com/komari-monitor/komari-web) |
| 探针 | 1.2.60 | `agent/` | [komari-agent](https://github.com/komari-monitor/komari-agent) |

源码来自上述版本的存档。构建只使用本仓库中的代码和依赖锁定文件，不自动同步原项目的新版本，后续也不再更新。

## 部署服务端

### Docker Compose

在服务器上创建部署目录，并执行：

```bash
curl -fsSL https://raw.githubusercontent.com/kadidalax/komari-classic/main/compose.yaml -o compose.yaml
docker compose up -d
```

访问 `http://服务器地址:25774`。此固定版本的初始登录信息由服务端启动时生成，请在自己的服务器上查看启动日志后登录；登录后可在后台修改。数据保存在部署目录下的 `data/`。

服务端镜像：`ghcr.io/kadidalax/komari-classic:latest`。

更新到本仓库最新正式发行版：

```bash
docker compose pull
docker compose up -d
```

需要固定某次发行版时，将 `compose.yaml` 中的 `:latest` 改为对应标签，例如 `:v1.2.5-fix2`。

### Linux 安装脚本

```bash
curl -fsSL https://raw.githubusercontent.com/kadidalax/komari-classic/main/install-komari.sh -o install-komari.sh
sudo bash install-komari.sh
```

菜单 `1` 安装，菜单 `2` 更新。重新执行本仓库的脚本即可管理和更新。沿用原版的 `komari` 服务名及 `/opt/komari` 安装目录。

也可以从 [Releases](https://github.com/kadidalax/komari-classic/releases) 下载对应系统和架构的 `komari-*` 文件；服务端启动参数仍为 `server`。

## 安装和更新探针

在本项目后台添加服务器，复制后台生成的安装命令。Linux、Windows、macOS 安装命令均从本仓库的 `agent/` 目录下载脚本。

- 探针二进制附件：`komari-agent-系统-架构`，Windows 文件以 `.exe` 结尾。
- 探针 Docker 镜像：`ghcr.io/kadidalax/komari-classic-agent:latest`。
- 探针自动更新只查询本仓库，并只选择 `komari-agent-*` 附件。
- 探针 Docker 部署通过拉取镜像并重建容器更新，沿用原有参数。

原项目已经安装的探针仍内置原项目更新地址。切换到 Classic 时，先更新服务端，再使用 Classic 后台生成的命令重新安装一次探针，之后才会跟随本仓库更新。探针的原有配置、服务名和命令参数保持兼容。若服务端从更高版本回退，请先备份数据，并使用独立数据目录或与本版本匹配的备份；本项目没有添加数据库降级转换。

## 来源与许可

感谢 [komari-monitor](https://github.com/komari-monitor) 和原项目贡献者。本仓库保留原版 [LICENSE](LICENSE)、[NOTICE](NOTICE)、[探针许可证](agent/LICENSE) 及前端作者署名。

原版功能文档、第三方主题市场、GeoIP 数据等资源引用继续沿用原来的来源；服务端、网页和探针的程序部署及更新来源统一为本仓库。
