package utils

import (
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// ParseDeviceInfo returns a human-readable browser + OS string from a User-Agent header.
// Uses simple keyword scanning — no external dependency required.
func ParseDeviceInfo(ua string) string {
	if ua == "" {
		return "Unknown Device"
	}
	lower := strings.ToLower(ua)

	browser := "Unknown Browser"
	switch {
	case strings.Contains(lower, "edg/"):
		browser = "Edge"
	case strings.Contains(lower, "chrome") && !strings.Contains(lower, "chromium"):
		browser = "Chrome"
	case strings.Contains(lower, "firefox"):
		browser = "Firefox"
	case strings.Contains(lower, "safari") && !strings.Contains(lower, "chrome"):
		browser = "Safari"
	case strings.Contains(lower, "opr") || strings.Contains(lower, "opera"):
		browser = "Opera"
	}

	os := "Unknown OS"
	switch {
	case strings.Contains(lower, "iphone"):
		os = "iPhone"
	case strings.Contains(lower, "ipad"):
		os = "iPad"
	case strings.Contains(lower, "android"):
		os = "Android"
	case strings.Contains(lower, "windows"):
		os = "Windows"
	case strings.Contains(lower, "macintosh") || strings.Contains(lower, "mac os"):
		os = "macOS"
	case strings.Contains(lower, "linux"):
		os = "Linux"
	}

	return browser + " on " + os
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPassword(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

func GetTokenFromHeader(authHeader string) string {
	if authHeader == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return ""
	}
	return parts[1]
}
