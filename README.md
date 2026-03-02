# Auto Register

OpenAI Codex 账号自动注册工具，支持 Docker / Zeabur 部署。

## 功能

- 自动注册 OpenAI Codex 账号并获取 OAuth Token
- 支持两种邮箱后端：Cloudflare Worker / Cloud-Mail
- 支持多域名随机选择
- 注册结果自动上传到 CLIProxyAPI（Management API）
- 支持并发注册、循环执行
- 连续失败自动暂停（可配置阈值）
- 自动/手动两种启动模式

## 部署

### Docker

```bash
docker build -t auto-register .
docker run -d --env-file .env auto-register
```

### Zeabur

1. 推送代码到 GitHub
2. Zeabur 控制台 → Deploy Service → Git → 选择仓库
3. 自动检测 Dockerfile 构建
4. 配置环境变量（见下表）

## 环境变量

### 邮箱服务（二选一）

| 变量 | 说明 |
|------|------|
| `EMAIL_MODE` | `worker`（默认）= Cloudflare Worker，`cloudmail` = Cloud-Mail |
| `OWN_DOMAIN` | 邮箱域名，支持逗号分隔多域名随机，如 `a.com,b.com,c.com` |

**Worker 模式：**

| 变量 | 说明 |
|------|------|
| `WORKER_URL` | Cloudflare Worker 地址（用于接收验证码） |

**Cloud-Mail 模式：**

| 变量 | 说明 |
|------|------|
| `CLOUDMAIL_URL` | Cloud-Mail 服务地址，如 `https://mail.example.com` |
| `CLOUDMAIL_TOKEN` | Cloud-Mail API Token（通过 `POST /api/public/genToken` 生成） |

### 注册控制

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `COUNT` | `3` | 每轮注册数量 |
| `MAX_WORKERS` | `1` | 并发线程数（`1` = 串行） |
| `OTP_TIMEOUT` | `30` | 验证码等待超时（秒），建议 `300` |
| `MAX_FAIL` | `3` | 连续失败此次数后暂停任务 |

### 启动模式

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `MANUAL_MODE` | `0` | `0` = 容器启动后自动运行，`1` = 待命，需手动执行 |
| `LOOP_INTERVAL` | `0` | 循环间隔（秒），`>0` 时自动循环执行 |自动执行间隔时间

### 上传到 CLIProxyAPI

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `UPLOAD` | `0` | `1` = 注册成功后自动上传 |
| `UPLOAD_URL` | - | CPA 地址，如 `https://your-cpa.example.com`（自动拼接 `/v0/management/auth-files`） |
| `UPLOAD_TOKEN` | - | CPA Management Key（`MANAGEMENT_PASSWORD` 的值） |

### 其他

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `LOG_TO_FILE` | `0` | `1` = 日志同时写入 `logs/` 目录 |

## 使用

### 自动模式

设置 `MANUAL_MODE=0`，容器启动后自动注册。配合 `LOOP_INTERVAL=3600` 可每小时自动执行一轮。

### 手动模式

设置 `MANUAL_MODE=1`，进入容器 CMD 执行：

```bash
python auto.py           # 使用环境变量配置
COUNT=5 python auto.py   # 临时指定数量
```

### 测试网络

容器内置 curl，可测试出口 IP 和代理：

```bash
curl ifconfig.me
```

## 文件结构

```
├── auto.py           # 主程序
├── config.yaml       # 本地配置（环境变量优先覆盖）
├── domains.py        # 查询可用邮箱域名
├── worker.js         # Cloudflare Worker 验证码服务
├── Dockerfile        # Docker 构建
├── entrypoint.sh     # 启动脚本（控制自动/手动模式）
├── requirements.txt  # Python 依赖
└── files/            # 注册结果 JSON 文件（建议挂载持久存储）
```

## 注意事项

- **代理格式**：`socks5://用户名:密码@主机:端口`
- **持久存储**：Zeabur 中为 `/app/files` 挂载 Persistent Storage，防止容器重启丢失数据
- **内网互访**：同项目下服务可用 `http://服务名.zeabur.internal:端口` 互访
