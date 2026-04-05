# openai-sentinel-go

一个可单独分享的纯 Go 版 OpenAI Sentinel 实现，包含：

- requirements token 生成
- enforcement token / PoW 求解
- turnstile dx VM 求解
- 最小可用的会话与 persona 类型
- 针对当前实现的基础测试

另外，`Persona` 支持覆盖最新浏览器样本中的关键字段，适合做中英文环境精确对齐：

- `DateString`
- `RequirementsScriptURL`
- `NavigatorProbe`
- `DocumentProbe`
- `WindowProbe`
- `PerformanceNow`
- `RequirementsElapsed`

## 文件说明

- `service.go`：核心对外 API、token 生成、PoW
- `turnstile_vm.go`：dx VM 与浏览器环境重建
- `random.go`：最小随机辅助函数
- `*_test.go`：基础回归测试

## 最小用法

```go
package main

import (
    "context"
    "fmt"
    "net/http"
    "time"

    sentinel "openai-sentinel-go"
)

func main() {
    svc := sentinel.NewService(sentinel.Config{
        SentinelBaseURL:     "https://sentinel.openai.com",
        SentinelTimeout:     10 * time.Second,
        SentinelMaxAttempts: 2,
    })

    session := &sentinel.Session{
        Client:              &http.Client{Timeout: 10 * time.Second},
        DeviceID:            "device-123",
        UserAgent:           "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36",
        ScreenWidth:         1920,
        ScreenHeight:        1080,
        HeapLimit:           4294705152,
        HardwareConcurrency: 32,
        Language:            "zh-CN",
        LanguagesJoin:       "zh-CN,en-US",
        Persona: sentinel.Persona{
            Platform:   "Win32",
            Vendor:     "Google Inc.",
            SessionID:  "30ac1e73-e555-40f9-8ac4-76a1328458a3",
            TimeOrigin: 1775190798250,
        },
    }

    token, err := svc.Build(context.Background(), session, "username_password_create", "https://auth.openai.com/create-account/password", "")
    if err != nil {
        panic(err)
    }
    fmt.Printf("sentinel token: %+v\n", token)
}
```

## 验证

```bash
cd share/openai-sentinel-go
gofmt -w .
go test ./...
```

### 作者

LINUXDO：ius.
