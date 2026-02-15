package main

import (
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/url"
	"os"
	"strings"

	_ "golang.org/x/image/webp"
)

func getImageDimension(imagePath string) (width int, height int, err error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return -1, -1, err
	}
	defer file.Close()

	image, _, err := image.DecodeConfig(file)
	if err != nil {
		return -1, -1, err
	}
	return image.Width, image.Height, nil
}

// cleanImagePath normalises an image src from HTML into a
// consistent absolute-from-root path for the filesystem.
func cleanImagePath(src string) string {
	// URL-decode percent-encoded characters
	decoded, err := url.PathUnescape(src)
	if err != nil {
		decoded = src
	}

	// Strip leading ../ or ./ prefixes
	for strings.HasPrefix(decoded, "../") {
		decoded = decoded[3:]
	}
	for strings.HasPrefix(decoded, "./") {
		decoded = decoded[2:]
	}

	// Ensure leading slash
	if !strings.HasPrefix(decoded, "/") {
		decoded = "/" + decoded
	}

	return decoded
}
