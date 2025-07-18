package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/spf13/cobra"
	tea "github.com/charmbracelet/bubbletea"
)

var (
	headless bool
	pm       *ProxyManager
)

func main() {
	pm = NewProxyManager()
	
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "proxy",
	Short: "A simple TCP proxy tool with TUI dashboard",
	Long: `A simple TCP proxy tool in Go that supports both forward and reverse proxy modes,
with automatic configuration file support and a beautiful TUI dashboard.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Default behavior: forward mode
		runForwardMode(cmd, args)
	},
}

var forwardCmd = &cobra.Command{
	Use:   "forward [remote:port localPort]",
	Short: "Forward localhost connections to remote servers",
	Long: `Forward mode connects localhost ports to remote servers.
Can be used with a config file for automatic setup or with manual arguments.`,
	Run: runForwardMode,
}

var reverseCmd = &cobra.Command{
	Use:   "reverse [localPort externalPort]",
	Short: "Expose localhost services on all network interfaces",
	Long: `Reverse mode exposes localhost services on all network interfaces (e.g., for Tailscale).
Can be used with a config file for automatic setup or with manual arguments.`,
	Aliases: []string{"r"},
	Run: runReverseMode,
}

func init() {
	// Add persistent flags
	rootCmd.PersistentFlags().BoolVar(&headless, "headless", false, "Run without TUI dashboard")
	
	// Add subcommands
	rootCmd.AddCommand(forwardCmd)
	rootCmd.AddCommand(reverseCmd)
	
	// Add flags to subcommands
	forwardCmd.Flags().BoolVar(&headless, "headless", false, "Run without TUI dashboard")
	reverseCmd.Flags().BoolVar(&headless, "headless", false, "Run without TUI dashboard")
}

func runForwardMode(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		// Auto forward mode using config file
		if headless {
			if err := pm.RunConfigForwardMode(); err != nil {
				log.Fatal(err)
			}
		} else {
			go func() {
				if err := pm.RunConfigForwardMode(); err != nil {
					log.Printf("Error in forward mode: %v", err)
				}
			}()
			runTUIMode(pm)
		}
	} else if len(args) == 2 {
		// Manual forward mode
		remoteAddr := args[0]
		localPort := args[1]
		
		if err := pm.RunSingleForwardProxy(remoteAddr, localPort); err != nil {
			log.Fatal(err)
		}
	} else {
		fmt.Fprintf(os.Stderr, "Usage: %s forward [remote:port localPort]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  %s forward                              # auto forward with config file\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s forward --headless                   # auto forward headless\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s forward work-mbp:8080 3000           # manual forward\n", os.Args[0])
		os.Exit(1)
	}
}

func runReverseMode(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		// Auto reverse mode using config file
		if headless {
			if err := pm.RunConfigReverseMode(); err != nil {
				log.Fatal(err)
			}
		} else {
			go func() {
				if err := pm.RunConfigReverseMode(); err != nil {
					log.Printf("Error in reverse mode: %v", err)
				}
			}()
			runTUIMode(pm)
		}
	} else if len(args) == 2 {
		// Manual reverse mode
		localPort := args[0]
		externalPort := args[1]
		
		if err := pm.RunSingleReverseProxy(localPort, externalPort); err != nil {
			log.Fatal(err)
		}
	} else {
		fmt.Fprintf(os.Stderr, "Usage: %s reverse [localPort externalPort]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  %s reverse                              # auto reverse with config file\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s reverse --headless                   # auto reverse headless\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s reverse 8080 8080                    # manual reverse\n", os.Args[0])
		os.Exit(1)
	}
}

func runTUIMode(pm *ProxyManager) {
	// Disable logging to prevent interference with TUI
	log.SetOutput(io.Discard)
	
	p := tea.NewProgram(initialModel(pm), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		// Re-enable logging for error output
		log.SetOutput(os.Stderr)
		log.Fatal(err)
	}
}