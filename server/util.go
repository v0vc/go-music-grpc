package main

import (
	"bytes"
	"html"
	"html/template"
	"math/rand"
	"os"
	"regexp"
	"sort"
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

func Diff(a, b []string) []string {
	a = sortIfNeeded(a)
	b = sortIfNeeded(b)
	var d []string
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		c := strings.Compare(a[i], b[j])
		if c == 0 {
			i++
			j++
		} else if c < 0 {
			d = append(d, a[i])
			i++
		} else {
			d = append(d, b[j])
			j++
		}
	}
	d = append(d, a[i:]...)
	d = append(d, b[j:]...)
	return d
}

func sortIfNeeded(a []string) []string {
	if sort.StringsAreSorted(a) {
		return a
	}
	s := append(a[:0:0], a...)
	sort.Strings(s)
	return s
}

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
