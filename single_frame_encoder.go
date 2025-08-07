package main

import (
	"bytes"
	"fmt"
	"image"
	"io/ioutil"

	"github.com/kbinani/screenshot"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

func main2() {

	for range 15 {
		img, _ := screenshot.CaptureRect(image.Rect(0, 0, 640, 480))
		encoded, err := encodeFrame(img.Pix, 640, 480)
		if err != nil {
			fmt.Printf("Error: %v", err)
		} else {
			ioutil.WriteFile("test.ivf", encoded, 0644)

			ivfData, err := getFrameFromIVF(encoded)
			if err != nil {
				fmt.Printf("Error: %v", err)
			}
			fmt.Printf("VP8 data length: %d bytes\n", len(ivfData))
		}
	}

}

func encodeFrame(rawFrame []byte, width, height int) ([]byte, error) {
	var buf bytes.Buffer

	err := ffmpeg.Input("pipe:",
		ffmpeg.KwArgs{
			"format":  "rawvideo",
			"pix_fmt": "rgba",
			"s":       fmt.Sprintf("%dx%d", width, height),
		}).
		Output("pipe:", ffmpeg.KwArgs{
			"c:v":          "libvpx",
			"f":            "ivf",
			"frames:v":     "1",
			"auto-alt-ref": "0",
			"loglevel":     "quiet",
		}).
		WithInput(bytes.NewReader(rawFrame)).
		WithOutput(&buf).
		Silent(true).
		Run()

	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// func getFrameFromIVF(ivfData []byte) ([]byte, error) {
// 	frameSize := binary.LittleEndian.Uint32(ivfData[32:36])
// 	return ivfData[44:44+frameSize], nil
// }
