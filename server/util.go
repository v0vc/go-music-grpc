package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"html"
	"html/template"
	"math"
	"math/big"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"
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

/*func FirstOrDefault[T any](slice []T, filter func(*T) bool) (element *T) {
	for i := 0; i < len(slice); i++ {
		if filter(&slice[i]) {
			return &slice[i]
		}
	}

	return nil
}*/

func Contains[T comparable](s []T, e T) bool {
	for _, v := range s {
		if v == e {
			return true
		}
	}
	return false
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

func RandomPause(minPause, duration int64) {
	nBig, _ := rand.Int(rand.Reader, big.NewInt(duration))
	time.Sleep(time.Duration(minPause+nBig.Int64()) * time.Second)
}

func GenerateRandomStr(l int) string {
	buff := make([]byte, int(math.Ceil(float64(l)/1.33333333333)))
	_, err := rand.Read(buff)
	if err != nil {
		return "tmp"
	}
	str := base64.RawURLEncoding.EncodeToString(buff)
	return str[:l] // strip 1 extra character we get from odd length results
}
