package sentinel

import (
	"math"
	"testing"
)

func TestBuildWindowUsesAuthPageDefaults(t *testing.T) {
	t.Parallel()

	solver := &turnstileSolver{
		profile: turnstileRequirementsProfile{
			UserAgent:           "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36",
			Language:            "zh-CN",
			LanguagesJoin:       "zh-CN,en-US",
			DocumentProbe:       "__reactContainer$b63yiita51i",
			WindowProbe:         "ondragend",
			HardwareConcurrency: 32,
			TimeOrigin:          1775190798250,
			PerformanceNow:      9270.399999976158,
		},
	}

	window := solver.buildWindow()
	windowKeys := objectKeys(window)
	expectPrefix(t, windowKeys, []any{"0", "window", "self", "document", "name", "location", "customElements", "history"})
	documentKeys := objectKeys(solver.jsGetProp(window, "document"))
	expectPrefix(t, documentKeys, []any{"location", "__reactContainer$b63yiita51i", "_reactListeningj3rmi50kcy"})

	location := solver.jsGetProp(window, "location")
	if got := solver.jsToString(location); got != "https://auth.openai.com/create-account/password" {
		t.Fatalf("unexpected location href: %q", got)
	}
	if got := solver.jsGetProp(solver.jsGetProp(window, "history"), "length"); got != float64(3) {
		t.Fatalf("unexpected history length: %#v", got)
	}
	if got := solver.jsGetProp(window, "innerWidth"); got != float64(800) {
		t.Fatalf("unexpected innerWidth: %#v", got)
	}
	if got := solver.jsGetProp(window, "innerHeight"); got != float64(600) {
		t.Fatalf("unexpected innerHeight: %#v", got)
	}
	if got := solver.jsGetProp(window, "devicePixelRatio"); got != 1.0000000149011612 {
		t.Fatalf("unexpected devicePixelRatio: %#v", got)
	}
	perfNow, err := solver.callFn(solver.jsGetProp(solver.jsGetProp(window, "performance"), "now"))
	if err != nil {
		t.Fatalf("performance.now error: %v", err)
	}
	value, ok := perfNow.(float64)
	if !ok {
		t.Fatalf("performance.now should be float64, got %#v", perfNow)
	}
	if math.Abs(value-9270.399999976158) > 50 {
		t.Fatalf("unexpected performance.now baseline: %f", value)
	}
}

func TestBuildWindowUsesEnglishSessionPersonaDefaults(t *testing.T) {
	t.Parallel()

	session := &Session{
		UserAgent:           "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36",
		Language:            "en-US",
		LanguagesJoin:       "en-US,en",
		ScreenWidth:         2048,
		ScreenHeight:        1152,
		HeapLimit:           4294705152,
		HardwareConcurrency: 22,
		Persona: Persona{
			Platform:       "Win32",
			Vendor:         "Google Inc.",
			DocumentProbe:  "__reactContainer$englishprobe",
			WindowProbe:    "ondblclick",
			PerformanceNow: 8822.900000035763,
		},
	}

	solver := &turnstileSolver{session: session}
	window := solver.buildWindow()
	document := solver.jsGetProp(window, "document")
	navigator := solver.jsGetProp(window, "navigator")

	if got := solver.jsGetProp(navigator, "language"); got != "en-US" {
		t.Fatalf("unexpected navigator.language: %#v", got)
	}
	if got := solver.jsGetProp(navigator, "hardwareConcurrency"); got != float64(22) {
		t.Fatalf("unexpected hardwareConcurrency: %#v", got)
	}
	documentKeys := objectKeys(document)
	expectPrefix(t, documentKeys, []any{
		"location",
		"__reactContainer$englishprobe",
		"_reactListeningj3rmi50kcy",
	})
	if _, ok := solver.jsGetProp(navigator, "xr").(map[string]any); !ok {
		t.Fatalf("expected navigator.xr object, got %#v", solver.jsGetProp(navigator, "xr"))
	}
	if _, ok := solver.jsGetProp(navigator, "clipboard").(map[string]any); !ok {
		t.Fatalf("expected navigator.clipboard object, got %#v", solver.jsGetProp(navigator, "clipboard"))
	}
	perfNow, err := solver.callFn(solver.jsGetProp(solver.jsGetProp(window, "performance"), "now"))
	if err != nil {
		t.Fatalf("performance.now error: %v", err)
	}
	value, ok := perfNow.(float64)
	if !ok {
		t.Fatalf("performance.now should be float64, got %#v", perfNow)
	}
	if math.Abs(value-8822.900000035763) > 50 {
		t.Fatalf("unexpected english performance.now baseline: %f", value)
	}
}

func expectPrefix(t *testing.T, got []any, want []any) {
	t.Helper()
	if len(got) < len(want) {
		t.Fatalf("prefix too short: got=%v want=%v", got, want)
	}
	for i, item := range want {
		if got[i] != item {
			t.Fatalf("unexpected prefix at %d: got=%v want=%v full=%v", i, got[i], item, got[:len(want)])
		}
	}
}

/*
LINUXDO：ius.
*/
