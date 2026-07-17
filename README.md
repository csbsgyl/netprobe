# NetProbe

NetProbe 是一个可自托管的网络适配检测平台。一台公网服务器即可同时提供：

- 浏览器快速检查：公网出口、HTTPS、WebRTC/STUN UDP 路径。
- Windows/Linux 一键深度检测：同一 UDP socket 访问两个服务端口，检查映射稳定性、备用端口回包和连通性。
- 统一服务端判定与 JSON 协议。
- 交互式一键部署：校验域名 DNS、安装 Docker、启动服务，并由 Caddy 自动申请和续期 HTTPS 证书。

> 单公网 IP 可以判断“映射是否随目标端口变化”，不能完整区分所有 RFC 4787/5780 映射与过滤行为。界面会把结果标为 `likely`，不会伪装成权威的传统 NAT 四分类。

## 用户使用

打开网页进行快速检测，或执行一条深度检测命令：

```bash
curl -fsSL https://check.example.com/install.sh | sh
```

Windows PowerShell：

```powershell
irm https://check.example.com/install.ps1 | iex
```

脚本自动识别 CPU 架构、下载客户端、校验 SHA-256、运行检测并清理临时文件。客户端也支持自动化 JSON 输出：

```bash
netcheck --server https://check.example.com --json
```

退出码：`0` 通过，`1` 不通过，`2` 参数错误，`3` 运行错误，`4` 结论不确定。

## 服务器一键部署

要求：

- 受支持的 Linux 公网服务器，建议 Ubuntu/Debian。
- 域名已有 A 或 AAAA 记录指向该服务器。
- 云安全组允许 `80/tcp`、`443/tcp`、`3478/udp`、`3479/udp`。
- Root 权限。

执行：

```bash
curl -fsSL https://raw.githubusercontent.com/csbsgyl/netprobe/main/scripts/install-server.sh | sudo sh
```

安装器会询问域名，并自动完成：

1. 获取服务器公网 IPv4/IPv6。
2. 解析域名并确认至少一个地址指向本机。
3. 安装 Docker Engine 与 Compose（缺失时）。
4. 配置服务密钥并启动容器。
5. Caddy 申请并自动续期 TLS 证书。
6. 等待 HTTPS 健康检查通过后输出用户命令。

无交互部署可预先传入域名：

```bash
curl -fsSL https://raw.githubusercontent.com/csbsgyl/netprobe/main/scripts/install-server.sh | sudo env DOMAIN=check.example.com sh
```

管理命令：

```bash
cd /opt/netprobe
docker compose -f deploy/compose.yaml ps
docker compose -f deploy/compose.yaml logs -f
docker compose -f deploy/compose.yaml up -d --build
```

## 本地开发

需要 Go 1.22+：

```bash
go test ./...
NETPROBE_SECRET=development-secret \
NETPROBE_PUBLIC_HOST=127.0.0.1 \
NETPROBE_HTTP_ADDR=:8080 \
NETPROBE_UDP_PORTS=3478,3479 \
go run ./cmd/netprobe-server
```

另一个终端运行：

```bash
go run ./cmd/netcheck --server http://127.0.0.1:8080
```

## 安全边界

- 深度 UDP 请求使用短期会话和 HMAC 令牌，服务端以一次性回执证明核验客户端报告。
- 每个会话限制 UDP 请求数，响应不会大于请求，降低反射放大风险。
- 服务端判定结果，不信任客户端自报 `PASS`。
- 浏览器 STUN 只说明当前浏览器 UDP 路径可用，不等于 P2P 必然成功。
- 检测结果随设备、VPN、网络和时间变化，不应作为永久身份属性。

## License

MIT
