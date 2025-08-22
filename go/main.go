package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"clangd-query/internal/client"
	"clangd-query/internal/daemon"
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

	if len(args) == 0 {
		return config, nil
	}

	// Command is always first argument (unless it's --help)
	if args[0] == "--help" || args[0] == "-h" {
		config.Help = true
		return config, nil
	}

	// First arg is the command
	config.Command = args[0]

	// Parse everything after the command
	i := 1
	var commandArgs []string

	for i < len(args) {
		arg := args[i]

		// Check if it's a global flag
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

		// Handle global flags with values
		if arg == "--limit" || arg == "--timeout" {
			if i+1 >= len(args) {
				return nil, fmt.Errorf("flag %s requires a value", arg)
			}

			value := args[i+1]

			switch arg {
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
			}
			i += 2
			continue
		}

		// Everything else is a command argument
		commandArgs = append(commandArgs, arg)
		i++
	}

	config.Arguments = commandArgs
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

func runDaemon(projectRoot string, verbose bool) {
	config := &daemon.Config{
		ProjectRoot: projectRoot,
		Verbose:     verbose,
	}
	daemon.Run(config)
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
		projectRoot := os.Args[2]
		verbose := false
		// Check for --verbose flag
		for i := 3; i < len(os.Args); i++ {
			if os.Args[i] == "--verbose" || os.Args[i] == "-v" {
				verbose = true
				break
			}
		}
		runDaemon(projectRoot, verbose)
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
