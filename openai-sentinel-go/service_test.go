package sentinel

import (
	"encoding/base64"
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestRequirementsTokenUsesSessionPersonaFingerprint(t *testing.T) {
	t.Parallel()

	session := &Session{
		DeviceID:            "device-123",
		UserAgent:           "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36",
		ScreenWidth:         1920,
		ScreenHeight:        1080,
		HeapLimit:           4294705152,
		HardwareConcurrency: 32,
		Language:            "zh-CN",
		LanguagesJoin:       "zh-CN,en-US",
		Persona: Persona{
			Platform:          "Win32",
			Vendor:            "Google Inc.",
			SessionID:         "30ac1e73-e555-40f9-8ac4-76a1328458a3",
			TimeOrigin:        1775190798250,
			TimezoneOffsetMin: -420,
		},
	}

	sdk := newSDK(session)
	token := sdk.RequirementsToken()
	if !strings.HasSuffix(token, "~S") {
		t.Fatalf("requirements token should end with ~S, got %q", token)
	}
	fields := decodeSentinelTokenFields(t, strings.TrimSuffix(strings.TrimPrefix(token, "gAAAAAC"), "~S"))

	if got := int(fields[0].(float64)); got != session.ScreenWidth+session.ScreenHeight {
		t.Fatalf("unexpected screen sum: %d", got)
	}
	if got := fields[2].(float64); int64(got) != session.HeapLimit {
		t.Fatalf("unexpected heap limit: %v", got)
	}
	if got := fields[4].(string); got != session.UserAgent {
		t.Fatalf("unexpected user-agent: %q", got)
	}
	if got := fields[5].(string); !strings.Contains(got, "sentinel.openai.com") {
		t.Fatalf("expected sentinel sdk url, got %q", got)
	}
	if fields[6] != nil {
		t.Fatalf("expected build marker field to be nil by default, got %#v", fields[6])
	}
	if got := fields[7].(string); got != session.Language {
		t.Fatalf("unexpected language: %q", got)
	}
	if got := fields[8].(string); got != session.LanguagesJoin {
		t.Fatalf("unexpected languages join: %q", got)
	}
	if got := fields[14].(string); got != session.Persona.SessionID {
		t.Fatalf("unexpected sentinel session id: %q", got)
	}
	if got := int(fields[16].(float64)); got != session.HardwareConcurrency {
		t.Fatalf("unexpected hardware concurrency: %d", got)
	}
	if got := fields[17].(float64); math.Abs(got-session.Persona.TimeOrigin) > 0.1 {
		t.Fatalf("unexpected time origin: got=%f want=%f", got, session.Persona.TimeOrigin)
	}
}

func TestRequirementsTokenSupportsEnglishBrowserOverrides(t *testing.T) {
	t.Parallel()

	session := &Session{
		DeviceID:            "device-en-123",
		UserAgent:           "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36",
		ScreenWidth:         2048,
		ScreenHeight:        1152,
		HeapLimit:           4294705152,
		HardwareConcurrency: 22,
		Language:            "en-US",
		LanguagesJoin:       "en-US,en",
		Persona: Persona{
			Platform:              "Win32",
			Vendor:                "Google Inc.",
			SessionID:             "5cf4a195-5ff9-4998-b6ad-3b0052431084",
			TimeOrigin:            1775208087319.6,
			TimezoneOffsetMin:     420,
			WindowFlags:           [7]int{},
			WindowFlagsSet:        true,
			RequirementsScriptURL: "https://sentinel.openai.com/sentinel/20260219f9f6/sdk.js",
			NavigatorProbe:        "xr−[object XRSystem]",
			DocumentProbe:         "location",
			WindowProbe:           "ondblclick",
			PerformanceNow:        8822.900000035763,
			DateString:            "Fri Apr 03 2026 02:13:28 GMT-0700 (Mountain Standard Time)",
		},
	}

	sdk := newSDK(session)
	token := sdk.RequirementsToken()
	fields := decodeSentinelTokenFields(t, strings.TrimSuffix(strings.TrimPrefix(token, "gAAAAAC"), "~S"))

	if got := fields[1].(string); got != session.Persona.DateString {
		t.Fatalf("unexpected date string: %q", got)
	}
	if got := fields[5].(string); got != session.Persona.RequirementsScriptURL {
		t.Fatalf("unexpected sdk url: %q", got)
	}
	if got := fields[10].(string); got != session.Persona.NavigatorProbe {
		t.Fatalf("unexpected navigator probe: %q", got)
	}
	if got := fields[11].(string); got != session.Persona.DocumentProbe {
		t.Fatalf("unexpected document probe: %q", got)
	}
	if got := fields[12].(string); got != session.Persona.WindowProbe {
		t.Fatalf("unexpected window probe: %q", got)
	}
	if got := fields[13].(float64); math.Abs(got-session.Persona.PerformanceNow) > 0.0001 {
		t.Fatalf("unexpected performance.now: got=%f want=%f", got, session.Persona.PerformanceNow)
	}
}

func TestBrowserEnglishTokenPSampleMatchesLocalFingerprintConfig(t *testing.T) {
	t.Parallel()

	const browserP = "gAAAAABWzMyMDAsIkZyaSBBcHIgMDMgMjAyNiAwMjoyMTozNiBHTVQtMDcwMCAoTW91bnRhaW4gU3RhbmRhcmQgVGltZSkiLDQyOTQ3MDUxNTIsODksIk1vemlsbGEvNS4wIChXaW5kb3dzIE5UIDEwLjA7IFdpbjY0OyB4NjQpIEFwcGxlV2ViS2l0LzUzNy4zNiAoS0hUTUwsIGxpa2UgR2Vja28pIENocm9tZS8xNDIuMC4wLjAgU2FmYXJpLzUzNy4zNiIsImh0dHBzOi8vc2VudGluZWwub3BlbmFpLmNvbS9zZW50aW5lbC8yMDI2MDIxOWY5ZjYvc2RrLmpzIixudWxsLCJlbi1VUyIsImVuLVVTLGVuIiwxMiwieHLiiJJbb2JqZWN0IFhSU3lzdGVtXSIsImxvY2F0aW9uIiwib25kYmxjbGljayIsODgyMi45MDAwMDAwMzU3NjMsIjVjZjRhMTk1LTVmZjktNDk5OC1iNmFkLTNiMDA1MjQzMTA4NCIsIiIsMjIsMTc3NTIwODA4NzMxOS42LDAsMCwwLDAsMCwwLDBd~S"

	session := &Session{
		UserAgent:           "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36",
		ScreenWidth:         2048,
		ScreenHeight:        1152,
		HeapLimit:           4294705152,
		HardwareConcurrency: 22,
		Language:            "en-US",
		LanguagesJoin:       "en-US,en",
		Persona: Persona{
			Platform:              "Win32",
			Vendor:                "Google Inc.",
			SessionID:             "5cf4a195-5ff9-4998-b6ad-3b0052431084",
			TimeOrigin:            1775208087319.6,
			TimezoneOffsetMin:     420,
			WindowFlags:           [7]int{},
			WindowFlagsSet:        true,
			RequirementsScriptURL: "https://sentinel.openai.com/sentinel/20260219f9f6/sdk.js",
			NavigatorProbe:        "xr−[object XRSystem]",
			DocumentProbe:         "location",
			WindowProbe:           "ondblclick",
			PerformanceNow:        8822.900000035763,
			DateString:            "Fri Apr 03 2026 02:21:36 GMT-0700 (Mountain Standard Time)",
		},
	}

	expected := decodeSentinelTokenFields(t, strings.TrimSuffix(strings.TrimPrefix(browserP, "gAAAAAB"), "~S"))
	got := newSDK(session).fingerprintConfig(true, 89, 12)
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(expected)
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("browser sample mismatch:\n got: %s\nwant:%s", gotJSON, wantJSON)
	}
}

func decodeSentinelTokenFields(t *testing.T, encoded string) []any {
	t.Helper()
	body, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode sentinel token: %v", err)
	}
	var fields []any
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatalf("unmarshal sentinel fields: %v", err)
	}
	if len(fields) < 18 {
		t.Fatalf("unexpected sentinel field count: %d", len(fields))
	}
	return fields
}

/*
LINUXDO：ius.
*/
