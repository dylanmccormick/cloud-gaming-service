package main

import (
	"encoding/binary"
	"fmt"
)

type parserState int

type IVFParser struct {
	buffer []byte
	state parserState
	headerRead bool
	expectedFrameSize uint32
	ringBuffer *RingBuffer
}


const (
	ReadingHeader parserState = iota
	ReadingSize 
	ReadingFrame
)

func NewIVFParser(bufferSize int) *IVFParser {
	return &IVFParser {
		buffer: make([]byte, 0),
		ringBuffer: NewRingBuffer(bufferSize),
	}
}

func (p *IVFParser) ProcessData(newData []byte) {
	p.buffer = append(p.buffer, newData...)
	fmt.Printf("Buffer bytes: %v\n", p.buffer[:8])

	if p.state == ReadingHeader && len(p.buffer) >= 32 && !p.headerRead {
		header := p.buffer[:32]
		fmt.Printf("investigate %v\n", header)
		p.buffer = p.buffer[32:]
		p.incrementState()
		fmt.Println("investigate: read header")
		p.headerRead = true
	}

	if p.state == ReadingSize && len(p.buffer) >= 12 {
		p.expectedFrameSize = binary.LittleEndian.Uint32(p.buffer[0:4])
		fmt.Printf("investigate frame details: %v\n", p.buffer[:12])
		p.buffer = p.buffer[12:]
		p.incrementState()
		fmt.Printf("investigate: read size: %d\n", p.expectedFrameSize)
	}

	if p.state == ReadingFrame && len(p.buffer) >= int(p.expectedFrameSize) {
		fmt.Printf("reading frame with size: %d\n", p.expectedFrameSize)
		frame := make([]byte, p.expectedFrameSize)
		copy(frame, p.buffer[:p.expectedFrameSize])

		p.ringBuffer.Write(frame)

		p.buffer = p.buffer[p.expectedFrameSize:]
		p.incrementState()

		fmt.Printf("investigate: wrote to buffer\n")

	}

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

func (p *IVFParser) ReadFrame() ([]byte, bool){
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
