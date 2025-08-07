package main

import (
	"context"
	"fmt"
	"image"
	"io"
	"log/slog"
	"os"
	"sync"

	"github.com/dylanmccormick/webrtc-streamer/internal/logging"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

type EncoderConfig struct {
	EncodeFrameRate int
	Bounds          image.Rectangle
}
type VideoEncoder struct {
	logger       *slog.Logger
	config       EncoderConfig
	input        <-chan []byte
	output       chan<- []byte
	stopChan     <-chan struct{}
	errChan      chan<- error
	context      context.Context
	inputWriter  *io.PipeWriter
	inputReader  *io.PipeReader
	ffmpegInput  *io.PipeWriter
	ffmpegOutput *io.PipeReader
}

func NewVideoEncoder(ctx context.Context, c *Config) *VideoEncoder {
	config := EncoderConfig{
		EncodeFrameRate: c.EncodeFrameRate,
		Bounds:          c.Bounds,
	}
	pr, pw := io.Pipe()
	ffmpegOut, ffmpegIn := io.Pipe()
	return &VideoEncoder{
		logger:       logging.FromContext(ctx),
		config:       config,
		context:      ctx,
		inputWriter:  pw,
		inputReader:  pr,
		ffmpegInput:  ffmpegIn,
		ffmpegOutput: ffmpegOut,
	}

}

func (v *VideoEncoder) StartStream(input <-chan []byte, output chan<- []byte, stop <-chan struct{}, err chan<- error) {
	v.logger.Info("Starting stream...")
	var wg sync.WaitGroup
	v.input = input
	v.output = output
	v.stopChan = stop
	v.errChan = err

	wg.Add(1)
	v.logger.Info("Executing startEncode", "location", "StartStream")
	go v.startEncode(&wg, v.ffmpegOutput, v.inputReader, v.ffmpegInput)
	wg.Add(1)
	v.logger.Info("Executing startWrite", "location", "StartStream")
	go v.startWrite(&wg, v.inputWriter)

	v.logger.Info("Executing outputToChan", "location", "StartStream")
	go v.outputToChan(&wg)
	wg.Wait()
}

func (v *VideoEncoder) startEncode(wg *sync.WaitGroup, ffmpegOut, pr *io.PipeReader, ffmpegIn *io.PipeWriter) {
	defer wg.Done()
	defer ffmpegOut.Close()
	v.logger.Debug("Starting encoding")
	err := ffmpeg.Input("pipe:",
		ffmpeg.KwArgs{
			"format":  "rawvideo",
			"pix_fmt": "rgba",
			"s":       fmt.Sprintf("%dx%d", v.config.Bounds.Dx(), v.config.Bounds.Dy()),
			"r":       fmt.Sprintf("%d", v.config.EncodeFrameRate),
		}).
		Output("pipe:", ffmpeg.KwArgs{
			"c:v":          "libvpx",
			"f":            "ivf",
			"fflags":       "+flush_packets",
			"g":            "1",
			"auto-alt-ref": "0",
			"cpu-used":     "16",
			"deadline":     "realtime",
		}).
		WithInput(pr).
		WithOutput(ffmpegIn).
		WithErrorOutput(os.Stdout).
		// Silent(true).
		Run()
	if err != nil {
		v.logger.Error("Error encoding video:", "error", err)
		v.errChan <- err
		return
	}

}

func (v *VideoEncoder) startWrite(wg *sync.WaitGroup, pw *io.PipeWriter) {
	defer wg.Done()
	defer pw.Close()

	count := 0
	for {
		select {
		case <-v.stopChan:
			return

		case frame, ok := <-v.input:
			count += 1
			v.logger.Debug("Attempting write", "frame", count, "frame_data", fmt.Sprintf("%v", frame[:10]))
			if !ok {
				return
			}
			n, err := pw.Write(frame)
			v.logger.Debug("Wrote to pipe", "bytes", n, "error", err)

		}
	}

}

func (v *VideoEncoder) outputToChan(wg *sync.WaitGroup) {
	defer wg.Done()
	buffer := make([]byte, 16384)
	for {
		select {
		case <-v.stopChan:
			return
		default:
			v.logger.Info("Writing to channel", "data", buffer[:10])
			n, err := v.ffmpegOutput.Read(buffer)
			if err != nil {
				v.logger.Error("Error reading ffmpegOutput", "location", "outputToChan", "error", err)
				return
			}
			if n > 0 {
				v.logger.Info("Loading data into channel", "data_len", n)
				v.output <- buffer[:n]
				// buffer = buffer[n:]
			} else {
				v.logger.Info("No data read into buffer")
			}

		}
	}
}
