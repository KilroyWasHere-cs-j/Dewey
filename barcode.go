package main

/*
	Barcode library used: github.com/makiuchi-d/gozxing
*/
import (
	"errors"
	"image"
	"image/png"
	"os"
	"fmt"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/oned"
	"github.com/makiuchi-d/gozxing/qrcode"
)

/*
	Creates a barcode using the Code128 format
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

func scanBarCode(path string) {
	// open and decode image file
	file, err := os.Open(path)
	if err != nil {
		Warn(err.Error())
		return
	}
	img, _, err := image.Decode(file)
	if err != nil {
		Warn(err.Error())
		return
	}

	// prepare BinaryBitmap
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		Warn(err.Error())
		return
	}

	// decode image
	qrReader := qrcode.NewQRCodeReader()
	result, _ := qrReader.Decode(bmp, nil)

	fmt.Println(result)
}

// Below is some GPTChat code for handling PDFs still need to test
// doc, _ := fitz.New("file.pdf")
// defer doc.Close()
//
// for n := 0; n < doc.NumPage(); n++ {
//     img, _ := doc.Image(n)
//
//     bmp, _ := gozxing.NewBinaryBitmapFromImage(img)
//     result, _ := qrReader.Decode(bmp, nil)
//
//     if result != nil {
//         fmt.Println(result.String())
//     }
// }
