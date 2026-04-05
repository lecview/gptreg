package sentinel

import (
	crand "crypto/rand"
	"encoding/hex"
	"math/rand"
	"time"
)

var seeded = rand.New(rand.NewSource(time.Now().UnixNano()))

func randomHex(n int) string {
	if n <= 0 {
		return ""
	}
	buf := make([]byte, (n+1)/2)
	_, _ = crand.Read(buf)
	text := hex.EncodeToString(buf)
	if len(text) > n {
		return text[:n]
	}
	return text
}

func randomChoice(values []string, fallback string) string {
	if len(values) == 0 {
		return fallback
	}
	return values[seeded.Intn(len(values))]
}

func randomInt(minValue, maxValue int) int {
	if maxValue <= minValue {
		return minValue
	}
	return minValue + seeded.Intn(maxValue-minValue+1)
}

/*
LINUXDO：ius.
*/
