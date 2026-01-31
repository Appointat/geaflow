package main

import (
	"flag"
	"log"

	"wsproxy/internal/config"
	"wsproxy/internal/server"
)

func main() {
	// Command-line flag for debug mode
	debugFlag := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	// Initialize configuration (reads DEBUG env var)
	config.Init()

	// Allow command-line flag to override environment variable
	if *debugFlag {
		config.SetDebugMode(true)
	}

	if config.IsDebugMode() {
		log.Println("DEBUG mode enabled.")
	}

	if err := server.Run(); err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}
