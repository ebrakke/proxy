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
	"sync/atomic"
	"time"
)

type ProxyConfig struct {
	Port        string
	Description string
}

type ProxyStats struct {
	Port              string
	Description       string
	Status            string
	ActiveConnections int64
	TotalConnections  int64
	BytesTransferred  int64
	LastActivity      time.Time
	StartTime         time.Time
	LocalAddr         string
	RemoteAddr        string
}

type ProxyManager struct {
	stats   map[string]*ProxyStats
	mu      sync.RWMutex
	configs []ProxyConfig
}

func NewProxyManager() *ProxyManager {
	return &ProxyManager{
		stats: make(map[string]*ProxyStats),
	}
}

func (pm *ProxyManager) GetStats() map[string]*ProxyStats {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	result := make(map[string]*ProxyStats)
	for k, v := range pm.stats {
		statsCopy := *v
		result[k] = &statsCopy
	}
	return result
}

func (pm *ProxyManager) UpdateStats(port string, field string, value interface{}) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	if pm.stats[port] == nil {
		return
	}
	
	switch field {
	case "active_connections":
		pm.stats[port].ActiveConnections = value.(int64)
	case "total_connections":
		atomic.AddInt64(&pm.stats[port].TotalConnections, value.(int64))
	case "bytes_transferred":
		atomic.AddInt64(&pm.stats[port].BytesTransferred, value.(int64))
	case "last_activity":
		pm.stats[port].LastActivity = time.Now()
	case "status":
		pm.stats[port].Status = value.(string)
	}
}

func findConfigFile() string {
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
		
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		config := ProxyConfig{
			Port: parts[0],
		}
		
		if len(parts) > 1 {
			config.Description = parts[1]
		}

		if _, err := strconv.Atoi(config.Port); err != nil {
			log.Printf("Skipping invalid port: %s", config.Port)
			continue
		}

		configs = append(configs, config)
	}

	return configs, scanner.Err()
}

func (pm *ProxyManager) RunSingleReverseProxy(localPort, externalPort string) error {
	localAddr := "localhost:" + localPort
	externalAddr := "0.0.0.0:" + externalPort

	pm.mu.Lock()
	pm.stats[localPort] = &ProxyStats{
		Port:        localPort,
		Description: "Manual reverse proxy",
		Status:      "Starting",
		StartTime:   time.Now(),
		LocalAddr:   localAddr,
		RemoteAddr:  externalAddr,
	}
	pm.mu.Unlock()

	conn, err := net.Dial("tcp", localAddr)
	if err != nil {
		pm.UpdateStats(localPort, "status", "Failed - Local service unavailable")
		return fmt.Errorf("failed to connect to local service %s: %v", localAddr, err)
	}
	conn.Close()

	listener, err := net.Listen("tcp", externalAddr)
	if err != nil {
		pm.UpdateStats(localPort, "status", "Failed - Cannot bind")
		return fmt.Errorf("failed to start listener on %s: %v", externalAddr, err)
	}
	defer listener.Close()

	pm.UpdateStats(localPort, "status", "Active")
	log.Printf("Reverse TCP proxy started: %s -> %s", externalAddr, localAddr)

	for {
		clientConn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection on %s: %v", externalAddr, err)
			continue
		}

		go pm.handleConnection(clientConn, localAddr, localPort)
	}
}

func (pm *ProxyManager) RunSingleForwardProxy(remoteAddr, localPort string) error {
	localAddr := "localhost:" + localPort

	pm.mu.Lock()
	pm.stats[localPort] = &ProxyStats{
		Port:        localPort,
		Description: "Manual forward proxy",
		Status:      "Starting",
		StartTime:   time.Now(),
		LocalAddr:   localAddr,
		RemoteAddr:  remoteAddr,
	}
	pm.mu.Unlock()

	conn, err := net.Dial("tcp", remoteAddr)
	if err != nil {
		pm.UpdateStats(localPort, "status", "Failed - Remote unavailable")
		return fmt.Errorf("failed to connect to remote server %s: %v", remoteAddr, err)
	}
	conn.Close()

	listener, err := net.Listen("tcp", localAddr)
	if err != nil {
		pm.UpdateStats(localPort, "status", "Failed - Cannot bind")
		return fmt.Errorf("failed to start listener on %s: %v", localAddr, err)
	}
	defer listener.Close()

	pm.UpdateStats(localPort, "status", "Active")
	log.Printf("Forward TCP proxy started: %s -> %s", localAddr, remoteAddr)

	for {
		clientConn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection on %s: %v", localAddr, err)
			continue
		}

		go pm.handleConnection(clientConn, remoteAddr, localPort)
	}
}

func (pm *ProxyManager) RunConfigReverseMode() error {
	configFile := findConfigFile()
	if configFile == "" {
		return fmt.Errorf("no .proxy.conf file found")
	}

	configs, err := parseConfigFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to parse config file %s: %v", configFile, err)
	}

	if len(configs) == 0 {
		return fmt.Errorf("no valid port configurations found")
	}

	pm.configs = configs
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

			pm.mu.Lock()
			pm.stats[cfg.Port] = &ProxyStats{
				Port:        cfg.Port,
				Description: desc,
				Status:      "Starting",
				StartTime:   time.Now(),
				LocalAddr:   localAddr,
				RemoteAddr:  externalAddr,
			}
			pm.mu.Unlock()
			
			conn, err := net.Dial("tcp", localAddr)
			if err != nil {
				pm.UpdateStats(cfg.Port, "status", "Failed - Local service unavailable")
				log.Printf("Failed to connect to local service %s (%s): %v", localAddr, desc, err)
				return
			}
			conn.Close()
			
			listener, err := net.Listen("tcp", externalAddr)
			if err != nil {
				pm.UpdateStats(cfg.Port, "status", "Failed - Cannot bind")
				log.Printf("Failed to start listener on %s (%s): %v", externalAddr, desc, err)
				return
			}
			defer listener.Close()
			
			pm.UpdateStats(cfg.Port, "status", "Active")
			log.Printf("Reverse proxy active: %s -> %s (%s)", externalAddr, localAddr, desc)
			
			for {
				clientConn, err := listener.Accept()
				if err != nil {
					log.Printf("Failed to accept connection on %s: %v", externalAddr, err)
					continue
				}
				
				go pm.handleConnection(clientConn, localAddr, cfg.Port)
			}
		}(config)
	}
	
	wg.Wait()
	return nil
}

func (pm *ProxyManager) RunConfigForwardMode() error {
	configFile := findConfigFile()
	if configFile == "" {
		return fmt.Errorf("no .proxy.conf file found")
	}

	configs, err := parseConfigFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to parse config file %s: %v", configFile, err)
	}

	if len(configs) == 0 {
		return fmt.Errorf("no valid port configurations found")
	}

	pm.configs = configs
	log.Printf("Using config file: %s", configFile)

	remoteHost := getRemoteHost()
	if remoteHost == "" {
		return fmt.Errorf("could not determine remote host")
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

			pm.mu.Lock()
			pm.stats[cfg.Port] = &ProxyStats{
				Port:        cfg.Port,
				Description: desc,
				Status:      "Starting",
				StartTime:   time.Now(),
				LocalAddr:   localAddr,
				RemoteAddr:  remoteAddr,
			}
			pm.mu.Unlock()
			
			conn, err := net.Dial("tcp", remoteAddr)
			if err != nil {
				pm.UpdateStats(cfg.Port, "status", "Failed - Remote unavailable")
				log.Printf("Failed to connect to %s (%s): %v", remoteAddr, desc, err)
				return
			}
			conn.Close()
			
			listener, err := net.Listen("tcp", localAddr)
			if err != nil {
				pm.UpdateStats(cfg.Port, "status", "Failed - Cannot bind")
				log.Printf("Failed to start listener on %s (%s): %v", localAddr, desc, err)
				return
			}
			defer listener.Close()
			
			pm.UpdateStats(cfg.Port, "status", "Active")
			log.Printf("Forward proxy active: %s -> %s (%s)", localAddr, remoteAddr, desc)
			
			for {
				clientConn, err := listener.Accept()
				if err != nil {
					log.Printf("Failed to accept connection on %s: %v", localAddr, err)
					continue
				}
				
				go pm.handleConnection(clientConn, remoteAddr, cfg.Port)
			}
		}(config)
	}
	
	wg.Wait()
	return nil
}

func (pm *ProxyManager) handleConnection(clientConn net.Conn, remoteAddr, port string) {
	defer clientConn.Close()

	atomic.AddInt64(&pm.stats[port].ActiveConnections, 1)
	pm.UpdateStats(port, "total_connections", int64(1))
	pm.UpdateStats(port, "last_activity", nil)

	defer atomic.AddInt64(&pm.stats[port].ActiveConnections, -1)

	remoteConn, err := net.Dial("tcp", remoteAddr)
	if err != nil {
		log.Printf("Failed to connect to remote server %s: %v", remoteAddr, err)
		return
	}
	defer remoteConn.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		bytes, err := io.Copy(remoteConn, clientConn)
		if err != nil {
			log.Printf("Error copying client->remote: %v", err)
		}
		pm.UpdateStats(port, "bytes_transferred", bytes)
		pm.UpdateStats(port, "last_activity", nil)
	}()

	go func() {
		defer wg.Done()
		bytes, err := io.Copy(clientConn, remoteConn)
		if err != nil {
			log.Printf("Error copying remote->client: %v", err)
		}
		pm.UpdateStats(port, "bytes_transferred", bytes)
		pm.UpdateStats(port, "last_activity", nil)
	}()

	wg.Wait()
}

func getRemoteHost() string {
	if host := os.Getenv("PROXY_REMOTE_HOST"); host != "" {
		return host
	}
	
	fmt.Print("Enter remote host (e.g., work-mbp.tailnet.ts.net): ")
	var host string
	fmt.Scanln(&host)
	return host
}