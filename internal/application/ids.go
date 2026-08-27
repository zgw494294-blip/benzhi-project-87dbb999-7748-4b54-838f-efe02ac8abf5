package application

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"
)

type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now().UTC() }

type RandomIDGenerator struct{ counter atomic.Uint64 }

func (g *RandomIDGenerator) New(prefix string) string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(raw[:]))
	}
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano(), g.counter.Add(1))
}
