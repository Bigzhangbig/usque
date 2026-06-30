# usque ZeroTrust 多平台使用手册

`usque` 通过 Cloudflare MASQUE (Connect-IP / RFC 9484) 协议与 Cloudflare WARP 端点通信。本文聚焦 ZeroTrust 团队模式（`warp=plus`，WARP+ 加速、WARP-to-WARP 互联）的多平台部署，验证记录见每节末尾。

---

## 通用前置条件

- 一台 Cloudflare 账户（[注册](https://dash.cloudflare.com/sign-up)）
- ZeroTrust 团队已启用（[Enable Zero Trust](https://one.dash.cloudflare.com/) → 选择 team name）
- 团队的 "WARP Login App" 已创建（一般默认存在）
- 一台允许登录团队的设备

获取 ZeroTrust 团队域名：`https://<your-team-name>.cloudflareaccess.com`。

> 本次验证使用的团队：`warpltyz.cloudflareaccess.com`。

---

## 一次性：提取 ZeroTrust JWT

`usque register --jwt <token>` 中的 token 必须从 ZeroTrust `/warp` 登录页提取（短时效，约 10 分钟）。

**方式 1（kimi-webbridge / 已登录的 Chrome）**：

1. 在已登录 Cloudflare 团队账户的 Chrome 中打开 `https://<team-name>.cloudflareaccess.com/warp`
2. 选择 IdP（Google / GitHub / 邮箱 OTP）完成登录
3. 登录成功后停留在 "Success!" 页面
4. 通过 DevTools Console 执行：
   ```js
   document.querySelector('meta[http-equiv="refresh"]').content.split('=')[2]
   ```
   返回的字符串即为 JWT。

**方式 2（README 推荐）**：

1. 同上访问 `/warp` 并完成 SSO 登录
2. 查看成功页的 HTML 源码，搜索 `meta http-equiv="refresh"`，从 `content` 中提取 `token=` 后的 JWT

> ⚠️ JWT 时效约 10 分钟，过期后需重新走认证流程。

---

## 通用流程

```sh
# 1. 注册（personal WARP 不需要 --jwt，ZT 必须）
usque register -a --jwt "$ZT_JWT" -n "device-name"

# 2. 启动代理（推荐 socks 模式，IPv6 可绕过部分网络限制）
usque socks -b 127.0.0.1 -p 1080 -6 -P 443 \
  -s zt-masque.cloudflareclient.com

# 3. 验证
curl -x socks5h://127.0.0.1:1080 https://www.cloudflare.com/cdn-cgi/trace
# 应包含 warp=plus
```

> `-s zt-masque.cloudflareclient.com` 是 ZeroTrust 推荐 SNI；默认 `consumer-masque.cloudflareclient.com` 在某些 ZT 团队可能返回 403。
> `-6` 走 IPv6 MASQUE 端点（`2606:4700:102::2`）。在仅阻断 UDP/443 (QUIC) v4 的网络下必选。

---

## macOS（已验证）

测试环境：macOS Darwin 25.4.0，arm64，Go 1.26.3，QuantumultX FakeIP 网络。

```sh
# 构建
cd /path/to/usque
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o usque .

# 提取 JWT（同上）+ 注册
ZT_JWT="eyJhbGciOi..."
./usque register -a --jwt "$ZT_JWT" -n "mac-$(scutil --get LocalHostName)"

# 启动 socks 代理
./usque socks -b 127.0.0.1 -p 1080 -6 -P 443 -s zt-masque.cloudflareclient.com &

# 验证
curl -x socks5h://127.0.0.1:1080 https://www.cloudflare.com/cdn-cgi/trace
```

实际验证结果：
```
warp=plus
colo=HKG
ip=2a09:bac5:1f49:2646::3d0:3d
gateway=off
```

**注意事项**：
- DNS 在某些 FakeIP 环境下可能让 UDP MASQUE 黑洞，必须用 `-6`（IPv6 端点 2606:4700:102::2 经 FakeIP 通常可达）。
- 如需绕过系统 FakeIP 拦截 API 调用，可设环境变量 `USQUE_BIND_IFACE=en0`（绑定物理网卡 SO_BINDTOIF），但需注意会脱离代理路由，**仅推荐用于个人 WARP 注册路径**，ZT 隧道拨号仍走原路径。
- 完整模式必须用 `usque socks`（gvisor netstack）。`l4-socks` 在 ZT 端点 `2606:4700:102::2` 下会出现 "CONNECT rejected with status 403"（截至本次验证），personal WARP 正常。

---

## Linux（amd64 / arm64）

构建：

```sh
git clone https://github.com/Diniboy1123/usque.git
cd usque
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o usque .
# 或交叉编译：
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o usque-linux-arm64 .
```

原生 TUN 模式（root + `tun` 模块 + iproute2）：

```sh
sudo ./usque nativetun -6 -P 443 -s zt-masque.cloudflareclient.com \
  --on-connect /etc/usque/up.sh \
  --on-disconnect /etc/usque/down.sh
```

`/etc/usque/up.sh`（手工接管路由）：
```sh
#!/bin/sh
set -e
ip route replace 2606:4700:102::2/128 via $(ip route show default | awk '/via/ {print $3; exit}') dev $(ip route show default | awk '/dev/ {print $5; exit}')
ip -6 route replace default dev "$USQUE_IFACE"
```

SOCKS 代理模式（无需 root）：
```sh
./usque socks -b 127.0.0.1 -p 1080 -6 -P 443 -s zt-masque.cloudflareclient.com
```

systemd 示例（`/etc/systemd/system/usque-zt.service`）：
```ini
[Unit]
Description=usque ZT SOCKS proxy
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/usque socks -b 127.0.0.1 -p 1080 -6 -P 443 -s zt-masque.cloudflareclient.com
Restart=on-failure
RestartSec=5
User=nobody
AmbientCapabilities=

[Install]
WantedBy=multi-user.target
```

`/etc/resolv.conf`（如果需要让系统走 WARP 解析）：
```sh
sudo systemctl enable --now usque-zt
# 之后将客户端代理指向 socks5://127.0.0.1:1080
```

---

## Windows（amd64 / arm64）

构建（从 Linux/macOS 交叉编译）：
```sh
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o usque.exe .
```

原生 TUN 模式：
- 需要 [wintun.dll](https://www.wintun.net/) 放到 `usque.exe` 同目录
- 需要管理员权限（PowerShell Run as Administrator）

```powershell
usque.exe nativetun -6 -P 443 -s zt-masque.cloudflareclient.com `
  --on-connect C:\usque\up.bat `
  --on-disconnect C:\usque\down.bat
```

`C:\usque\up.bat`：
```bat
@echo off
netsh interface ipv6 delete route ::/0 "%USQUE_IFACE%" >nul 2>&1
netsh interface ipv6 add route ::/0 "%USQUE_IFACE%" ::
```

SOCKS 代理（无需管理员）：
```powershell
usque.exe socks -b 127.0.0.1 -p 1080 -6 -P 443 -s zt-masque.cloudflareclient.com
```

NSSM 注册为服务：
```powershell
nssm install usque-zt "C:\usque\usque.exe" "socks -b 127.0.0.1 -p 1080 -6 -P 443 -s zt-masque.cloudflareclient.com"
nssm set usque-zt AppDirectory C:\usque
nssm start usque-zt
```

---

## Android（arm64）

构建：
```sh
GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o usque-android-arm64 .
```

运行限制：
- Android 14+ 需要 root 才能操作 `tun` 设备
- 无 root 推荐走 SOCKS + Android 本地代理 App（如 SagerNet / NekoBox / Clash.Meta for Android）
- Termux + root 下运行：

```sh
# Termux
pkg install proot-distro
proot-distro install ubuntu
proot-distro login ubuntu
# 在 ubuntu proot 内
apt update && apt install -y curl
# 把 usque 二进制拷入 Termux home
chmod +x usque-android-arm64
./usque-android-arm64 register -a --jwt "$ZT_JWT" -n "android-termux"
./usque-android-arm64 socks -b 127.0.0.1 -p 1080 -6 -P 443 -s zt-masque.cloudflareclient.com
```

通过 NekoBox for Android 导入 SOCKS5 代理 `127.0.0.1:1080`，即可在所有 App 内强制走 WARP。

> 验证状态：本次未做端到端验证（无 root 设备）。二进制可构建，运行路径与 Linux 一致。

---

## Docker

项目自带 `Dockerfile`（基于 `golang:1.26.3-alpine` 编译 + `alpine` 运行时）：

```sh
docker build -t usque:latest .
```

运行（注入 JWT，挂载配置目录）：

```sh
# 1. 提取 JWT（参见上文）
# 2. 启动容器，注册一次后退出
docker run -it --rm \
  -v $PWD/usque-data:/data \
  usque:latest \
  register -a --jwt "$ZT_JWT" -c /data/config.json -n "docker-zt"

# 3. 长跑 socks 代理
docker run -d --restart=unless-stopped \
  --name usque-zt \
  -p 1080:1080 \
  -v $PWD/usque-data:/data \
  usque:latest \
  socks -b 0.0.0.0 -p 1080 -6 -P 443 -s zt-masque.cloudflareclient.com \
       -c /data/config.json
```

`docker-compose.yml` 示例：
```yaml
services:
  usque-zt:
    build: .
    container_name: usque-zt
    restart: unless-stopped
    ports:
      - "1080:1080"
    volumes:
      - ./usque-data:/data
    command: >
      socks -b 0.0.0.0 -p 1080 -6 -P 443
           -s zt-masque.cloudflareclient.com
           -c /data/config.json
```

测试：
```sh
docker exec usque-zt curl -x socks5h://127.0.0.1:1080 https://www.cloudflare.com/cdn-cgi/trace
```

> Docker 镜像需要 privileged 或 `--cap-add=NET_ADMIN` 才能跑 `nativetun` 模式；SOCKS / HTTP-proxy / L4 模式无需特殊权限。

---

## Cloudflare Worker —— 不可行说明

Cloudflare Workers 运行时**不支持 raw UDP 套接字**，而 MASQUE 协议建立在 QUIC（UDP）之上。Worker's TCP-only fetch API 不能承载 Connect-IP。

**替代方案**：
- **Workers AI / KV / Durable Objects 仅能用作辅助**：例如部署一个 Worker 把 ZT JWT 短期缓存，供多设备复用（但 JWT 时效短且绑定 IP/UA，缓存价值有限）。
- **Cloudflare Tunnel (`cloudflared`)** 是 Cloudflare 自家的反向隧道，不等价于出站 VPN，但可用作"让局域网服务对外暴露"——常见于 ZeroTrust 私有应用的部署场景。
- 真正跑 WARP 出站，仍然需要 Go / Rust / Swift 原生客户端（即 usque）。

结论：**Cloudflare Worker 不能运行 usque**。若需"零运维运行 usque"，推荐 Docker / 家用 Linux 盒子 / Cloudflare Spectrum 转发（也不可行 — Spectrum 仍是入站）。

---

## 故障排查

| 现象 | 原因 | 解决 |
|---|---|---|
| `register` 失败 `failed to send request: EOF` | FakeIP/代理拦截 DNS | 改物理网卡直连，或获取真实 IP 加到 hosts |
| 注册后隧道 `timeout: no recent network activity` | IPv4 MASQUE 端点被本地网络黑洞 | 加 `-6` 走 IPv6 端点 `2606:4700:102::2` |
| `l4-socks` 模式返回 `403 CONNECT rejected` | ZT 端点与 l4-socks QUIC transport 兼容问题（personal WARP 正常） | 改用完整 `usque socks` 模式 |
| 关闭代理后仍走 IPv4 失败 | 系统代理 / TUN 拦截 UDP/443 (QUIC) | 关闭代理（macOS: QuantumultX）或改 `-6` 走 IPv6 端点 |
| `tls: access denied` | 设备密钥未在该端点注册（用了 personal 端点拉 ZT 设备） | 使用 config 中实际的 endpoint_v6 (`2606:4700:102::2`) |
| `warp=on` 而非 `warp=plus` | 注册为 personal WARP 而非 ZeroTrust | 用 `--jwt` 重新注册 |

---

## 验证时间戳

- 个人 WARP IPv6 MASQUE 隧道：2026-06-30 23:39 CST，colo=HKG
- ZeroTrust `warpltyz` 团队 IPv6 隧道：2026-07-01 00:02 CST，colo=HKG，`warp=plus`
- ZeroTrust `warpltyz` 团队 IPv4 隧道（QX 关闭后）：2026-07-01 00:37 CST，colo=SJC，`warp=plus`，端点 `162.159.197.2:443`
- 验证脚本：`curl -x socks5h://127.0.0.1:1080 https://www.cloudflare.com/cdn-cgi/trace`

---

## 参考

- usque README: <https://github.com/Diniboy1123/usque>
- Cloudflare ZeroTrust WARP Login: <https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/>
- MASQUE / RFC 9484: <https://datatracker.ietf.org/doc/rfc9484/>
- connect-ip-go fork: <https://github.com/Diniboy1123/connect-ip-go>