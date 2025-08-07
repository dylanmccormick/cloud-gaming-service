package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/dylanmccormick/webrtc-streamer/internal/logging"
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
	EncodeFrameRate  int
	Bounds           image.Rectangle
	LogLevel         LogLevel
	Port             int
}

func main() {
	defaultAttrs := []slog.Attr{}
	slogHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}).
		WithAttrs(defaultAttrs)

	logger := slog.New(slogHandler)

	ctx := context.Background()
	ctx = logging.WithContext(ctx, logger)

	logger.Debug("Starting program...\n")
	c := Config{
		CaptureFrameRate: 20,
		EncodeFrameRate:  20,
		Bounds:           image.Rect(0, 0, 640, 480),
		LogLevel:         INFO,
		Port:             42069,
	}

	stopChan := make(chan struct{})
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		close(stopChan)
	}()

	go runServer(c, ctx)
	// runStream(c, ctx, stopChan)

	select {
	case <-stopChan:
		logger.Info("Main Function Stopping")
		return
	}

}

func runStream(c Config, ctx context.Context, stopChan chan struct{}) {
	logger := logging.FromContext(ctx)

	captureErr := make(chan error)
	frameChan := make(chan []byte)

	go func() {
		defer close(captureErr)
		c.captureScreenshots(ctx, frameChan, stopChan, captureErr)
	}()

	encodeOutputChan := make(chan []byte, 10)
	encodeErr := make(chan error)

	ve := NewVideoEncoder(ctx, &c)
	go ve.StartStream(frameChan, encodeOutputChan, stopChan, encodeErr)
	go captureErrors(ctx, captureErr, encodeErr, stopChan)

	parser := NewIVFParser(ctx, 16)

	outputChan := make(chan []byte, 10)

	parser.ParseStream(encodeOutputChan, outputChan, stopChan)

	count := 0
	for {
		select {
		case <-stopChan:
			logger.Info("Main Function Stopping")
			return
		case _, ok := <-outputChan:
			if !ok {
				logger.Error("Output channel closed. Stopping main...")
				return
			}

			logger.Info("Reading frame", "count", count, "location", "main")
			count += 1
		}
	}

}

func getFrameFromIVF(ivfData []byte) ([]byte, error) {
	frameSize := binary.LittleEndian.Uint32(ivfData[32:36])
	return ivfData[44 : 44+frameSize], nil
}

func captureErrors(ctx context.Context, captureErr, encodeErr chan error, stopChan chan struct{}) {
	logger := logging.FromContext(ctx)

	for {
		select {
		case err := <-captureErr:
			if err != nil {
				logger.Error("Capture error", "error", err)
				return
			}
		case err := <-encodeErr:
			if err != nil {
				logger.Error("Encoding error", "error", err)
				return
			}
		case <-stopChan:
			return
		}
	}
}

func httpSDPServer(port int) chan string {
	sdpChan := make(chan string)
	http.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		fmt.Fprintf(res, "done") //nolint: errcheck
		sdpChan <- string(body)
	})

	go func() {
		// nolint: gosec
		panic(http.ListenAndServe(":"+strconv.Itoa(port), nil))
	}()

	return sdpChan
}
