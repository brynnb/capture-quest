package server

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

// GlobalLogBuffer holds the last N lines of logs
var GlobalLogBuffer *LogBuffer

type LogBuffer struct {
	mu       sync.RWMutex
	lines    []string
	maxLines int
}

func NewLogBuffer(maxLines int) *LogBuffer {
	return &LogBuffer{
		lines:    make([]string, 0, maxLines),
		maxLines: maxLines,
	}
}

func (lb *LogBuffer) Write(p []byte) (n int, err error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	line := string(p)
	if len(lb.lines) >= lb.maxLines {
		lb.lines = lb.lines[1:]
	}
	lb.lines = append(lb.lines, line)
	return len(p), nil
}

func (lb *LogBuffer) GetLines() []string {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	// Return a copy to avoid timing issues.
	res := make([]string, len(lb.lines))
	copy(res, lb.lines)
	return res
}

// InitLogging sets up the global log buffer and multi-writer
func InitLogging() {
	GlobalLogBuffer = NewLogBuffer(500) // Keep last 500 lines

	// Create a multi-writer to both stdout and our buffer
	multi := io.MultiWriter(os.Stdout, GlobalLogBuffer)
	log.SetOutput(multi)

	fmt.Println("[Server] Log buffer initialized.")
}
