package main

import (
	"context"
	"fmt"
	"time"

	"github.com/dylanmccormick/webrtc-streamer/internal/logging"
	"github.com/kbinani/screenshot"
)

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
				logger.Info("Screenshot data", "data", fmt.Sprintf("%v", img.Pix[:10]))
				// logger.Info("Capturing Frame", "number", frameCount, "location", "captureScreenshots")
				// we have nothing celebrate
			default:
				select {
				case <-frameChan:
					select {
					case frameChan <- img.Pix:
						// logger.Info("Capturing Frame", "number", frameCount, "location", "captureScreenshots")
						logger.Info("Screenshot data", "data", fmt.Sprintf("%v", img.Pix[:10]))
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
