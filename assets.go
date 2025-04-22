package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"crypto/rand"
	"encoding/base64"
	"os/exec"
	"bytes"
	"encoding/json"
	"errors"
)

func (cfg apiConfig) ensureAssetsDir() error {
	if _, err := os.Stat(cfg.assetsRoot); os.IsNotExist(err) {
		return os.Mkdir(cfg.assetsRoot, 0755)
	}
	return nil
}

func getAssetPath(mediaType string) string {
	base := make([]byte, 32)
	_, err := rand.Read(base)
	if err != nil {
		panic("Failed to generate random byte")
	}
	id := base64.RawURLEncoding.EncodeToString(base)

	ext := mediaTypeToExt(mediaType)
	return fmt.Sprintf("%s%s", id, ext)
}

func (cfg apiConfig) getObjectURL(key string) string {
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, key)
}

// store bucket and key as a comma delimited string
func (cfg apiConfig) getVideoURL(fileKey string) string {
	return fmt.Sprintf("%s,%s", cfg.s3Bucket, fileKey)
}

func (cfg apiConfig) getAssetDiskPath(assetPath string) string {
	return filepath.Join(cfg.assetsRoot, assetPath)
}

func (cfg apiConfig) getAssetURL(assetPath string) string {
	return fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, assetPath)
}

func mediaTypeToExt(mediaType string) string {
	parts := strings.Split(mediaType, "/")
	if len(parts) != 2 {
		return ".bin"
	}
	return "." + parts[1]
}

// Take a file path and return the aspect ratio as a string
func getVideoAspectRatio(filePath string) (string, error) {
	type ffprobeOutput struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"streams"`
	}
	cmd := exec.Command("ffprobe", 
		"-v", "error", 
		"-print_format", 
		"json", "-show_streams", 
		filePath,
	)

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", err
	}

	var probe ffprobeOutput
	decoder := json.NewDecoder(&out)
	if err := decoder.Decode(&probe); err != nil {
		return "", err
	}

	// Determine ratio
	if len(probe.Streams) == 0 || probe.Streams[0].Width == 0 || probe.Streams[0].Height == 0 {
		return "", errors.New("could not find valid video stream")
	}

	w := probe.Streams[0].Width
	h := probe.Streams[0].Height

	ratio := float64(w) / float64(h)
	switch {
		case approx(ratio, 16.0 / 9.0):
			return "landscape", nil
		case approx(ratio, 9.0 / 16.0):
			return "portrait", nil
		default:
			return "other", nil
	}
}

// takes a file path as input and creates and returns a new path to a file with "fast start" encoding
func processVideoForFastStart(filePath string) (string, error) {
	outFilePath := filePath + ".processing"

	// This cmd will optimize a mp4 file to support fast start
	cmd := exec.Command("ffmpeg",
		"-i", filePath,
		"-c", "copy",
		"-movflags", "faststart",
		"-f", "mp4",
		outFilePath,
	)
	if err := cmd.Run(); err != nil {
		return "", err
	}

	return outFilePath, nil
}
