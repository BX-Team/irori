package daemon

import (
	"regexp"
	"strings"

	"github.com/bx-team/irori/internal/models"
)

var (
	levelRe  = regexp.MustCompile(`\[(?:[^\]/]*/)?(INFO|WARN|WARNING|ERROR|SEVERE|FATAL|DEBUG|TRACE)\]`)
	chatRe   = regexp.MustCompile(`\]:\s*<[^>]+>`)
	joinRe   = regexp.MustCompile(`\]:\s*([A-Za-z0-9_]{1,16}) joined the game`)
	leaveRe  = regexp.MustCompile(`\]:\s*([A-Za-z0-9_]{1,16}) left the game`)
	readyRe  = regexp.MustCompile(`\]:\s*Done \([0-9.]+s\)!`)
	proxyUp  = regexp.MustCompile(`Done \([0-9.]+s\)!|Listening on /`)
	stopping = regexp.MustCompile(`\]:\s*(Stopping the server|Stopping server|Closing Server)`)
)

func detectLevel(text string) models.LogLevel {
	if m := levelRe.FindStringSubmatch(text); m != nil {
		switch m[1] {
		case "WARN", "WARNING":
			return models.LevelWarn
		case "ERROR", "SEVERE", "FATAL":
			return models.LevelError
		case "DEBUG", "TRACE":
			return models.LevelDebug
		}
		if chatRe.MatchString(text) {
			return models.LevelChat
		}
		return models.LevelInfo
	}
	upper := strings.ToUpper(text)
	switch {
	case strings.Contains(upper, "EXCEPTION") || strings.Contains(upper, "ERROR"):
		return models.LevelError
	case strings.Contains(upper, "WARN"):
		return models.LevelWarn
	case strings.HasPrefix(text, "\tat "):
		return models.LevelError
	}
	return models.LevelInfo
}

type consoleEvent struct {
	ready    bool
	stopping bool
	joined   string
	left     string
}

func detectEvent(text string, proxy bool) consoleEvent {
	var e consoleEvent
	if readyRe.MatchString(text) || (proxy && proxyUp.MatchString(text)) {
		e.ready = true
	}
	if stopping.MatchString(text) {
		e.stopping = true
	}
	if m := joinRe.FindStringSubmatch(text); m != nil {
		e.joined = m[1]
	}
	if m := leaveRe.FindStringSubmatch(text); m != nil {
		e.left = m[1]
	}
	return e
}
