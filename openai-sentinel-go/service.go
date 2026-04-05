package sentinel

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	SentinelBaseURL        string
	SentinelTimeout        time.Duration
	SentinelMaxAttempts    int
	SentinelDirectFallback bool
	TurnstileStaticToken   string
}

type Persona struct {
	Platform              string
	Vendor                string
	TimezoneOffsetMin     int
	SessionID             string
	TimeOrigin            float64
	WindowFlags           [7]int
	WindowFlagsSet        bool
	EntropyA              float64
	EntropyB              float64
	DateString            string
	RequirementsScriptURL string
	NavigatorProbe        string
	DocumentProbe         string
	WindowProbe           string
	PerformanceNow        float64
	RequirementsElapsed   float64
}

type Session struct {
	Client              *http.Client
	DeviceID            string
	UserAgent           string
	ScreenWidth         int
	ScreenHeight        int
	HeapLimit           int64
	HardwareConcurrency int
	Language            string
	LanguagesJoin       string
	Persona             Persona
}

func (s *Session) Do(req *http.Request) (*http.Response, error) {
	if s == nil {
		return nil, fmt.Errorf("nil session")
	}
	if s.Client == nil {
		return nil, fmt.Errorf("nil session client")
	}
	return s.Client.Do(req)
}

type Token struct {
	P    string `json:"p"`
	T    string `json:"t,omitempty"`
	C    string `json:"c,omitempty"`
	ID   string `json:"id"`
	Flow string `json:"flow"`
}

type Service struct {
	cfg Config
}

type sentinelResponse struct {
	Token     string `json:"token"`
	Turnstile struct {
		Required bool   `json:"required"`
		DX       string `json:"dx"`
	} `json:"turnstile"`
	ProofOfWork struct {
		Required   bool   `json:"required"`
		Seed       string `json:"seed"`
		Difficulty string `json:"difficulty"`
	} `json:"proofofwork"`
}

func NewService(cfg Config) *Service {
	if strings.TrimSpace(cfg.SentinelBaseURL) == "" {
		cfg.SentinelBaseURL = "https://sentinel.openai.com"
	}
	if cfg.SentinelTimeout <= 0 {
		cfg.SentinelTimeout = 10 * time.Second
	}
	if cfg.SentinelMaxAttempts <= 0 {
		cfg.SentinelMaxAttempts = 2
	}
	cfg.SentinelBaseURL = strings.TrimRight(cfg.SentinelBaseURL, "/")
	return &Service{cfg: cfg}
}

func (s *Service) Build(ctx context.Context, session *Session, flow, referer, turnstileToken string) (Token, error) {
	sdk := newSDK(session)
	pInitial := sdk.RequirementsToken()
	token := Token{P: pInitial, ID: sdk.deviceID, Flow: flow}

	reqBody, _ := json.Marshal(map[string]string{"p": pInitial, "id": sdk.deviceID, "flow": flow})
	var parsed sentinelResponse
	var err error
	maxAttempts := int(math.Max(1, float64(s.cfg.SentinelMaxAttempts)))
	for attempt := 0; attempt < maxAttempts; attempt++ {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.SentinelBaseURL+"/backend-api/sentinel/req", bytes.NewReader(reqBody))
		if reqErr != nil {
			return token, reqErr
		}
		if session != nil && session.UserAgent != "" {
			req.Header.Set("User-Agent", session.UserAgent)
		}
		req.Header.Set("Accept", "*/*")
		req.Header.Set("Content-Type", "text/plain;charset=UTF-8")
		req.Header.Set("Origin", s.cfg.SentinelBaseURL)
		if strings.TrimSpace(referer) != "" {
			req.Header.Set("Referer", referer)
		}
		var resp *http.Response
		var requestErr error
		if s.cfg.SentinelDirectFallback || session == nil || session.Client == nil {
			client := &http.Client{Timeout: s.cfg.SentinelTimeout}
			resp, requestErr = client.Do(req)
		} else {
			resp, requestErr = session.Do(req)
		}
		if requestErr != nil {
			err = requestErr
			time.Sleep(time.Duration(attempt+1) * 200 * time.Millisecond)
			continue
		}
		err = nil
		decodeErr := json.NewDecoder(resp.Body).Decode(&parsed)
		_ = resp.Body.Close()
		if decodeErr != nil {
			err = decodeErr
			time.Sleep(time.Duration(attempt+1) * 200 * time.Millisecond)
			continue
		}
		break
	}
	if err != nil {
		token.T = strings.TrimSpace(turnstileToken)
		if token.T == "" {
			token.T = s.cfg.TurnstileStaticToken
		}
		return token, nil
	}

	token.C = parsed.Token
	token.T = strings.TrimSpace(turnstileToken)
	if token.T == "" {
		token.T = s.cfg.TurnstileStaticToken
	}
	if parsed.Turnstile.Required && token.T == "" {
		token.T, err = solveTurnstileDXWithSession(pInitial, parsed.Turnstile.DX, session)
		if err != nil {
			return token, fmt.Errorf("solve turnstile dx: %w", err)
		}
	}
	token.P = sdk.EnforcementToken(parsed.ProofOfWork.Required, parsed.ProofOfWork.Seed, parsed.ProofOfWork.Difficulty)
	return token, nil
}

type sdk struct {
	deviceID            string
	sessionID           string
	userAgent           string
	screenWidth         int
	screenHeight        int
	heapLimit           int64
	hardwareConcurrency int
	timeOrigin          float64
	perfStart           time.Time
	language            string
	languagesJoin       string
	platform            string
	vendor              string
	timezoneOffsetMin   int
	windowFlags         [7]int
	entropyA            float64
	entropyB            float64
	dateStringOverride  string
	scriptURL           string
	navigatorProbeValue string
	documentProbeValue  string
	windowProbeValue    string
	performanceNowValue float64
	requirementsElapsed float64
}

func newSDK(session *Session) sdk {
	flags := [][7]int{{0, 0, 0, 0, 0, 0, 0}, {1, 0, 0, 0, 0, 0, 0}, {0, 1, 0, 0, 0, 0, 0}}
	flagChoice := flags[randomInt(0, len(flags)-1)]
	width := 1920
	height := 1080
	heapLimit := int64(4294967296)
	hardwareConcurrency := 8
	language := "en-US"
	languagesJoin := "en-US,en"
	platform := "Win32"
	vendor := "Google Inc."
	timezoneOffsetMin := 0
	deviceID := ""
	ua := ""
	sessionID := randomHex(32)
	timeOrigin := float64(time.Now().Add(-time.Duration(randomInt(5000, 120000)) * time.Millisecond).UnixMilli())
	entropyA := browserEntropyFallback()
	entropyB := browserEntropyFallback()
	dateStringOverride := ""
	scriptURL := defaultSentinelSDKURL
	navigatorProbeValue, documentProbeValue, windowProbeValue := sentinelProbeDefaults(language)
	performanceNowValue := 0.0
	requirementsElapsed := 0.0
	if session != nil {
		deviceID = session.DeviceID
		ua = session.UserAgent
		if session.ScreenWidth > 0 {
			width = session.ScreenWidth
		}
		if session.ScreenHeight > 0 {
			height = session.ScreenHeight
		}
		if session.HeapLimit > 0 {
			heapLimit = session.HeapLimit
		}
		if session.HardwareConcurrency > 0 {
			hardwareConcurrency = session.HardwareConcurrency
		}
		if strings.TrimSpace(session.Language) != "" {
			language = session.Language
		}
		if strings.TrimSpace(session.LanguagesJoin) != "" {
			languagesJoin = session.LanguagesJoin
		}
		if strings.TrimSpace(session.Persona.Platform) != "" {
			platform = session.Persona.Platform
		}
		if strings.TrimSpace(session.Persona.Vendor) != "" {
			vendor = session.Persona.Vendor
		}
		if session.Persona.TimezoneOffsetMin != 0 {
			timezoneOffsetMin = session.Persona.TimezoneOffsetMin
		}
		if session.Persona.SessionID != "" {
			sessionID = session.Persona.SessionID
		}
		if session.Persona.TimeOrigin > 0 {
			timeOrigin = session.Persona.TimeOrigin
		}
		if session.Persona.WindowFlagsSet || session.Persona.WindowFlags != [7]int{} {
			flagChoice = session.Persona.WindowFlags
		}
		if session.Persona.EntropyA > 0 {
			entropyA = session.Persona.EntropyA
		}
		if session.Persona.EntropyB > 0 {
			entropyB = session.Persona.EntropyB
		}
		if strings.TrimSpace(session.Persona.DateString) != "" {
			dateStringOverride = strings.TrimSpace(session.Persona.DateString)
		}
		if strings.TrimSpace(session.Persona.RequirementsScriptURL) != "" {
			scriptURL = strings.TrimSpace(session.Persona.RequirementsScriptURL)
		}
		if strings.TrimSpace(session.Persona.NavigatorProbe) != "" {
			navigatorProbeValue = strings.TrimSpace(session.Persona.NavigatorProbe)
		}
		if strings.TrimSpace(session.Persona.DocumentProbe) != "" {
			documentProbeValue = strings.TrimSpace(session.Persona.DocumentProbe)
		}
		if strings.TrimSpace(session.Persona.WindowProbe) != "" {
			windowProbeValue = strings.TrimSpace(session.Persona.WindowProbe)
		}
		if session.Persona.PerformanceNow > 0 {
			performanceNowValue = session.Persona.PerformanceNow
		}
		if session.Persona.RequirementsElapsed > 0 {
			requirementsElapsed = session.Persona.RequirementsElapsed
		}
	}
	return sdk{
		deviceID:            deviceID,
		sessionID:           sessionID,
		userAgent:           ua,
		screenWidth:         width,
		screenHeight:        height,
		heapLimit:           heapLimit,
		hardwareConcurrency: hardwareConcurrency,
		timeOrigin:          timeOrigin,
		perfStart:           time.Now(),
		language:            language,
		languagesJoin:       languagesJoin,
		platform:            platform,
		vendor:              vendor,
		timezoneOffsetMin:   timezoneOffsetMin,
		windowFlags:         flagChoice,
		entropyA:            entropyA,
		entropyB:            entropyB,
		dateStringOverride:  dateStringOverride,
		scriptURL:           scriptURL,
		navigatorProbeValue: navigatorProbeValue,
		documentProbeValue:  documentProbeValue,
		windowProbeValue:    windowProbeValue,
		performanceNowValue: performanceNowValue,
		requirementsElapsed: requirementsElapsed,
	}
}

const defaultSentinelSDKURL = "https://sentinel.openai.com/sentinel/20260219f9f6/sdk.js"

func (s sdk) perfNow() float64 {
	if s.performanceNowValue > 0 {
		return s.performanceNowValue
	}
	return float64(time.Since(s.perfStart).Milliseconds())
}

func (s sdk) requirementsElapsedNow() float64 {
	if s.requirementsElapsed > 0 {
		return s.requirementsElapsed
	}
	return float64(time.Since(s.perfStart).Milliseconds())
}

func (s sdk) dateString() string {
	if strings.TrimSpace(s.dateStringOverride) != "" {
		return strings.TrimSpace(s.dateStringOverride)
	}
	offsetMinutes := -s.timezoneOffsetMin
	sign := "+"
	if offsetMinutes < 0 {
		sign = "-"
		offsetMinutes = -offsetMinutes
	}
	hours := offsetMinutes / 60
	minutes := offsetMinutes % 60
	zoneSeconds := (hours*60 + minutes) * 60
	if sign == "-" {
		zoneSeconds = -zoneSeconds
	}
	localNow := time.Now().UTC().Add(time.Duration(zoneSeconds) * time.Second)
	label := localizedTimezoneName(s.timezoneOffsetMin, s.language)
	if label == "" {
		return fmt.Sprintf("%s GMT%s%02d%02d", localNow.Format("Mon Jan 02 2006 15:04:05"), sign, hours, minutes)
	}
	return fmt.Sprintf("%s GMT%s%02d%02d (%s)", localNow.Format("Mon Jan 02 2006 15:04:05"), sign, hours, minutes, label)
}

func (s sdk) fingerprintConfig(forPow bool, nonce int, elapsed int64) []any {
	field3 := any(s.entropyA)
	field9 := any(s.entropyB)
	if forPow {
		field3 = nonce
		field9 = elapsed
	}
	return []any{
		s.screenWidth + s.screenHeight,
		s.dateString(),
		s.heapLimit,
		field3,
		s.userAgent,
		s.scriptURL,
		nil,
		s.language,
		s.languagesJoin,
		field9,
		s.navigatorProbe(),
		s.documentProbe(),
		s.windowProbe(),
		s.perfNow(),
		s.sessionID,
		"",
		s.hardwareConcurrency,
		s.timeOrigin,
		s.windowFlags[0],
		s.windowFlags[1],
		s.windowFlags[2],
		s.windowFlags[3],
		s.windowFlags[4],
		s.windowFlags[5],
		s.windowFlags[6],
	}
}

func browserEntropyFallback() float64 { return float64(randomInt(10000, 99999)) / 100000 }

func (s sdk) RequirementsToken() string {
	cfg := s.fingerprintConfig(false, 1, 0)
	cfg[3] = 1
	cfg[9] = s.requirementsElapsedNow()
	return "gAAAAAC" + mustB64JSON(cfg) + "~S"
}

func (s sdk) EnforcementToken(required bool, seed, difficulty string) string {
	if required {
		answer, _ := s.solve(seed, difficulty)
		return "gAAAAAB" + answer
	}
	return "gAAAAAB" + mustB64JSON(s.fingerprintConfig(false, 0, 0)) + "~S"
}

func (s sdk) solve(seed, difficulty string) (string, int) {
	if difficulty == "" {
		difficulty = "0"
	}
	cfg := s.fingerprintConfig(true, 0, 0)
	start := time.Now()
	for nonce := 0; nonce < 500000; nonce++ {
		cfg[3] = nonce
		cfg[9] = time.Since(start).Milliseconds()
		answer := mustB64JSON(cfg)
		if strings.Compare(mixedFNV(seed + answer)[:len(difficulty)], difficulty) <= 0 {
			return answer + "~S", nonce + 1
		}
	}
	return "wQ8Lk5FbGpA2NcR9dShT6gYjU7VxZ4Dtimeout", 500000
}

func mustB64JSON(value any) string {
	body, _ := json.Marshal(value)
	return base64.StdEncoding.EncodeToString(body)
}

func (s sdk) navigatorProbe() string {
	if strings.TrimSpace(s.navigatorProbeValue) != "" {
		return s.navigatorProbeValue
	}
	probes := []string{
		"hardwareConcurrency−" + strconv.Itoa(maxInt(1, s.hardwareConcurrency)),
		"language−" + strings.TrimSpace(s.language),
		"languages−" + strings.TrimSpace(s.languagesJoin),
		"platform−" + strings.TrimSpace(s.platform),
	}
	if strings.TrimSpace(s.vendor) != "" {
		probes = append(probes, "vendor−"+strings.TrimSpace(s.vendor))
	}
	return randomChoice(probes, probes[0])
}

func (s sdk) documentProbe() string {
	if strings.TrimSpace(s.documentProbeValue) != "" {
		return s.documentProbeValue
	}
	sessionSuffix := strings.ToLower(strings.TrimSpace(s.sessionID))
	if sessionSuffix == "" {
		sessionSuffix = randomHex(11)
	}
	if len(sessionSuffix) > 11 {
		sessionSuffix = sessionSuffix[:11]
	}
	probes := []string{"__reactContainer$" + sessionSuffix, "onvisibilitychange", "hidden", "readyState", "characterSet"}
	return randomChoice(probes, probes[0])
}

func (s sdk) windowProbe() string {
	if strings.TrimSpace(s.windowProbeValue) != "" {
		return s.windowProbeValue
	}
	probes := []string{"__oai_so_bm", "ondragend", "onbeforematch", "__next_f", "__oai_cached_session"}
	return randomChoice(probes, probes[0])
}

func localizedTimezoneName(timezoneOffsetMin int, language string) string {
	lang := strings.ToLower(strings.TrimSpace(language))
	switch timezoneOffsetMin {
	case 420:
		if strings.HasPrefix(lang, "zh") {
			return "北美山区标准时间"
		}
		return "Mountain Standard Time"
	case 480:
		if strings.HasPrefix(lang, "zh") {
			return "太平洋标准时间"
		}
		return "Pacific Standard Time"
	case 0:
		if strings.HasPrefix(lang, "zh") {
			return "协调世界时"
		}
		return "Coordinated Universal Time"
	default:
		return ""
	}
}

func sentinelProbeDefaults(language string) (navigatorProbe, documentProbe, windowProbe string) {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(language)), "zh") {
		return "clipboard−[object Clipboard]", "__reactContainer$b63yiita51i", "releaseEvents"
	}
	return "xr−[object XRSystem]", "location", "ondblclick"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func mixedFNV(text string) string {
	var e uint32 = 2166136261
	for _, ch := range text {
		e ^= uint32(ch)
		e *= 16777619
	}
	e ^= e >> 16
	e *= 2246822507
	e ^= e >> 13
	e *= 3266489909
	e ^= e >> 16
	return fmt.Sprintf("%08x", e)
}

/*
LINUXDO：ius.
*/
