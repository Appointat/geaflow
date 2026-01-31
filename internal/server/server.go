package server

import (
	"log"
	"net/http"

	"wsproxy/internal/config"
	"wsproxy/internal/imagegen"
	"wsproxy/internal/pool"
	"wsproxy/internal/proxy"
	"wsproxy/internal/websocket"
)

// Run starts the WebSocket proxy server
func Run() error {
	// Initialize config (debug mode, etc.)
	config.Init()

	// Initialize connection pool
	connPool := pool.NewConnectionPool()

	// Register WebSocket route
	http.HandleFunc(config.WSPath, websocket.HandleWebSocket(connPool))

	// Register image generation endpoint
	http.HandleFunc("/v1/images/generations", imagegen.HandleImageGeneration(connPool))

	// Register HTTP proxy route (catch-all)
	http.HandleFunc("/", proxy.HandleProxyRequest(connPool))

	log.Printf("Starting server on %s", config.ProxyListenAddr)
	log.Printf("WebSocket endpoint available at ws://%s%s", config.ProxyListenAddr, config.WSPath)
	log.Printf("HTTP proxy available at http://%s/", config.ProxyListenAddr)

	return http.ListenAndServe(config.ProxyListenAddr, nil)
}
