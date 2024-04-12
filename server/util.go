package main

import (
	"bytes"
	"html"
	"html/template"
	"math/rand"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"
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
		return 0
	default:
		return 0
	}
}
