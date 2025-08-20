package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/firi/clangd-query/internal/client"
	"github.com/firi/clangd-query/internal/daemon"
)

type Config struct {
	Command     string
	Arguments   []string
	Limit       int
	Verbose     bool
	Timeout     int
	Help        bool
	ProjectRoot string
}

func parseArgs(args []string) (*Config, error) {
	config := &Config{
		Limit:   -1,
		Timeout: 30,
	}

	var positionalArgs []string
	i := 0

	for i < len(args) {
		arg := args[i]

		if strings.HasPrefix(arg, "--") {
			// Handle flags
			if arg == "--help" || arg == "-h" {
				config.Help = true
				i++
				continue
			}

			if arg == "--verbose" || arg == "-v" {
				config.Verbose = true
				i++
				continue
			}

			// Handle flags with values
			var key, value string
			if strings.Contains(arg, "=") {
				parts := strings.SplitN(arg, "=", 2)
				key = parts[0]
				value = parts[1]
				i++
			} else if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				key = arg
				value = args[i+1]
				i += 2
			} else {
				return nil, fmt.Errorf("flag %s requires a value", arg)
			}

			switch key {
			case "--limit":
				limit, err := strconv.Atoi(value)
				if err != nil {
					return nil, fmt.Errorf("invalid limit value: %s", value)
				}
				config.Limit = limit
			case "--timeout":
				timeout, err := strconv.Atoi(value)
				if err != nil {
					return nil, fmt.Errorf("invalid timeout value: %s", value)
				}
				config.Timeout = timeout
			default:
				return nil, fmt.Errorf("unknown flag: %s", key)
			}
		} else {
			// Positional argument
			positionalArgs = append(positionalArgs, arg)
			i++
		}
	}

	// First positional arg is the command
	if len(positionalArgs) > 0 {
		config.Command = positionalArgs[0]
		config.Arguments = positionalArgs[1:]
	}

	return config, nil
}

func printHelp() {
	fmt.Println(`clangd-query - Fast C++ code intelligence CLI

Usage:
  clangd-query <command> [arguments] [flags]

Commands:
  search <query>              Search for symbols across the project
  show <symbol|location>      Show declaration and definition
  view <symbol|location>      View complete source code
  usages <symbol|location>    Find all usages of a symbol
  hierarchy <symbol|location> Show type hierarchy
  signature <symbol|location> Show function signature
  interface <symbol|location> Show public interface
  logs                        Show daemon logs
  status                      Show daemon status
  shutdown                    Shutdown the daemon

Flags:
  --limit <n>      Limit number of results
  --verbose        Enable verbose output
  --timeout <s>    Request timeout in seconds (default: 30)
  --help           Show this help message

Examples:
  clangd-query search Widget
  clangd-query show GameScene::update
  clangd-query usages src/main.cpp:42:15
  clangd-query hierarchy BaseClass --limit 10`)
}

func findProjectRoot(startDir string) (string, error) {
	dir := startDir
	for {
		if _, err := os.Stat(filepath.Join(dir, "CMakeLists.txt")); err == nil {
			return dir, nil
		}
		
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("no CMakeLists.txt found in any parent directory")
}

func runDaemon(projectRoot string) {
	daemon.Run(projectRoot)
}

func runClient(config *Config) {
	clientConfig := &client.Config{
		Command:     config.Command,
		Arguments:   config.Arguments,
		Limit:       config.Limit,
		Verbose:     config.Verbose,
		Timeout:     config.Timeout,
		ProjectRoot: config.ProjectRoot,
	}
	
	if err := client.Run(clientConfig); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func main() {
	// Check if running as daemon (hidden mode)
	if len(os.Args) > 1 && os.Args[1] == "daemon" {
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "daemon mode requires project root argument\n")
			os.Exit(1)
		}
		runDaemon(os.Args[2])
		return
	}

	// Parse arguments
	config, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Handle help
	if config.Help || config.Command == "help" {
		printHelp()
		return
	}

	// Validate command
	validCommands := []string{"search", "show", "view", "usages", "hierarchy", 
		"signature", "interface", "logs", "status", "shutdown"}
	
	if config.Command == "" {
		fmt.Fprintf(os.Stderr, "Error: no command specified\n")
		printHelp()
		os.Exit(1)
	}

	commandValid := false
	for _, cmd := range validCommands {
		if config.Command == cmd {
			commandValid = true
			break
		}
	}

	if !commandValid {
		fmt.Fprintf(os.Stderr, "Error: unknown command '%s'\n", config.Command)
		printHelp()
		os.Exit(1)
	}

	// Find project root
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	projectRoot, err := findProjectRoot(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Run client with project root
	config.ProjectRoot = projectRoot
	runClient(config)
}