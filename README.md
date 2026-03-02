# Auto Register

OpenAI Codex 账号自动注册工具，支持 Docker / Zeabur 部署。

## 功能

- 自动注册 OpenAI Codex 账号并获取 OAuth Token
- 支持两种邮箱后端：Cloudflare Worker / Cloud-Mail
- 注册结果自动上传到 CLIProxyAPI
- 支持并发注册、循环执行、连续失败自动暂停

## 部署

### Docker

```bash
docker build -t auto-register .
docker run -d --env-file .env auto-register
```

### Zeabur

1. 推送代码到 GitHub
2. Zeabur 控制台 → Deploy Service → Git → 选择仓库
3. 配置环境变量（见下表）

## 环境变量

### 邮箱服务（二选一）

| 变量 | 说明 |
|------|------|
| `EMAIL_MODE` | `worker`（默认）= Cloudflare Worker，`cloudmail` = Cloud-Mail |
| `OWN_DOMAIN` | 邮箱域名，如 `example.com`（两种模式都需要） |

**Worker 模式：**

| 变量 | 说明 |
|------|------|
| `WORKER_URL` | Cloudflare Worker 地址 |

**Cloud-Mail 模式：**

| 变量 | 说明 |
|------|------|
| `CLOUDMAIL_URL` | Cloud-Mail 服务地址，如 `https://mail.example.com` |
| `CLOUDMAIL_TOKEN` | Cloud-Mail API Token（通过 genToken 接口生成） |

### 可选

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `COUNT` | `3` | 每轮注册数量 |
| `MAX_WORKERS` | `1` | 并发线程数（1 = 串行） |
| `MANUAL_MODE` | `0` | `1` = 手动模式（容器待命，需 CMD 执行 `python auto.py`） |
| `LOOP_INTERVAL` | `0` | 循环间隔（秒），`>0` 时自动循环执行 |
| `OTP_TIMEOUT` | `30` | 验证码等待超时（秒） |
| `MAX_FAIL` | `3` | 连续失败此次数后暂停任务 |
| `UPLOAD` | `0` | `1` = 注册成功后上传到 CLIProxyAPI |
| `UPLOAD_URL` | - | CLIProxyAPI 地址（如 `https://your-cpa.example.com`） |
| `UPLOAD_TOKEN` | - | CLIProxyAPI Management Key |
| `LOG_TO_FILE` | `0` | `1` = 日志写入文件 |

## 使用

### 自动模式

设置 `MANUAL_MODE=0`，容器启动后自动注册。

### 手动模式

设置 `MANUAL_MODE=1`，进入容器 CMD 执行：

```bash
python auto.py           # 使用环境变量配置
COUNT=5 python auto.py   # 临时指定数量
```

## 文件结构

```
├── auto.py           # 主程序
├── config.yaml       # 本地配置（环境变量优先）
├── domains.py        # 查询可用邮箱域名
├── Dockerfile        # Docker 构建
├── entrypoint.sh     # 启动脚本
├── requirements.txt  # Python 依赖
└── files/            # 注册结果 JSON 文件
```
