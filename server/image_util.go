package main

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"log"

	"github.com/disintegration/imaging"
)

func PrepareThumb(imgByte []byte, borderWidth int, length int, width int, jpegQuality int) []byte {
	img, _, err := image.Decode(bytes.NewReader(imgByte))
	if err != nil {
		log.Println(err)
	}

	borderColor := color.RGBA{R: 0, A: 0}
	borderRect := image.Rect(borderWidth, 0, img.Bounds().Dx()+borderWidth, img.Bounds().Dy()+borderWidth*2)

	borderImg := image.NewRGBA(borderRect)
	draw.Draw(borderImg, borderImg.Bounds(), &image.Uniform{C: borderColor}, image.Point{}, draw.Src)
	draw.Draw(borderImg, img.Bounds().Add(image.Point{X: borderWidth, Y: borderWidth}), img, image.Point{}, draw.Src)

	dstImage := imaging.Resize(borderImg, length, width, imaging.NearestNeighbor)

	buff := bytes.Buffer{}
	err = jpeg.Encode(&buff, dstImage, &jpeg.Options{Quality: jpegQuality})
	if err != nil {
		log.Println(err)
	}
	return buff.Bytes()
}
