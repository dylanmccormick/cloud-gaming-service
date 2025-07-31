package main

import (
	"github.com/dylanmccormick/webrtc-streamer/internal/logging"
	"context"
	"encoding/binary"
	"image"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type LogLevel int

const (
	SILENT LogLevel = iota
	INFO
	ERROR
	DEBUG
)

type Config struct {
	CaptureFrameRate int
	EncodeFrameRate int
	Bounds image.Rectangle
	LogLevel LogLevel
}

func main() {
	defaultAttrs := []slog.Attr{}
	slogHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}).
		WithAttrs(defaultAttrs)

	logger := slog.New(slogHandler)

	ctx := context.Background()
	ctx = logging.WithContext(ctx, logger)

	logger.Debug("Starting program...\n")
	c := Config {
		CaptureFrameRate: 20, 
		EncodeFrameRate: 20, 
		Bounds: image.Rect(0,0,640,480),
		LogLevel: INFO,
	}
	stopChan := make(chan struct{})
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		close(stopChan)
	}()

	captureErr := make(chan error)
	frameChan := make(chan []byte)

	go func() {
		defer close(captureErr)
		c.captureScreenshots(ctx, frameChan, stopChan, captureErr)
	}()

	outputChan := make(chan []byte, 10)
	encodeErr := make(chan error)
	
	ve := NewVideoEncoder(ctx, &c)
	go ve.StartStream(frameChan, outputChan, stopChan, encodeErr)
	go captureErrors(ctx, captureErr, encodeErr, stopChan)


	count := 0
	for {
		select {
		case <-stopChan:
			logger.Info("Main Function Stopping")
			return
		case _, ok := <- outputChan:
			if !ok {
				logger.Error("Output channel closed. Stopping main...")
				return
			}

			logger.Info("Reading frame", "count", count, "location", "main")
			count += 1
		}
	}

}



func parseIVFStream(ctx context.Context, reader io.Reader, output chan<- []byte, stopChan <-chan struct{}) {
	logger := logging.FromContext(ctx)
	parser := NewIVFParser(ctx, 16)
	buffer := make([]byte, 4096)

	go func ()  {
		ticker := time.NewTicker(time.Second / 20)
		count := 0
		for {
			select {
			case <-stopChan:
				return 
			case <-ticker.C:
				if frame, ok := parser.ReadFrame(); ok{
					count += 1
					logger.Info("Writing frame", "number", count)
					output <- frame
				}else {
					logger.Warn("No return from ReadFrame")
				}
			}
		}
	}()

	go func() {
		count := 0
		for {
			select {
			case <- stopChan:
				return
			default:
				count += 1
				logger.Info("Read", "count", count, "location", "parse ivf stream gofunc 2")
				bytes, err := reader.Read(buffer)
				if err != nil {
					logger.Error("Unable to read", "error", err)
					return
				}
				if bytes == 0 {
					logger.Warn("Didn't read anything from reader")
					return
				}

				parser.ProcessData(buffer)


			}
		}
	}()


}
func getFrameFromIVF(ivfData []byte) ([]byte, error) {
	frameSize := binary.LittleEndian.Uint32(ivfData[32:36])
	return ivfData[44:44+frameSize], nil
}

func captureErrors(ctx context.Context, captureErr, encodeErr chan error, stopChan chan struct{}){
	logger := logging.FromContext(ctx)
	
	for {
		select {
		case err := <- captureErr:
			if err != nil {
				logger.Error("Capture error", "error", err)
				return
			}
		case err := <- encodeErr:
			if err != nil {
				logger.Error("Encoding error", "error", err)
				return
			}
		case <- stopChan:
			return
		}
	}
}
