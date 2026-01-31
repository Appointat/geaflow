package config

import (
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// --- Constants ---
const (
	WSPath              = "/v1/ws"
	ProxyListenAddr     = ":5345"
	WSReadTimeout       = 60 * time.Second
	ProxyRequestTimeout = 600 * time.Second
	HTTPProxyAddr       = "http://127.0.0.1:7890"
	MaxFileSize         = 50 * 1024 * 1024 // 50MB
)

// ProxyClient is the HTTP client configured with proxy for fetching images/files
var ProxyClient *http.Client

// DebugMode controls whether debug logging is enabled
var DebugMode bool

// Init initializes the configuration, including the HTTP proxy client and debug mode
func Init() {
	// Initialize proxy client
	proxyURL, _ := url.Parse(HTTPProxyAddr)
	ProxyClient = &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
		Timeout: 30 * time.Second,
	}

	// Initialize debug mode from environment variable
	debugEnv := os.Getenv("DEBUG")
	if debugEnv != "" {
		debugEnv = strings.ToLower(strings.TrimSpace(debugEnv))
		DebugMode = debugEnv == "true" || debugEnv == "1" || debugEnv == "yes"
	}
}

// SetDebugMode enables or disables debug logging
func SetDebugMode(enabled bool) {
	DebugMode = enabled
}

// IsDebugMode returns whether debug mode is currently enabled
func IsDebugMode() bool {
	return DebugMode
}
