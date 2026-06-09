package main

/*
	Barcode library used: github.com/makiuchi-d/gozxing
*/
import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"os"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/oned"
	// "github.com/makiuchi-d/gozxing/multi"
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
	if width+10 <= height {
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

func toHighContrast(src image.Image) *image.Gray {
	gray := image.NewGray(src.Bounds())
	draw.Draw(gray, gray.Bounds(), src, src.Bounds().Min, draw.Src)

	// Apply threshold: anything below 180 becomes black, else white
	// This handles the navy-blue bars which land around 80-120 in gray
	b := gray.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := gray.GrayAt(x, y)
			if c.Y < 180 {
				gray.SetGray(x, y, color.Gray{Y: 0})
			} else {
				gray.SetGray(x, y, color.Gray{Y: 255})
			}
		}
	}
	return gray
}

func scanBarCode(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return "", fmt.Errorf("decode image: %w", err)
	}

	// Apply high-contrast threshold to handle colored/gray bars
	gray := toHighContrast(img)

	b := gray.Bounds()
	hints := map[gozxing.DecodeHintType]interface{}{
		gozxing.DecodeHintType_TRY_HARDER: true,
	}
	reader := oned.NewCode128Reader()

	for i := 0; i <= 18; i++ {
		top := b.Dy() * (i * 5) / 100
		bot := b.Dy() * (i*5 + 15) / 100
		slice := gray.SubImage(image.Rect(0, top, b.Dx(), bot))

		bmp, err := gozxing.NewBinaryBitmapFromImage(slice)
		if err != nil {
			continue
		}
		result, err := reader.Decode(bmp, hints)
		if err == nil {
			return result.GetText(), nil
		}
	}

	return "", fmt.Errorf("no barcode found in %s", path)
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

//
// Threshold value (180) — lower it if you get false positives, raise it if dark bars are getting missed
// Slice size/step (15% height, 5% increments) — tighten the increments if barcodes are very thin relative to page height
