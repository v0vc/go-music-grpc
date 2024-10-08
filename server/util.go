package main

import (
	"bytes"
	"fmt"
	"html"
	"html/template"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"log"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/disintegration/imaging"
)

const (
	letterBytes   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
)

func FindDifference(a, b []string) []string {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}

	var diff []string

	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}

	return diff
}

func FileExists(path string) (bool, error) {
	f, err := os.Stat(path)
	if err == nil {
		return !f.IsDir(), nil
	} else if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func sanitize(filename string, isFolder bool) string {
	var regexStr string
	if isFolder {
		regexStr = `[:*?"><|]`
	} else {
		regexStr = `[\/:*?"><|]`
	}
	str := regexp.MustCompile(regexStr).ReplaceAllString(filename, "_")
	return strings.TrimRightFunc(str, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) })
}

func ParseTemplate(tags map[string]string, defTemplate string) string {
	var buffer bytes.Buffer

	for {
		err := template.Must(template.New("").Parse(defTemplate)).Execute(&buffer, tags)
		if err == nil {
			break
		}

		buffer.Reset()
	}
	resPath := html.UnescapeString(buffer.String())
	return sanitize(resPath, false)
}

func RandomPause(minPause, duration int) {
	time.Sleep(time.Duration(minPause+rand.Intn(duration)) * time.Second)
}

func RandStringBytesMask(n int) string {
	b := make([]byte, n)
	for i := 0; i < n; {
		if idx := int(rand.Int63() & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i++
		}
	}
	return string(b)
}

func MapReleaseType(r string) int32 {
	switch r {
	case "album":
		return 0
	case "single":
		return 1
	default:
		return 0
	}
}

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

func ConvertYoutubeDurationToSec(str string) string {
	n := len(str)
	ans := 0
	curr := 0

	for i := 0; i < n; i++ {
		if str[i] == 'P' || str[i] == 'T' {
			continue
		} else if str[i] == 'D' {
			ans += 86400 * curr
			curr = 0
		} else if str[i] == 'H' {
			ans += 3600 * curr
			curr = 0
		} else if str[i] == 'M' {
			ans += 60 * curr
			curr = 0
		} else if str[i] == 'S' {
			ans += curr
			curr = 0
		} else {
			digit, _ := strconv.Atoi(string(str[i]))
			curr = 10*curr + digit
		}
	}
	return toHumanTime(ans)
}

func toHumanTime(duration int) string {
	if duration == 0 {
		return "00:00"
	}
	days := duration / 86400
	if days >= 1 {
		hours := (duration - days*86400) / 3600
		minutes := (duration - (days*86400 + hours*3600)) / 60
		seconds := duration - (days*86400 + hours*3600 + minutes*60)
		return fmt.Sprintf("%02d:%02d:%02d:%02d", days, hours, minutes, seconds)
	} else {
		hours := duration / 3600
		if hours >= 1 {
			minutes := (duration - hours*3600) / 60
			return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, duration-hours*3600-minutes*60)
		} else {
			minutes := duration / 60
			if minutes >= 1 {
				return fmt.Sprintf("%02d:%02d", minutes, duration-minutes*60)
			} else {
				return fmt.Sprintf("%02d", duration)
			}
		}
	}
}
