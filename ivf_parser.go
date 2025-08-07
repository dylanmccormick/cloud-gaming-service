package main

import (
	"context"
	"encoding/binary"
	"log/slog"
	"sync"
	"time"

	"github.com/dylanmccormick/webrtc-streamer/internal/logging"
)

type parserState int

type IVFParser struct {
	buffer            []byte
	state             parserState
	headerRead        bool
	expectedFrameSize uint32
	ringBuffer        *RingBuffer
	logger            *slog.Logger
}

const (
	ReadingHeader parserState = iota
	ReadingSize
	ReadingFrame
)

func NewIVFParser(ctx context.Context, bufferSize int) *IVFParser {
	return &IVFParser{
		buffer:     make([]byte, 0),
		ringBuffer: NewRingBuffer(ctx, bufferSize),
		logger:     logging.FromContext(ctx),
	}
}

func (p *IVFParser) ProcessData(newData []byte) {
	p.buffer = append(p.buffer, newData...)
	p.logger.Info("Attempting to process data", "data", p.buffer[:10])

	if p.state == ReadingHeader && len(p.buffer) >= 32 && !p.headerRead {
		p.buffer = p.buffer[32:]
		p.incrementState()
		p.headerRead = true
	}

	if p.state == ReadingSize && len(p.buffer) >= 12 {
		p.expectedFrameSize = binary.LittleEndian.Uint32(p.buffer[0:4])
		p.buffer = p.buffer[12:]
		p.incrementState()
	}

	if p.state == ReadingFrame && len(p.buffer) >= int(p.expectedFrameSize) {
		frame := make([]byte, p.expectedFrameSize)
		copy(frame, p.buffer[:p.expectedFrameSize])

		p.ringBuffer.Write(frame)

		p.buffer = p.buffer[p.expectedFrameSize:]
		p.incrementState()

	}

}

func (p *IVFParser) streamWrite(wg *sync.WaitGroup, output chan<- []byte, stopChan <-chan struct{}) {
	defer wg.Done()
	ticker := time.NewTicker(time.Second / 20)
	count := 0
	for {
		select {
		case <-stopChan:
			return
		case <-ticker.C:
			if frame, ok := p.ReadFrame(); ok {
				count += 1
				// p.logger.Info("Writing frame", "number", count)
				output <- frame
			} else {
				// p.logger.Warn("No return from ReadFrame")
			}
		}
	}

}

func (p *IVFParser) streamRead(wg *sync.WaitGroup, input <-chan []byte, stopChan <-chan struct{}) {
	defer wg.Done()
	count := 0
	for {
		select {
		case <-stopChan:
			return
		case bytes := <-input:
			count += 1
			p.logger.Info("Read", "count", count, "location", "parse ivf stream gofunc 2")
			if len(bytes) == 0 {
				p.logger.Warn("Didn't read anything from reader")
				return
			}

			p.ProcessData(bytes)

		}
	}

}

func (p *IVFParser) ParseStream(input <-chan []byte, output chan<- []byte, stopChan <-chan struct{}) {
	var wg sync.WaitGroup
	p.logger.Info("Starting goroutine streamRead")
	wg.Add(1)
	go p.streamRead(&wg, input, stopChan)

	p.logger.Info("Starting goroutine streamWrite")
	wg.Add(1)
	go p.streamWrite(&wg, output, stopChan)

	wg.Wait()
}

func (p *IVFParser) incrementState() {
	switch p.state {
	case ReadingHeader:
		p.state = ReadingSize
	case ReadingSize:
		p.state = ReadingFrame
	default:
		p.state = ReadingSize
	}
}

func (p *IVFParser) ReadFrame() ([]byte, bool) {
	return p.ringBuffer.Read()
}

func (p *IVFParser) GetState() int {
	return int(p.state)
}

func (p *IVFParser) GetBuffer() []byte {
	return p.buffer
}

func (p *IVFParser) GetExpectedFrameSize() uint32 {
	return p.expectedFrameSize
}

func (p *IVFParser) GetRingBuffer() *RingBuffer {
	return p.ringBuffer
}
