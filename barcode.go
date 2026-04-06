package main

import (
	"image/png"
	// "log"
	"os"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/oned"
)

func createBarCode() (*gozxing.BitMatrix, error) {
	enc := oned.NewCode128Writer()

	img, err := enc.Encode(
		"Hello, World!",
		gozxing.BarcodeFormat_CODE_128,
		250,
		50,
		nil,
	)
	if err != nil {
		return nil, err
	}

	return img, nil
}

func createImage(filename string, img *gozxing.BitMatrix) error {
	file, err := os.Create(filename + ".png")
	if err != nil {
		return err
	}
	defer file.Close()

	return png.Encode(file, img)
}
//
// func main() {
// 	img, err := createBarCode()
// 	if err != nil {
// 		log.Fatal(err)
// 	}
//
// 	if err := createImage("barcode", img); err != nil {
// 		log.Fatal(err)
// 	}
// }
