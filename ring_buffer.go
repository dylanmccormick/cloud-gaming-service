package main

import (
	"fmt"
	"sync"
)

type RingBuffer struct {
	frames   [][]byte
	writePos int
	readPos  int
	size     int
	count    int
	mutex    sync.RWMutex
}

func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		frames: make([][]byte, size),
		size:   size,
	}
}

func (rb *RingBuffer) Write(frame []byte) {
	fmt.Println("write start")
	rb.LogData()
	rb.mutex.Lock()
	defer rb.mutex.Unlock()

	frameCopy := make([]byte, len(frame))
	copy(frameCopy, frame)

	rb.frames[rb.writePos] = frameCopy
	fmt.Printf("write in pos: %d\n", rb.writePos)
	rb.writePos = (rb.writePos + 1) % rb.size

	if rb.count < rb.size {
		rb.count++
	} else {
		// Buffer full. Advance read pointer
		rb.readPos = (rb.readPos + 1) % rb.size
	}
	fmt.Println("write end")

}

func (rb *RingBuffer) Read() ([]byte, bool) {
	fmt.Println("read start")
	rb.LogData()
	rb.mutex.Lock()
	fmt.Println("read has lock")
	defer rb.mutex.Unlock()

	if rb.count == 0 {
		return nil, false
	}

	frame := rb.frames[rb.readPos]
	fmt.Printf("read readPos: %d\n", rb.readPos)
	rb.readPos = (rb.readPos + 1) % rb.size
	rb.count--

	fmt.Println("read end")
	return frame, true
}

func (rb *RingBuffer) Available() int {
	rb.mutex.RLock()
	defer rb.mutex.RUnlock()

	return rb.count
}

func (rb *RingBuffer) LogData() {
	fmt.Printf("\t\t\tRead Position : %5d\n", rb.readPos)
	fmt.Printf("\t\t\tWrite Position: %5d\n", rb.writePos)
	fmt.Printf("\t\t\tCount   	: %5d\n", rb.count)
	fmt.Printf("\t\t\tFrames 	: %5d\n", len(rb.frames))
}
