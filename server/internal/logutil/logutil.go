package logutil

import (
	"log"
	"os"
	"strings"
	"sync/atomic"
)

var debugEnabled atomic.Bool

func init() {
	debugEnabled.Store(envBool("CAPTUREQUEST_DEBUG_LOG") || envBool("VERBOSE_DEBUG_LOG"))
}

func DebugEnabled() bool {
	return debugEnabled.Load()
}

func Debugf(format string, args ...any) {
	if debugEnabled.Load() {
		log.Printf(format, args...)
	}
}

func envBool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
