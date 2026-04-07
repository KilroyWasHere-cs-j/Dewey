package main

import (
	"errors"
	"image/png"
	"os"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/oned"
)

/*
	Creates a barcode
	# Arguments
	- text (string)
	- width (int)
	- height (int)
	# Returns 
	- barcode (gozxing.BitMatrix, error)
*/
func createBarCode(text string, width int, height int) (*gozxing.BitMatrix, error) {
	enc := oned.NewCode128Writer()

	// Ensure the barcode will be longer(wider) than it is tall(height), adding in 10 as a "square prevention buffer"
	if width + 10 <= height {
		Warn("Barcode dimensions miss-match")
		return nil, errors.New("Dimensions miss-match")
	}

	img, err := enc.Encode(
		text,
		gozxing.BarcodeFormat_CODE_128,
		width,
		height,
		nil,
	)

	if err != nil {
		Warn(err.Error())
		return nil, err
	}

	return img, nil
}

/*
	Creates a barcode image from a gozxing.BitMatrix
	# Arguments
	- filename (string)
	- img (gozxing.BitMatrix)
	# Returns
	- error
*/
func createImage(filename string, img *gozxing.BitMatrix) error {
	file, err := os.Create(filename + ".png")
	if err != nil {
		Warn(err.Error())
		return err
	}
	defer file.Close()

	return png.Encode(file, img)
}

