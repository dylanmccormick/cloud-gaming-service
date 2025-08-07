package main

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/dylanmccormick/webrtc-streamer/internal/logging"
)

type RingBuffer struct {
	frames   [][]byte
	writePos int
	readPos  int
	size     int
	count    int
	mutex    sync.RWMutex
	logger   *slog.Logger
}

func NewRingBuffer(ctx context.Context, size int) *RingBuffer {
	return &RingBuffer{
		frames: make([][]byte, size),
		size:   size,
		logger: logging.FromContext(ctx),
	}
}

func (rb *RingBuffer) Write(frame []byte) {
	rb.mutex.Lock()
	defer rb.mutex.Unlock()

	frameCopy := make([]byte, len(frame))
	copy(frameCopy, frame)

	rb.frames[rb.writePos] = frameCopy
	rb.writePos = (rb.writePos + 1) % rb.size

	if rb.count < rb.size {
		rb.count++
	} else {
		// Buffer full. Advance read pointer
		rb.readPos = (rb.readPos + 1) % rb.size
	}

}

func (rb *RingBuffer) Read() ([]byte, bool) {
	rb.mutex.Lock()
	defer rb.mutex.Unlock()

	if rb.count == 0 {
		return nil, false
	}

	frame := rb.frames[rb.readPos]
	rb.logger.Debug("Ring Buffer", "readPos", rb.readPos)
	rb.readPos = (rb.readPos + 1) % rb.size
	rb.count--

	rb.logger.Debug("end read")
	return frame, true
}

func (rb *RingBuffer) Available() int {
	rb.mutex.RLock()
	defer rb.mutex.RUnlock()

	return rb.count
}

func (rb *RingBuffer) LogData() {
	rb.logger.Info(fmt.Sprintf("\t\t\tRead Position : %5d\n", rb.readPos))
	rb.logger.Info(fmt.Sprintf("\t\t\tWrite Position: %5d\n", rb.writePos))
	rb.logger.Info(fmt.Sprintf("\t\t\tCount   	: %5d\n", rb.count))
	rb.logger.Info(fmt.Sprintf("\t\t\tFrames 	: %5d\n", len(rb.frames)))
}
