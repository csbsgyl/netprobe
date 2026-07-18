# NetProbe

NetProbe 是一个可自托管的网络适配检测平台。一台公网服务器、一个公网 IP 和两个 UDP 端口即可同时提供：

- 浏览器快速检查：公网出口、HTTPS、WebRTC/STUN UDP 路径。
- Windows/Linux 一键深度检测：同一 UDP socket 访问同一 IP 的两个服务端口，检查映射稳定性、备用端口回包、RTT 和连通性。
- 服务端统一判定和 JSON 输出，便于自动筛选用户网络是否满足要求。
- Go 一键部署器：检查域名 DNS、准备 Docker、生成配置、启动服务，并等待 Caddy 自动签发 HTTPS 证书。

> 一个 IP 足以判断映射是否随目标端口变化，也能测试来自同一 IP 备用源端口的回包。它不能完整实现需要第二公网 IP 的 RFC 5780 全部测试，因此结果使用 `likely`，不会伪装成绝对准确的传统 NAT 四分类。

## 技术栈

- Go 1.22+：HTTP/UDP/STUN 服务端、Windows/Linux 检测客户端、部署器和发布工具。
- Vue 3 + TypeScript：网页界面和浏览器检测逻辑。
- Docker Compose、Caddyfile 和 GitHub Actions 仅作为部署配置。

仓库中的 `scripts/install-server.sh` 是首次安装的极薄启动入口，只识别 Linux CPU 架构、下载并校验 Go 部署器，然后立即执行它。DNS 判断、配置生成、Docker 编排和健康检查全部由 Go 完成；仓库不再使用原生 JavaScript、独立 CSS 或其他语言实现业务功能。

## 用户检测

网页打开部署域名即可进行快速检测。Linux 深度检测只需：

```bash
curl -fsSL https://check.example.com/install.sh | sh
```

Windows PowerShell：

```powershell
irm https://check.example.com/install.ps1 | iex
```

启动入口会自动识别 CPU 架构、下载客户端、验证 SHA-256、运行检测并清理临时文件。自动化系统也可直接使用 JSON：

```bash
netcheck --server https://check.example.com --json
```

退出码：`0` 通过，`1` 不通过，`2` 参数错误，`3` 运行错误，`4` 结论不确定。

## 服务器一键部署

服务器要求：

- 带公网 IPv4 或 IPv6 的 Linux，建议 Ubuntu/Debian。
- 域名已有 A 或 AAAA 记录直接指向该服务器。
- 云安全组允许 `80/tcp`、`443/tcp`、`3478/udp`、`3479/udp`；`443/udp` 用于 HTTP/3，可选。
- Root 权限和 `curl`、`sha256sum`。Docker 缺失时会调用 Docker 官方安装器。

执行：

```bash
curl -fsSL https://raw.githubusercontent.com/csbsgyl/netprobe/main/scripts/install-server.sh | sudo sh
```

安装器会询问域名并自动完成 DNS 校验、Docker/Compose 检查、源码配置、容器启动和 HTTPS 健康检查。无交互部署：

```bash
curl -fsSL https://raw.githubusercontent.com/csbsgyl/netprobe/main/scripts/install-server.sh | sudo env DOMAIN=check.example.com sh
```

固定安装某个发布版本：

```bash
curl -fsSL https://raw.githubusercontent.com/csbsgyl/netprobe/main/scripts/install-server.sh | sudo env NETPROBE_VERSION=v0.2.1 DOMAIN=check.example.com sh
```

管理命令：

```bash
cd /opt/netprobe
docker compose -f deploy/compose.yaml ps
docker compose -f deploy/compose.yaml logs -f
docker compose -f deploy/compose.yaml up -d
```

## 本地开发

需要 Go 1.22+、Node.js 22 和 pnpm 10：

```bash
pnpm install --frozen-lockfile
pnpm --dir web build
go test ./...
```

启动后端和生产前端：

```bash
NETPROBE_SECRET=development-secret \
NETPROBE_PUBLIC_HOST=127.0.0.1 \
NETPROBE_HTTP_ADDR=:8080 \
NETPROBE_UDP_PORTS=3478,3479 \
NETPROBE_WEB_DIR=web/dist \
go run ./cmd/netprobe-server
```

另一个终端运行：

```bash
go run ./cmd/netcheck --server http://127.0.0.1:8080
```

开发 Vue 界面时可运行 `pnpm --dir web dev`，Vite 会把检测 API、下载和一键命令入口代理到 `127.0.0.1:8080`。

## 安全边界

- 深度 UDP 请求使用短期会话和 HMAC 令牌，服务端以一次性回执证明核验客户端报告。
- 每个会话限制 UDP 请求数，响应不会大于请求，降低反射放大风险。
- 最终结论由服务端生成，不信任客户端自报 `PASS`。
- 浏览器 STUN 只说明当前浏览器 UDP 路径可用，不等于 P2P 必然成功。
- 结果仅代表当前设备、网络、VPN 状态和测试时间，不应作为永久身份属性。

## License

MIT
