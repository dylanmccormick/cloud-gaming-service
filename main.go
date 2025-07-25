package main

import (
	"github.com/dylanmccormick/webrtc-streamer/internal/logging"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/kbinani/screenshot"
	ffmpeg "github.com/u2takey/ffmpeg-go"
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
	
	go func() {
		defer close(encodeErr)
		c.encodeVideo(ctx, frameChan, outputChan, stopChan, encodeErr)
	}()

	go func() {
		select {
		case err := <- captureErr:
			if err != nil {
				logger.Error("Capture error", "error", err)
			}
		case err := <- encodeErr:
			if err != nil {
				logger.Error("Encoding error", "error", err)
			}
		case <- stopChan:
			return
		}
	}()

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

func (c *Config) captureScreenshots(ctx context.Context, frameChan chan []byte, stop chan struct{}, errChan chan<- error) {
	logger := logging.FromContext(ctx)
	bounds := c.Bounds
	ticker := time.NewTicker(time.Second / time.Duration(c.CaptureFrameRate)) 
	defer ticker.Stop()
	defer close(frameChan)
	frameCount := 0
	for {
		select {
		case <-stop:
			logger.Info("Stopping capture")
			return
		case <-ticker.C:
			frameCount++
			img, err := screenshot.CaptureRect(bounds)
			if err != nil {
				errChan <- err
				return
			}
			select {
			case frameChan <- img.Pix:
				logger.Info("Capturing Frame", "number", frameCount, "location", "captureScreenshots")
				// we have nothing celebrate
			default:
				select{
				case <-frameChan:
					select{
					case frameChan <- img.Pix:
						logger.Info("Capturing Frame", "number", frameCount, "location", "captureScreenshots")
						// Success
					default:
					}
					// drained old frame
				default:
				}
			}

		}
	}

}

func (c *Config) encodeVideo(ctx context.Context, input <-chan []byte, output chan<- []byte, stopChan chan struct{}, errChan chan<- error){
	defer close(output)
	logger := logging.FromContext(ctx)

	var wg sync.WaitGroup

	pr, pw := io.Pipe()
	ffmpegOut, ffmpegIn := io.Pipe()

	wg.Add(1)
	go func()  {
		defer wg.Done()
		defer ffmpegOut.Close()
		err := ffmpeg.Input("pipe:", 
			ffmpeg.KwArgs{
			"format": "rawvideo",
			"pix_fmt": "rgba",
			"s": fmt.Sprintf("%dx%d", c.Bounds.Dx(), c.Bounds.Dy()),
			"r": fmt.Sprintf("%d", c.EncodeFrameRate),
			}).
			Output("pipe:", ffmpeg.KwArgs{
				"c:v": "libvpx",
				"f": "ivf",
				"fflags": "+flush_packets",
				"g": "1",
				"auto-alt-ref": "0",
				"cpu-used": "16",
				"deadline": "realtime",
			}).
			WithInput(pr).
			WithOutput(ffmpegIn).
			// WithErrorOutput(os.Stdout).
			Silent(true).
			Run()
		if err != nil {
			logger.Error("Error encoding video 2:", "error", err)
			errChan <- err
			return
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer pw.Close()

		count := 0
		for {
			select{
			case <-stopChan:
				return

			case frame, ok := <-input :
				count += 1
				if !ok {
					return
				}
				logger.Info("investigation:", "frame", count, "size", len(frame), slog.String("first 12 bytes", fmt.Sprintf("%v", frame[:12])))
				n, err := pw.Write(frame)
				logger.Info("investigation: wrote", "bytes", n, "err", err)
				
			}
		}
	}()

	c.parseIVFStream(ctx, ffmpegOut, output, stopChan)
	wg.Wait()
}

func (c *Config) parseIVFStream(ctx context.Context, reader io.Reader, output chan<- []byte, stopChan <-chan struct{}) {
	logger := logging.FromContext(ctx)
	parser := NewIVFParser(16)
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
