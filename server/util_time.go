package main

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	Day   = time.Hour * 24
	Month = Day * 30
	Year  = Day * 365
)

type FormatPeriod struct {
	D    time.Duration
	One  string
	Many string
}

var singleInstance *Config

var lock = &sync.Mutex{}

type Config struct {
	PastPrefix   string
	PastSuffix   string
	FuturePrefix string
	FutureSuffix string
	Periods      []FormatPeriod
	Zero         string
}

func TimeAgo(t time.Time) string {
	if singleInstance == nil {
		lock.Lock()
		defer lock.Unlock()
		singleInstance = &Config{
			PastPrefix:   "",
			PastSuffix:   " ago",
			FuturePrefix: "in ",
			FutureSuffix: "",

			Periods: []FormatPeriod{
				{time.Second, "about a second", "%d seconds"},
				{time.Minute, "about a minute", "%d minutes"},
				{time.Hour, "about an hour", "%d hours"},
				{Day, "one day", "%d days"},
				{Month, "one month", "%d months"},
				{Year, "one year", "%d years"},
			},
			Zero: "about a second",
		}
	}
	return singleInstance.FormatReference(t, time.Now())
}

func (cfg Config) FormatReference(t time.Time, reference time.Time) string {
	d := reference.Sub(t)
	return cfg.FormatRelativeDuration(d)
}

func (cfg Config) FormatRelativeDuration(d time.Duration) string {
	isPast := d >= 0
	if d < 0 {
		d = -d
	}
	s, _ := cfg.getTimeText(d, true)
	if isPast {
		return strings.Join([]string{cfg.PastPrefix, s, cfg.PastSuffix}, "")
	} else {
		return strings.Join([]string{cfg.FuturePrefix, s, cfg.FutureSuffix}, "")
	}
}

func round(d time.Duration, step time.Duration, roundCloser bool) time.Duration {
	if roundCloser {
		return time.Duration(float64(d)/float64(step) + 0.5)
	}
	return time.Duration(float64(d) / float64(step))
}

func nbParamInFormat(f string) int {
	return strings.Count(f, "%") - 2*strings.Count(f, "%%")
}

func (cfg Config) getTimeText(d time.Duration, roundCloser bool) (string, time.Duration) {
	if len(cfg.Periods) == 0 || d < cfg.Periods[0].D {
		return cfg.Zero, 0
	}
	for i, p := range cfg.Periods {
		next := p.D
		if i+1 < len(cfg.Periods) {
			next = cfg.Periods[i+1].D
		}
		if i+1 == len(cfg.Periods) || d < next {
			r := round(d, p.D, roundCloser)
			if next != p.D && r == round(next, p.D, roundCloser) {
				continue
			}

			if r == 0 {
				return "", d
			}
			layout := p.Many
			if r == 1 {
				layout = p.One
			}

			if nbParamInFormat(layout) == 0 {
				return layout, d - r*p.D
			}
			return fmt.Sprintf(layout, r), d - r*p.D
		}
	}

	return d.String(), 0
}
