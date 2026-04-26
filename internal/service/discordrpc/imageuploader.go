// Package discordrpc handles the uploading of raw cover art images to a
// temporary image hosting API so they can be consumed by the Discord RPC client.
//
// Key Components:
//   - ImageUploader: Wrapper struct holding http.Client and caching the last uploaded URL
//   - UploadImage(): Encodes PNG bytes and posts them as multipart form data
//   - GetImageURL(): Caches upload results to prevent redundant network requests
//
// Dependencies:
//   - net/http: Executing multipart POST requests to the temporary host
//   - image/png: Encoding the raw image.Image to PNG bytes
//   - encoding/json: Parsing the temporary host's API response
//
// Error Types:
//   - Errors are returned if the image encoding, HTTP request, or JSON parsing fails.
//
// Example:
//   uploader := discordrpc.NewImageUploader()
//   url, err := uploader.GetImageURL(img)
//   if err != nil { return err }
package discordrpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
)

const uploadAPI = "https://tmpfiles.org/api/v1/upload"

type apiResponse struct {
	Status string `json:"status"`
	Data   struct {
		URL string `json:"url"`
	} `json:"data"`
}

type ImageUploader struct {
	client  *http.Client
	lastImg image.Image
	lastURL string
}

func NewImageUploader() *ImageUploader {
	return &ImageUploader{
		client: &http.Client{},
	}
}

func (u *ImageUploader) UploadImage(img image.Image) (string, error) {
	b, err := encodePNG(img)
	if err != nil {
		return "", fmt.Errorf("encode image: %w", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "image.png")
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}

	if _, err = part.Write(b); err != nil {
		return "", fmt.Errorf("write image bytes: %w", err)
	}
	writer.Close()

	req, err := http.NewRequest(http.MethodPost, uploadAPI, body)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := u.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var parsed apiResponse
	if err = json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	url := strings.Replace(parsed.Data.URL, "org/", "org/dl/", 1)

	// Discord requires HTTPS for external image URLs
	url = strings.Replace(url, "http://", "https://", 1)

	return url, nil
}

func (u *ImageUploader) GetImageURL(img image.Image) (string, error) {
	if img == u.lastImg && u.lastURL != "" {
		return u.lastURL, nil
	}

	url, err := u.UploadImage(img)
	if err != nil {
		return "", err
	}

	u.lastImg = img
	u.lastURL = url
	return url, nil
}

func encodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
