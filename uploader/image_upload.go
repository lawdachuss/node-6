package uploader

import (
	"fmt"
	"strings"
	"time"
)

type imageHost struct {
	name   string
	upload func(string) (string, error)
}

// MultiImageUploader uploads thumbnails/sprites with durable fallbacks:
// Freeimage → Catbox → Pixhost (NSFW fallback).
type MultiImageUploader struct {
	hosts []imageHost
}

// NewMultiImageUploader creates the default thumbnail upload chain.
func NewMultiImageUploader() *MultiImageUploader {
	freeimage := NewFreeimageUploader()
	catbox := NewCatboxUploader()
	pixhost := NewThumbnailUploader("")

	hosts := []imageHost{
		{name: "Freeimage", upload: freeimage.Upload},
		{name: "Catbox", upload: catbox.Upload},
		{name: "Pixhost", upload: pixhost.Upload},
	}

	return &MultiImageUploader{hosts: hosts}
}

const (
	imageUploadRetries    = 2
	imageUploadBaseDelay  = 2 * time.Second
)

// Upload tries each host in order until one succeeds.
// Retries the entire fallback chain up to imageUploadRetries times.
func (m *MultiImageUploader) Upload(filePath string) (url, host string, err error) {
	var lastErrors []string
	for attempt := 0; attempt <= imageUploadRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(imageUploadBaseDelay * time.Duration(1<<(attempt-1)))
		}
		for _, h := range m.hosts {
			var upErr error
			url, upErr = h.upload(filePath)
			if upErr == nil {
				return url, h.name, nil
			}
			lastErrors = append(lastErrors, fmt.Sprintf("%s: %v", h.name, upErr))
		}
	}
	return "", "", fmt.Errorf("all image hosts failed after %d attempts: [%s]", imageUploadRetries+1, strings.Join(lastErrors, "; "))
}
