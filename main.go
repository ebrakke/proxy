package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

func main() {
	// Handle config-based modes
	if len(os.Args) == 1 {
		// Auto forward mode
		runConfigForwardMode()
		return
	}

	if len(os.Args) == 2 && os.Args[1] == "-r" {
		// Auto reverse mode
		runConfigReverseMode()
		return
	}

	// Manual modes
	if len(os.Args) < 3 || len(os.Args) > 4 {
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  Auto forward mode: %s\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  Auto reverse mode: %s -r\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  Manual forward mode: %s [remote]:[port] [localPort]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  Manual reverse mode: %s -r [localPort] [externalPort]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s                                    # auto forward using .proxy.conf\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -r                                 # auto reverse using .proxy.conf\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s myserver.tailnet.ts.net:8080 3000  # manual forward\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -r 8080 8080                       # manual reverse\n", os.Args[0])
		os.Exit(1)
	}

	// Check for manual reverse mode
	if os.Args[1] == "-r" {
		if len(os.Args) != 4 {
			fmt.Fprintf(os.Stderr, "Manual reverse mode usage: %s -r [localPort] [externalPort]\n", os.Args[0])
			os.Exit(1)
		}
		runReverseMode(os.Args[2], os.Args[3])
		return
	}

	// Manual forward mode (original behavior)
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Manual forward mode usage: %s [remote]:[port] [localPort]\n", os.Args[0])
		os.Exit(1)
	}

	remoteAddr := os.Args[1]
	localPort := os.Args[2]

	// Test remote connectivity
	log.Printf("Testing connection to %s...", remoteAddr)
	conn, err := net.Dial("tcp", remoteAddr)
	if err != nil {
		log.Fatalf("Failed to connect to remote server %s: %v", remoteAddr, err)
	}
	conn.Close()
	log.Printf("Successfully connected to %s", remoteAddr)

	// Start local listener
	localAddr := "localhost:" + localPort
	listener, err := net.Listen("tcp", localAddr)
	if err != nil {
		log.Fatalf("Failed to start listener on %s: %v", localAddr, err)
	}
	defer listener.Close()

	log.Printf("TCP proxy started: %s -> %s", localAddr, remoteAddr)

	for {
		clientConn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		go handleConnection(clientConn, remoteAddr)
	}
}

func runReverseMode(localPort, externalPort string) {
	localAddr := "localhost:" + localPort
	externalAddr := "0.0.0.0:" + externalPort

	// Test local service connectivity
	log.Printf("Testing connection to local service %s...", localAddr)
	conn, err := net.Dial("tcp", localAddr)
	if err != nil {
		log.Fatalf("Failed to connect to local service %s: %v", localAddr, err)
	}
	conn.Close()
	log.Printf("Successfully connected to local service %s", localAddr)

	// Start external listener
	listener, err := net.Listen("tcp", externalAddr)
	if err != nil {
		log.Fatalf("Failed to start listener on %s: %v", externalAddr, err)
	}
	defer listener.Close()

	log.Printf("Reverse TCP proxy started: %s -> %s", externalAddr, localAddr)

	for {
		clientConn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		go handleConnection(clientConn, localAddr)
	}
}

func handleConnection(clientConn net.Conn, remoteAddr string) {
	defer clientConn.Close()

	log.Printf("New connection from %s", clientConn.RemoteAddr())

	// Connect to remote server
	remoteConn, err := net.Dial("tcp", remoteAddr)
	if err != nil {
		log.Printf("Failed to connect to remote server %s: %v", remoteAddr, err)
		return
	}
	defer remoteConn.Close()

	// Use WaitGroup to manage bidirectional copying
	var wg sync.WaitGroup
	wg.Add(2)

	// Copy data from client to remote
	go func() {
		defer wg.Done()
		_, err := io.Copy(remoteConn, clientConn)
		if err != nil {
			log.Printf("Error copying client->remote: %v", err)
		}
	}()

	// Copy data from remote to client
	go func() {
		defer wg.Done()
		_, err := io.Copy(clientConn, remoteConn)
		if err != nil {
			log.Printf("Error copying remote->client: %v", err)
		}
	}()

	wg.Wait()
	log.Printf("Connection closed from %s", clientConn.RemoteAddr())
}

type ProxyConfig struct {
	Port        string
	Description string
}

func findConfigFile() string {
	// Look for .proxy.conf in current directory and parent directories
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		configPath := filepath.Join(dir, ".proxy.conf")
		if _, err := os.Stat(configPath); err == nil {
			return configPath
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return ""
}

func parseConfigFile(filename string) ([]ProxyConfig, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var configs []ProxyConfig
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse port:description format
		parts := strings.SplitN(line, ":", 2)
		config := ProxyConfig{
			Port: parts[0],
		}
		
		if len(parts) > 1 {
			config.Description = parts[1]
		}

		// Validate port number
		if _, err := strconv.Atoi(config.Port); err != nil {
			log.Printf("Skipping invalid port: %s", config.Port)
			continue
		}

		configs = append(configs, config)
	}

	return configs, scanner.Err()
}

func runConfigForwardMode() {
	configFile := findConfigFile()
	if configFile == "" {
		log.Fatal("No .proxy.conf file found in current directory or parent directories")
	}

	configs, err := parseConfigFile(configFile)
	if err != nil {
		log.Fatalf("Failed to parse config file %s: %v", configFile, err)
	}

	if len(configs) == 0 {
		log.Fatal("No valid port configurations found in .proxy.conf")
	}

	log.Printf("Using config file: %s", configFile)

	// For forward mode, we need to determine the remote host
	// We'll use the hostname from the config file or ask for it
	remoteHost := getRemoteHost()
	if remoteHost == "" {
		log.Fatal("Could not determine remote host. Please specify manually.")
	}

	var wg sync.WaitGroup
	for _, config := range configs {
		wg.Add(1)
		go func(cfg ProxyConfig) {
			defer wg.Done()
			
			remoteAddr := remoteHost + ":" + cfg.Port
			localAddr := "localhost:" + cfg.Port
			
			desc := cfg.Description
			if desc == "" {
				desc = "port " + cfg.Port
			}
			
			log.Printf("Starting forward proxy for %s: %s -> %s", desc, localAddr, remoteAddr)
			
			// Test remote connectivity
			conn, err := net.Dial("tcp", remoteAddr)
			if err != nil {
				log.Printf("Failed to connect to %s (%s): %v", remoteAddr, desc, err)
				return
			}
			conn.Close()
			
			// Start listener
			listener, err := net.Listen("tcp", localAddr)
			if err != nil {
				log.Printf("Failed to start listener on %s (%s): %v", localAddr, desc, err)
				return
			}
			defer listener.Close()
			
			log.Printf("Proxy active: %s -> %s (%s)", localAddr, remoteAddr, desc)
			
			for {
				clientConn, err := listener.Accept()
				if err != nil {
					log.Printf("Failed to accept connection on %s: %v", localAddr, err)
					continue
				}
				
				go handleConnection(clientConn, remoteAddr)
			}
		}(config)
	}
	
	wg.Wait()
}

func runConfigReverseMode() {
	configFile := findConfigFile()
	if configFile == "" {
		log.Fatal("No .proxy.conf file found in current directory or parent directories")
	}

	configs, err := parseConfigFile(configFile)
	if err != nil {
		log.Fatalf("Failed to parse config file %s: %v", configFile, err)
	}

	if len(configs) == 0 {
		log.Fatal("No valid port configurations found in .proxy.conf")
	}

	log.Printf("Using config file: %s", configFile)

	var wg sync.WaitGroup
	for _, config := range configs {
		wg.Add(1)
		go func(cfg ProxyConfig) {
			defer wg.Done()
			
			localAddr := "localhost:" + cfg.Port
			externalAddr := "0.0.0.0:" + cfg.Port
			
			desc := cfg.Description
			if desc == "" {
				desc = "port " + cfg.Port
			}
			
			log.Printf("Starting reverse proxy for %s: %s -> %s", desc, externalAddr, localAddr)
			
			// Test local service connectivity
			conn, err := net.Dial("tcp", localAddr)
			if err != nil {
				log.Printf("Failed to connect to local service %s (%s): %v", localAddr, desc, err)
				return
			}
			conn.Close()
			
			// Start external listener
			listener, err := net.Listen("tcp", externalAddr)
			if err != nil {
				log.Printf("Failed to start listener on %s (%s): %v", externalAddr, desc, err)
				return
			}
			defer listener.Close()
			
			log.Printf("Proxy active: %s -> %s (%s)", externalAddr, localAddr, desc)
			
			for {
				clientConn, err := listener.Accept()
				if err != nil {
					log.Printf("Failed to accept connection on %s: %v", externalAddr, err)
					continue
				}
				
				go handleConnection(clientConn, localAddr)
			}
		}(config)
	}
	
	wg.Wait()
}

func getRemoteHost() string {
	// Try to get hostname from environment or config
	if host := os.Getenv("PROXY_REMOTE_HOST"); host != "" {
		return host
	}
	
	// For now, we'll prompt for it or use a default
	// In a real implementation, you might want to store this in the config file
	fmt.Print("Enter remote host (e.g., work-mbp.tailnet.ts.net): ")
	var host string
	fmt.Scanln(&host)
	return host
}