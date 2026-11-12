package viewer

import (
	"html"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	reScript = regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(script|style)>`)
	reBreak = regexp.MustCompile(`(?i)<br\s*/?>|</(p|div|li|h[1-6]|tr)>`)
	reTag = regexp.MustCompile(`(?s)<[^>]*>`)
	reBlank = regexp.MustCompile(`\n{3,}`)
)

// StripHTML converts html to plainText
func StripHTML(s string) string {
	s = reScript.ReplaceAllString(s, "")
	s = reBreak.ReplaceAllString(s, "\n")
	s = reTag.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = reBlank.ReplaceAllString(s, "\n\n")
	return s
}

// FileKind classifies a file by extension
func FileKind(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".avif":
		return "image"
	case ".mp4", ".webm", ".mov", ".m4v":
		return "video"	
	case ".mp3", ".ogg", ".wav", ".m4a", ".flac":
		return "audio"
	}
	return "other"
}

func short(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "..."
}

func day(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}
