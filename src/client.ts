#!/usr/bin/env node

/**
 * clangd-query - Fast C++ code intelligence CLI tool
 *
 * This is the client for the clangd-query system, providing fast access to C++ code
 * intelligence features through a persistent daemon. The client:
 *
 * - Auto-starts a background daemon if not already running
 * - Communicates via Unix domain sockets for low latency
 * - Provides human and AI-agent friendly output formats
 * - Supports all major C++ code navigation features
 *
 * Commands:
 * - search <query>         Search for symbols by name (fuzzy matching)
 * - show <symbol>          Show complete source code of a symbol
 * - usages <symbol>        Find all references to a symbol
 * - hierarchy <symbol>     Show base and derived classes of the symbol
 * - status                 Show daemon status information
 * - shutdown               Stop the background daemon
 *
 * The tool is designed for minimal startup time and clear output. When used for the
 * first time in a project, it will start a daemon that maintains a warm clangd
 * instance, making subsequent queries nearly instantaneous.
 *
 * Example usage:
 *   clangd-query search GameObject
 *   clangd-query show Update
 *   clangd-query usages GameObject::Update
 *   clangd-query hierarchy Character
 */

import * as net from "node:net";
import * as fs from "node:fs";
import * as path from "node:path";
import { spawn } from "node:child_process";
import {
  generateSocketPath,
  generateLockFilePath,
  readLockFile,
  cleanupStaleLockFile,
  isProcessRunning,
  calculateBuildTimestamp,
} from "./socket-utils.js";

// Command-line argument parsing
interface ParsedArgs {
  command: string;
  args: string[];
  options: {
    limit?: number;
    timeout?: number;
    help?: boolean;
    debug?: boolean;
    lines?: number;
    logLevel?: string;
  };
}

// JSON-RPC types
interface JsonRpcRequest {
  jsonrpc: "2.0";
  id: string | number;
  method: string;
  params?: any;
}

interface JsonRpcResponse {
  jsonrpc: "2.0";
  id: string | number;
  result?: any;
  error?: {
    code: number;
    message: string;
    data?: any;
  };
  logs?: string[]; // Request-specific debug logs
}

// Custom error for invalid query patterns
class InvalidQueryError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "InvalidQueryError";
  }
}

/**
 * Find project root by looking for CMakeLists.txt
 */
function findProjectRoot(startDir: string): string | null {
  let currentDir = startDir;

  while (currentDir !== path.dirname(currentDir)) {
    const cmakePath = path.join(currentDir, "CMakeLists.txt");
    if (fs.existsSync(cmakePath)) {
      return currentDir;
    }
    currentDir = path.dirname(currentDir);
  }

  return null;
}

/**
 * Parse command-line arguments with flexible ordering.
 * Flags can appear anywhere in the command line.
 */
function parseArgs(argv: string[]): ParsedArgs {
  const result: ParsedArgs = {
    command: "",
    args: [],
    options: {},
  };

  // First pass: separate flags from positional arguments
  const positionalArgs: string[] = [];
  let i = 2; // Skip node and script name

  while (i < argv.length) {
    const arg = argv[i];

    if (arg === "--help" || arg === "-h") {
      result.options.help = true;
      i++;
    } else if (arg === "--verbose" || arg === "-v") {
      result.options.debug = true;
      i++;
    } else if (arg === "--error") {
      result.options.logLevel = "error";
      i++;
    } else if (arg === "--info") {
      result.options.logLevel = "info";
      i++;
    } else if (arg === "--debug") {
      result.options.logLevel = "debug";
      i++;
    } else if (arg === "--limit") {
      if (i + 1 >= argv.length || argv[i + 1].startsWith("-")) {
        console.error(`Error: --limit requires a value`);
        console.error(`Example: clangd-query search Scene --limit 50`);
        process.exit(1);
      }
      const limit = parseInt(argv[i + 1], 10);
      if (isNaN(limit) || limit <= 0) {
        console.error(`Error: --limit must be a positive number, got: ${argv[i + 1]}`);
        process.exit(1);
      }
      result.options.limit = limit;
      i += 2;
    } else if (arg === "--timeout") {
      if (i + 1 >= argv.length || argv[i + 1].startsWith("-")) {
        console.error(`Error: --timeout requires a value`);
        console.error(`Example: clangd-query search Scene --timeout 60`);
        process.exit(1);
      }
      const timeout = parseInt(argv[i + 1], 10);
      if (isNaN(timeout) || timeout <= 0) {
        console.error(`Error: --timeout must be a positive number, got: ${argv[i + 1]}`);
        process.exit(1);
      }
      result.options.timeout = timeout;
      i += 2;
    } else if (arg === "--lines") {
      if (i + 1 >= argv.length || argv[i + 1].startsWith("-")) {
        console.error(`Error: --lines requires a value`);
        console.error(`Example: clangd-query logs --lines 50`);
        process.exit(1);
      }
      const lines = parseInt(argv[i + 1], 10);
      if (isNaN(lines) || lines <= 0) {
        console.error(`Error: --lines must be a positive number, got: ${argv[i + 1]}`);
        process.exit(1);
      }
      result.options.lines = lines;
      i += 2;
    } else if (arg.startsWith("-")) {
      console.error(`Unknown option: ${arg}`);
      console.error(`Run 'clangd-query --help' for usage information`);
      process.exit(1);
    } else {
      // It's a positional argument
      positionalArgs.push(arg);
      i++;
    }
  }

  // Extract command and arguments from positional args
  if (positionalArgs.length > 0) {
    result.command = positionalArgs[0];
    result.args = positionalArgs.slice(1);
  }

  return result;
}

/**
 * Show general help
 */
function showHelp(): void {
  console.log(`clangd-query - Fast C++ code intelligence tool

USAGE:
  clangd-query <command> <command_argument> [options]

COMMANDS:
  search <query>              Search for symbols by name (classes, functions, methods)
                              Example: search GameObject finds GameObject, CreateGameObject, etc.
                              Note: Single symbol names only - no spaces or regex patterns

  show <symbol>               Show complete source code and context
                              Example: show GameObject shows full class implementation
                              Example: show GameObject::Update shows declaration & definition

  usages <symbol|location>    Find all places where a symbol is used
                              Example: usages GameObject finds all uses of the GameObject class
                              Example: usages include/core/game_object.h:30:7 finds all uses at that location

  hierarchy <class>           Show inheritance hierarchy for a class
                              Example: hierarchy Character shows base and derived classes
                              Shows both parent classes (inherits from) and child classes

  signature <function>        Show function/method signatures with documentation
                              Example: signature Update shows all overloads
                              Includes parameters, return types, and comments

  interface <class>           Show public interface of a class
                              Example: interface Engine shows public API
                              Clean view of what the class exposes to users

  logs                        Show recent daemon logs
  status                      Show daemon health and statistics
  shutdown                    Stop the background daemon

OPTIONS:
  --verbose, -v               Show detailed request/response logs
  --limit <n>                 Limit number of results (default: 20)
  --timeout <seconds>         Request timeout (default: 30)

Note: Options can appear anywhere on the command line.

EXAMPLES:
  clangd-query search GameObject
  clangd-query show Update
  clangd-query search Transform --verbose    # Options can come after the command
  clangd-query usages GameObject --timeout 60 --verbose

For command-specific help, use:
  clangd-query <command> --help`);
}

/**
 * Show command-specific help
 */
function showCommandHelp(command: string): void {
  switch (command) {
    case "search":
      console.log(`Search for C++ symbols by name

USAGE:
  clangd-query search <query> [--limit <n>]

DESCRIPTION:
  Searches all symbols using best-match scoring. Finds symbols containing
  your query anywhere in their name. Single symbol names only - no spaces.

EXAMPLES:
  search "Scene"              # Finds: Scene, SceneManager, GameScene, MenuScene
  search "Load"               # Finds: LoadResources, LoadTexture, AsyncLoad, Preload
  search "Button"             # Finds: Button, ButtonView, RadioButton, SubmitButton

IMPORTANT: Search uses fuzzy matching on single terms only. Cannot search for
multiple words like "Scene Load" or use regex patterns.

TIP: Use when exploring a codebase or finding symbol locations`);
      break;

    case "view":
      console.log(`View complete source code of a symbol

USAGE:
  clangd-query view <symbol>

DESCRIPTION:
  Shows the COMPLETE implementation including all methods, members, and bodies.
  Much better than grep - gets the entire class/function definition.

EXAMPLES:
  view "SceneManager"         # Shows entire class with all methods
  view "Update"               # Shows complete function implementation
  view "GameObject::Update"   # Shows specific method implementation

TIP: Use to understand how something works or to see full implementations`);
      break;

    case "usages":
      console.log(`Find all usages of a symbol

USAGE:
  clangd-query usages <symbol|location>

DESCRIPTION:
  Find every place a symbol is used. Critical for refactoring or understanding impact.
  Shows calls, instantiations, and references with code context.

  You can provide either:
  - A symbol name (e.g., "SceneManager") - will search for it and show its usages
  - A file location (e.g., "src/scene.h:45:7") - shows usages at that specific location

EXAMPLES:
  usages "SceneManager"        # Find all uses of SceneManager class
  usages "LoadResources"       # Find all calls to LoadResources function
  usages "src/scene.h:45:7"    # Find all uses of symbol at that location
  usages "src/view.cc:100:5"   # Find all calls to method at that location

TIP: Use before renaming/changing to see what will be affected`);
      break;

    case "hierarchy":
      console.log(`Show class inheritance hierarchy

USAGE:
  clangd-query hierarchy <class>

DESCRIPTION:
  Display the complete inheritance tree for a class, showing both:
  - Base classes (what this class inherits from)
  - Derived classes (what inherits from this class)

  The output shows a tree structure with file locations for each class.

EXAMPLES:
  hierarchy "Scene"            # Show hierarchy for Scene class
  hierarchy "GameObject"       # Show all GameObject subclasses and base classes
  hierarchy "Character"        # Show Character inheritance tree

OUTPUT FORMAT:
  Shows inheritance relationships with tree characters:
  - Base classes listed under "Inherits from:"
  - The target class in the middle
  - Derived classes shown as a tree below

TIP: Use to understand class relationships and find all implementations`);
      break;

    case "signature":
      console.log(`Show function/method signatures with documentation

USAGE:
  clangd-query signature <function/method>

DESCRIPTION:
  Display detailed signature information for functions and methods including:
  - Full type signatures with parameter names and types
  - Return types and template parameters
  - Access levels (public/private/protected) for methods
  - Function modifiers (virtual, const, static, etc.)
  - Documentation comments if available
  - Shows all overloads if multiple exist

EXAMPLES:
  signature "GameObject::Update" # Show GameObject::Update signature
  signature "Update"             # Show Update method signatures
  signature "CreateGameObject"   # Show CreateGameObject function signatures

OUTPUT FORMAT:
  Each signature shows:
  - Location with file:line:column
  - Access level and full signature
  - Return type
  - Parameter details with types and names
  - Documentation/comments if present
  - Function attributes (virtual, override, const, etc.)

TIP: Use to understand function interfaces and find overloads`);
      break;

    case "interface":
      console.log(`Show public interface of a class

USAGE:
  clangd-query interface <class>

DESCRIPTION:
  Display only the public methods and members of a class, providing a clean
  API reference without implementation details. Shows what users of the class
  can access.

  Includes:
  - Public constructors and destructor
  - Public methods with full signatures
  - Public member variables
  - Public operators
  - Public type aliases (using/typedef)

EXAMPLES:
  interface "Engine"           # Show public API of Engine
  interface "GameObject"       # Show public interface of GameObject class
  interface "SceneManager"     # Show what SceneManager exposes

OUTPUT FORMAT:
  Shows a clean class definition with only public members:
  class ClassName {
  public:
    // Constructors/destructor
    // Methods
    // Member variables
    // Type aliases
  };

TIP: Use to quickly understand what a class offers without reading implementation`);
      break;

    case "show":
      console.log(`Show complete source code and context

USAGE:
  clangd-query show <symbol>

DESCRIPTION:
  Display complete source code of a symbol, intelligently handling C++
  declaration/definition split. For functions and methods, shows BOTH:
  - Declaration from header file (with doc comments and signature)
  - Definition from source file (with implementation details)

  For classes, shows the complete class definition with all members.
  The command automatically extracts the right amount of context to understand
  the symbol, including preceding comments and complete function bodies.

EXAMPLES:
  show GameObject::Update      # Shows declaration in .h and definition in .cpp
  show GameObject              # Shows complete class definition
  show CreateGameObject        # Shows function declaration and implementation
  show Character::Update       # Shows both declaration and definition

OUTPUT FORMAT:
  - Shows file location for each section
  - Displays declaration first (if separate from definition)
  - Includes surrounding context and comments
  - Formats code in syntax-highlighted blocks

WHY USE THIS:
  - Perfect for understanding how something works
  - Shows exactly what you need: complete implementation
  - Eliminates jumping between header and source files
  - Great for code reviews and understanding unfamiliar code

TIP: Use when you need to understand what something does and how it's implemented`);
      break;

    case "logs":
      console.log(`Show recent daemon logs

USAGE:
  clangd-query logs [--lines <n>] [--error | --info | --debug]

DESCRIPTION:
  Displays recent logs from the daemon, including initialization logs,
  clangd output, and file watching registration.

OPTIONS:
  --lines <n>    Number of log lines to show (default: 100)
  --error        Show only ERROR level logs
  --info         Show INFO and ERROR logs (default)
  --debug        Show all logs (DEBUG, INFO, ERROR)`);
      break;

    case "status":
      console.log(`Show daemon status

USAGE:
  clangd-query status

DESCRIPTION:
  Shows information about the running daemon including uptime,
  memory usage, and indexing status.`);
      break;

    case "shutdown":
      console.log(`Stop the daemon

USAGE:
  clangd-query shutdown

DESCRIPTION:
  Gracefully stops the background daemon for the current project.`);
      break;

    default:
      console.log(`Unknown command: ${command}`);
      console.log(`Run 'clangd-query --help' for available commands`);
  }
}

/**
 * Check if daemon is running
 */
async function isDaemonRunning(projectRoot: string): Promise<boolean> {
  const lockFilePath = generateLockFilePath(projectRoot);
  const lockData = readLockFile(lockFilePath);

  if (!lockData) {
    return false;
  }

  // Check if process is alive
  if (!isProcessRunning(lockData.pid)) {
    // Clean up stale lock file
    cleanupStaleLockFile(lockFilePath);
    return false;
  }

  // Try to connect to socket
  const socketPath = generateSocketPath(projectRoot);
  try {
    await new Promise<void>((resolve, reject) => {
      const client = net.createConnection(socketPath, () => {
        client.end();
        resolve();
      });
      client.on("error", reject);
      client.setTimeout(1000);
      client.on("timeout", () => {
        client.destroy();
        reject(new Error("Connection timeout"));
      });
    });
    return true;
  } catch {
    // Can't connect, daemon is not running
    cleanupStaleLockFile(lockFilePath);
    return false;
  }
}

/**
 * Start the daemon
 */
async function startDaemon(projectRoot: string): Promise<void> {
  console.error(`Starting clangd-daemon for ${projectRoot}...`);

  // Find the daemon script
  const daemonPath = path.join(path.dirname(new URL(import.meta.url).pathname), "daemon.js");

  // Spawn daemon process
  const daemon = spawn("node", [daemonPath, projectRoot], {
    detached: true,
    stdio: "ignore",
    env: {
      ...process.env,
      // Pass through any relevant environment variables
      CLANGD_DAEMON_TIMEOUT: process.env.CLANGD_DAEMON_TIMEOUT,
    },
  });

  // Let the daemon process run independently
  daemon.unref();

  // Wait for daemon to be ready (check lock file and socket)
  const lockFilePath = generateLockFilePath(projectRoot);
  const maxWaitTime = 10000; // 10 seconds
  const startTime = Date.now();

  while (Date.now() - startTime < maxWaitTime) {
    if (await isDaemonRunning(projectRoot)) {
      return;
    }

    // Wait a bit before checking again
    await new Promise(resolve => setTimeout(resolve, 100));
  }

  throw new Error("Failed to start daemon: timeout waiting for daemon to be ready");
}

/**
 * Send request to daemon
 */
async function sendRequest(
  projectRoot: string,
  method: string,
  params: any,
  timeout: number = 30000,
  debug: boolean = false
): Promise<any> {
  const socketPath = generateSocketPath(projectRoot);

  return new Promise((resolve, reject) => {
    const client = net.createConnection(socketPath);
    let responseData = "";

    // Set timeout
    client.setTimeout(timeout);

    client.on("connect", () => {
      // Send JSON-RPC request
      const request: JsonRpcRequest = {
        jsonrpc: "2.0",
        id: Date.now(),
        method,
        params,
      };

      client.write(JSON.stringify(request) + "\n");
    });

    client.on("data", (data) => {
      responseData += data.toString();

      // Check if we have a complete JSON response (ends with newline)
      if (responseData.includes("\n")) {
        const lines = responseData.split("\n");
        const jsonLine = lines[0];

        try {
          const response: JsonRpcResponse = JSON.parse(jsonLine);

          if (response.error) {
            reject(new Error(response.error.message));
          } else {
            // Display verbose logs if requested
            if (debug && response.logs && response.logs.length > 0) {
              console.error("\n--- Verbose Logs ---");
              for (const log of response.logs) {
                console.error(log);
              }
              console.error("--- End Verbose Logs ---\n");
            }

            resolve(response.result);
          }
        } catch (error) {
          reject(new Error(`Invalid response from daemon: ${error}`));
        }

        client.end();
      }
    });

    client.on("timeout", () => {
      client.destroy();
      reject(new Error("Request timeout"));
    });

    client.on("error", (error) => {
      reject(error);
    });
  });
}


/**
 * Process a symbol query by detecting and cleaning regex patterns.
 * Shows warnings if patterns are detected and returns the cleaned query.
 * Throws InvalidQueryError if no valid query can be extracted.
 */
function processSymbolQuery(query: string, commandName: string): string {
  const { cleaned, hasPattern } = detectAndCleanRegexPattern(query);
  
  // If we couldn't extract any valid search term, throw an error
  if (hasPattern && !cleaned) {
    throw new InvalidQueryError(`Could not extract a valid symbol name from the pattern: '${query}'`);
  }
  
  return cleaned || query;
}

/**
 * Detect regex-like patterns in a query and clean them up for clangd's fuzzy search.
 * Returns the cleaned query and whether a pattern was detected.
 */
function detectAndCleanRegexPattern(query: string): { cleaned: string; hasPattern: boolean } {
  // Check if query contains any regex-like characters
  const regexChars = /[\s*?^$+|[\]{}]/;
  const hasPattern = regexChars.test(query);
  
  if (!hasPattern) {
    return { cleaned: query, hasPattern: false };
  }
  
  // Extract the first alphanumeric word
  const match = query.match(/[a-zA-Z_]\w*/);
  const cleaned = match ? match[0] : '';
  
  // Show warning
  console.error(`Warning: clangd-query does not support wildcard patterns or regex syntax.`);
  if (cleaned) {
    console.error(`Searching instead for: '${cleaned}'`);
  } else {
    console.error(`Could not extract a valid search term from: '${query}'`);
  }
  
  return { cleaned, hasPattern: true };
}

/**
 * Execute a command with the given arguments
 */
async function executeCommand(
  projectRoot: string,
  command: string,
  args: string[],
  options: ParsedArgs["options"]
): Promise<void> {
  let result: any;

  switch (command) {
    case "search":
      result = await sendRequest(projectRoot, "searchSymbols", {
        query: processSymbolQuery(args[0], "search"),
        limit: options.limit || 20,
      }, options.timeout ? options.timeout * 1000 : undefined, options.debug);
      console.log(result.text);
      break;

    case "view":
      result = await sendRequest(projectRoot, "viewSourceCode", {
        query: processSymbolQuery(args[0], "view"),
      }, options.timeout ? options.timeout * 1000 : undefined, options.debug);
      console.log(result.text);
      break;

    case "usages":
      const input = args[0];

      // Check if input looks like a file location (path with line:col)
      // File paths contain '/' and have the pattern file:line:col
      const isFileLocation = input.includes('/') && /:\d+:\d+$/.test(input);

      if (isFileLocation) {
        // It's a file location, use directly
        result = await sendRequest(projectRoot, "findReferences", {
          location: input,
        }, options.timeout ? options.timeout * 1000 : undefined, options.debug);
      } else {
        // It's a symbol name, process it for regex patterns
        result = await sendRequest(projectRoot, "findReferencesToSymbol", {
          symbolName: processSymbolQuery(input, "usages"),
        }, options.timeout ? options.timeout * 1000 : undefined, options.debug);
      }

      console.log(result.text);
      break;

    case "hierarchy":
      result = await sendRequest(projectRoot, "getTypeHierarchy", {
        className: processSymbolQuery(args[0], "hierarchy"),
      }, options.timeout ? options.timeout * 1000 : undefined, options.debug);
      console.log(result.text);
      break;

    case "signature":
      result = await sendRequest(projectRoot, "getSignature", {
        functionName: processSymbolQuery(args[0], "signature"),
      }, options.timeout ? options.timeout * 1000 : undefined, options.debug);
      console.log(result.text);
      break;

    case "interface":
      result = await sendRequest(projectRoot, "getInterface", {
        className: processSymbolQuery(args[0], "interface"),
      }, options.timeout ? options.timeout * 1000 : undefined, options.debug);
      console.log(result.text);
      break;

    case "show":
      result = await sendRequest(projectRoot, "getShow", {
        symbol: processSymbolQuery(args[0], "show"),
      }, options.timeout ? options.timeout * 1000 : undefined, options.debug);
      console.log(result.text);
      break;

    case "logs":
      const lineCount = options.lines || 100; // default to 100
      const logLevel = options.logLevel || "info"; // default to INFO and ERROR
      
      result = await sendRequest(projectRoot, "getRecentLogs", { 
        lines: lineCount,
        logLevel: logLevel 
      }, undefined, options.debug);
      
      if (result.logs && result.logs.length > 0) {
        console.log(`--- Last ${result.logs.length} log entries ---`);
        for (const log of result.logs) {
          console.log(log);
        }
        console.log("--- End of logs ---");
      } else {
        console.log("No logs available");
      }
      break;

    case "status":
      result = await sendRequest(projectRoot, "getStatus", {}, undefined, options.debug);
      // Format status nicely
      console.log(`clangd-daemon running for ${Math.floor(result.uptime / 60000)} minutes`);
      console.log(`Project: ${result.projectRoot}`);
      console.log(`Memory: ${result.memory.rss}`);
      console.log(`Indexed files: ${result.indexingComplete ? "complete" : "in progress"}`);
      console.log(`Recent queries: ${result.requestCount}`);
      break;

    case "shutdown":
      await sendRequest(projectRoot, "shutdown", {}, undefined, options.debug);
      console.log("clangd-daemon stopped");
      break;

    default:
      // This should never happen because we check above
      throw new Error(`Unknown command: ${command}`);
  }
}

/**
 * Main entry point
 */
async function main(): Promise<void> {
  const parsed = parseArgs(process.argv);

  // Handle help
  if (parsed.options.help || !parsed.command) {
    if (parsed.command && parsed.command !== "--help" && parsed.command !== "-h") {
      showCommandHelp(parsed.command);
    } else {
      showHelp();
    }
    process.exit(0);
  }

  // Find project root
  const projectRoot = findProjectRoot(process.cwd());
  if (!projectRoot) {
    console.error("Error: Could not find project root (no CMakeLists.txt found)");
    console.error("Make sure you're running this command from within a C++ project");
    process.exit(1);
  }

  try {
    // Check if daemon is running and if it needs restart due to code changes
    const lockFilePath = generateLockFilePath(projectRoot);
    const lockData = readLockFile(lockFilePath);
    const isRunning = await isDaemonRunning(projectRoot);

    // Check if daemon code has been rebuilt
    let needsRestart = false;
    if (isRunning && lockData?.buildTimestamp) {
      const currentBuildTimestamp = calculateBuildTimestamp(import.meta.url);
      if (currentBuildTimestamp > lockData.buildTimestamp) {
        console.error("Daemon code has been updated, restarting daemon...");
        needsRestart = true;

        // Send shutdown command to the old daemon
        try {
          await sendRequest(projectRoot, "shutdown", {}, 5000);
          // Wait a bit for daemon to shut down
          await new Promise(resolve => setTimeout(resolve, 500));
        } catch (error) {
          // Log the error but continue with restart
          console.error(`Warning: Failed to cleanly shutdown old daemon: ${error}`);
          console.error("Continuing with restart...");
        }
      }
    }

    // Start daemon if not running or needs restart
    if (!isRunning || needsRestart) {
      // Don't auto-start for status/shutdown commands
      if (parsed.command === "status") {
        console.log("clangd-daemon not running");
        return;
      }
      if (parsed.command === "shutdown") {
        console.log("clangd-daemon not running");
        return;
      }

      // Start daemon
      await startDaemon(projectRoot);
    }

    // Validate command arguments
    const commandExpectations: Record<string, { argCount: number, argName?: string }> = {
      search: { argCount: 1, argName: "query" },
      view: { argCount: 1, argName: "symbol" },
      show: { argCount: 1, argName: "symbol" },
      usages: { argCount: 1, argName: "symbol or location" },
      hierarchy: { argCount: 1, argName: "class name" },
      signature: { argCount: 1, argName: "function/method name" },
      interface: { argCount: 1, argName: "class name" },
      context: { argCount: 1, argName: "symbol" },
      logs: { argCount: 0 },
      status: { argCount: 0 },
      shutdown: { argCount: 0 },
    };

    const expectation = commandExpectations[parsed.command];
    if (!expectation) {
      console.error(`Unknown command: ${parsed.command}\n`);
      showHelp();
      process.exit(1);
    }

    // Validate argument count
    if (expectation.argCount === 0 && parsed.args.length > 0) {
      console.error(`Error: ${parsed.command} does not accept any arguments`);
      console.error(`Got: ${parsed.args.join(' ')}`);
      console.error(`Usage: clangd-query ${parsed.command}`);
      process.exit(1);
    } else if (expectation.argCount === 1) {
      if (parsed.args.length === 0) {
        console.error(`Error: ${parsed.command} requires a ${expectation.argName} argument`);
        console.error(`Usage: clangd-query ${parsed.command} <${expectation.argName}>`);
        process.exit(1);
      }
      if (parsed.args.length > 1) {
        console.error(`Error: ${parsed.command} expects only one argument`);
        console.error(`Got: ${parsed.args.join(' ')}`);
        console.error(`Usage: clangd-query ${parsed.command} <${expectation.argName}>`);
        process.exit(1);
      }
    }

    // Execute command
    await executeCommand(projectRoot, parsed.command, parsed.args, parsed.options);
  } catch (error) {
    if (error instanceof InvalidQueryError) {
      // Invalid query errors are already user-friendly, just exit
      process.exit(1);
    } else if (error instanceof Error) {
      console.error(`Error: ${error.message}`);

      // Provide helpful error messages
      if (error.message.includes("ECONNREFUSED")) {
        console.error("\nThe daemon appears to have crashed. Try running the command again.");
      } else if (error.message.includes("timeout")) {
        console.error("\nThe operation timed out. This might happen on first run while indexing.");
        console.error("Try increasing the timeout with --timeout <seconds>");
      }
    } else {
      console.error(`Error: ${error}`);
    }
    process.exit(1);
  }
}

// Run main
main().catch((error) => {
  console.error(`Fatal error: ${error}`);
  process.exit(1);
});