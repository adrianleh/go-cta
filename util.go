package cta

import (
	"fmt"
	"image/color"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func redactApiKey(input string) string {
	re := regexp.MustCompile(`(key=)([^&]+)`)
	return re.ReplaceAllString(input, `${1}[REDACTED]`)
}

func hexToRGBA(h string) (color.RGBA, error) {
	if h[0] == '#' {
		h = h[1:]
	}
	if len(h) != 6 {
		return color.RGBA{}, fmt.Errorf("invalid hex color length: %d", len(h))
	}
	r, err := strconv.ParseUint(h[0:2], 16, 8)
	if err != nil {
		return color.RGBA{}, err
	}
	g, err := strconv.ParseUint(h[2:4], 16, 8)
	if err != nil {
		return color.RGBA{}, err
	}
	b, err := strconv.ParseUint(h[4:6], 16, 8)
	if err != nil {
		return color.RGBA{}, err
	}
	return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}, nil
}

// Basic functional helpers (sad Go doesn't have them built-in)

func transform[T, U any](ts []T, f func(T) U) []U {
	us := make([]U, len(ts))
	for i := range ts {
		us[i] = f(ts[i])
	}
	return us
}

func filter[T any](ts []T, f func(T) bool) []T {
	us := make([]T, 0)
	for _, t := range ts {
		if f(t) {
			us = append(us, t)
		}
	}
	return us
}

func chunk[T any](ts []T, size int) [][]T {
	var chunks [][]T
	for size < len(ts) {
		ts, chunks = ts[size:], append(chunks, ts[0:size:size])
	}
	chunks = append(chunks, ts)
	return chunks
}

func parseCtaDetourTime(timeStr string) (time.Time, error) {
	const layout = "20060102 15:04"
	loc, err := time.LoadLocation("America/Chicago")
	errorTime := time.Unix(0, 0)
	if err != nil {
		return errorTime, fmt.Errorf("loading location: %w", err)
	}
	s := strings.TrimSpace(timeStr)
	if s == "" {
		return errorTime, fmt.Errorf("empty start datetime")
	}
	t, err := time.ParseInLocation(layout, s, loc)
	if err != nil {
		return errorTime, fmt.Errorf("parsing start datetime %q: %w", s, err)
	}
	return t, nil
}

func unixTimeToTimeUnknownGranularity(unixTime int64) time.Time {
	// In testing I found different endpoints return unix time in different granularities
	// Since it is inconsistent, I don't want to assume consistency from run to run
	// So here we try to guess the granularity based on the size of the number
	// Not perfect but good enough for this use case
	if unixTime > 1e17 { // likely nanoseconds
		secs := unixTime / 1_000_000_000
		nsecs := unixTime % 1_000_000_000
		return time.Unix(secs, nsecs)
	}
	if unixTime > 1e14 { // likely microseconds
		secs := unixTime / 1_000_000
		nsecs := (unixTime % 1_000_000) * 1_000
		return time.Unix(secs, nsecs)
	}
	if unixTime > 1e11 { // likely milliseconds
		secs := unixTime / 1_000
		nsecs := (unixTime % 1_000) * 1_000_000
		return time.Unix(secs, nsecs)
	}
	return time.Unix(unixTime, 0)
}
