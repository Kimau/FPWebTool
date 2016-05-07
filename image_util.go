package main

import (
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
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
