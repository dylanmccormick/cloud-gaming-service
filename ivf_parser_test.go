package main_test

import (
	"fmt"
	"os"
	"testing"
	main "github.com/dylanmccormick/webrtc-streamer"
)

func TestIVFProcessData(t *testing.T){
	// header := []byte{68, 75, 73, 70, 0, 0, 32, 0, 86, 80, 56, 48, 128, 2, 224, 25, 0, 0, 0, 1, 0, 0, 0, 255, 255, 255, 255, 0, 0, 0}
	// frame_metadata := []byte{157, 187, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	file, err := os.ReadFile("test.ivf")
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
	}

	fmt.Printf("%d\n", file[:44])

	parser := main.NewIVFParser(16)

	parser.ProcessData(file)
	frame2 := file[32:]
	for range(4){
		parser.ProcessData(frame2)
		fmt.Printf("Processor State: %v\n", parser.GetState())
		fmt.Printf("Buffer size: %d\n", len(parser.GetBuffer()))
	}







}
