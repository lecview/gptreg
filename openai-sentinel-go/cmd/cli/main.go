package main

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "os"
    "time"

    sentinel "openai-sentinel-go"
)

type CLIInput struct {
    DeviceID  string `json:"device_id"`
    UserAgent string `json:"user_agent"`
    Flow      string `json:"flow"`
}

func main() {
    if len(os.Args) < 2 {
        fmt.Fprintln(os.Stderr, "Usage: sentinel-cli <json_input>")
        os.Exit(1)
    }

    var input CLIInput
    if err := json.Unmarshal([]byte(os.Args[1]), &input); err != nil {
        fmt.Fprintln(os.Stderr, "Invalid input JSON:", err)
        os.Exit(1)
    }

    svc := sentinel.NewService(sentinel.Config{
        SentinelBaseURL:     "https://sentinel.openai.com",
        SentinelTimeout:     15 * time.Second,
        SentinelMaxAttempts: 3,
    })

    client := &http.Client{Timeout: 15 * time.Second}
    // http.Client default transport already respects HTTP_PROXY, HTTPS_PROXY env vars

    session := &sentinel.Session{
        Client:              client,
        DeviceID:            input.DeviceID,
        UserAgent:           input.UserAgent,
        ScreenWidth:         1920,
        ScreenHeight:        1080,
        HeapLimit:           4294705152,
        HardwareConcurrency: 8,
        Language:            "en-US",
        LanguagesJoin:       "en-US,en",
        Persona: sentinel.Persona{
            Platform: "Win32",
            Vendor:   "Google Inc.",
        },
    }

    token, err := svc.Build(context.Background(), session, input.Flow, "https://sentinel.openai.com/backend-api/sentinel/frame.html?sv=20260219f9f6", "")
    if err != nil {
        fmt.Fprintln(os.Stderr, "Build Error:", err)
        os.Exit(1)
    }

    out, _ := json.Marshal(token)
    fmt.Println(string(out))
}
