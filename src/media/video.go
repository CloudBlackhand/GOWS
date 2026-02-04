package media

import (
	"bytes"
	"fmt"
	"github.com/u2takey/ffmpeg-go"
	"io"
	"os"
)

// VideoThumbnail generates a thumbnail image from a video at a specific frame.
// Uses temporary file to avoid keeping entire video in RAM during processing.
func VideoThumbnail(content []byte, frameNum int, size struct{ Width int }) ([]byte, error) {
	// Create temporary file to avoid keeping video in RAM
	tmpFile, err := os.CreateTemp("", "gows-video-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath) // Clean up temp file

	// Write video content to temp file
	_, err = tmpFile.Write(content)
	if err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("failed to write video to temp file: %v", err)
	}
	tmpFile.Close()

	// Create pipe for output
	outputReader, outputWriter := io.Pipe()

	// Run ffmpeg process reading from temp file instead of pipe
	go func() {
		defer outputWriter.Close()
		cmd := ffmpeg_go.Input(tmpPath).
			Filter("scale", ffmpeg_go.Args{fmt.Sprintf("%d:-1", size.Width)}).
			Filter("select", ffmpeg_go.Args{fmt.Sprintf("gte(n,%d)", frameNum)}).
			Output("pipe:", ffmpeg_go.KwArgs{"vframes": 1, "format": "image2"}).
			WithOutput(outputWriter).
			WithErrorOutput(os.Stderr).
			OverWriteOutput()
		err := cmd.Run()
		if err != nil {
			outputWriter.CloseWithError(err)
		}
	}()

	// Read the output into a buffer
	var buf bytes.Buffer
	_, err = buf.ReadFrom(outputReader)
	if err != nil {
		return nil, err
	}

	data := buf.Bytes()
	if len(data) == 0 {
		return nil, fmt.Errorf("no thumbnail data returned")
	}
	return buf.Bytes(), nil
}
