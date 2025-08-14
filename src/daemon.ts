#!/usr/bin/env node

/**
 * clangd-daemon - Background server for clangd-query
 *
 * This daemon maintains a persistent clangd instance to provide fast code intelligence
 * queries without repeated indexing overhead. Features:
 * - Single daemon per project root via lock files
 * - Unix domain socket communication with JSON-RPC 2.0 protocol
 * - Automatic idle timeout after 30 minutes of inactivity
 * - Graceful shutdown with proper resource cleanup
 * - Concurrent client handling
 *
 * The daemon is automatically started by clangd-query when needed and runs in the
 * background until explicitly stopped or idle timeout expires.
 */

import * as net from "node:net";
import * as fs from "node:fs";
import * as path from "node:path";
import * as os from "node:os";
import { ClangdClient } from "./clangd-client.js";
import { Logger } from "./logger.js";
import {
  generateSocketPath,
  generateLockFilePath,
  getLogFilePath,
  readLockFile,
  writeLockFile,
  cleanupStaleLockFile,
  calculateBuildTimestamp,
  type LockFileData,
} from "./socket-utils.js";
import * as commands from "./commands/index.js";
import { FileWatcher, FileEvent } from "./file-watcher.js";

// Types for JSON-RPC 2.0
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

interface DaemonStatus {
  uptime: number;
  projectRoot: string;
  memory: {
    heapUsed: string;
    heapTotal: string;
    rss: string;
  };
  requestCount: number;
  lastRequestTime: number;
  indexingComplete: boolean;
}

enum LogLevel {
  ERROR = 0,
  INFO = 1,
  DEBUG = 2,
}

/**
 * Logger implementation that captures logs to a buffer during request processing.
 * This allows the daemon to capture logs from ClangdClient and include them in responses.
 */
class RequestLogger implements Logger {
  constructor(private buffer: string[]) {}

  error(message: string, ...args: any[]): void {
    this.captureLog(LogLevel.ERROR, message, args);
  }

  info(message: string, ...args: any[]): void {
    this.captureLog(LogLevel.INFO, message, args);
  }

  debug(message: string, ...args: any[]): void {
    this.captureLog(LogLevel.DEBUG, message, args);
  }

  private captureLog(level: LogLevel, message: string, args: any[]): void {
    const levelName = LogLevel[level];
    let logMessage = `[${levelName}] ${message}`;

    // Add arguments if any
    if (args.length > 0) {
      // Format each argument nicely
      for (const arg of args) {
        if (typeof arg === 'string') {
          // Try to parse as JSON for better formatting
          try {
            const parsed = JSON.parse(arg);
            logMessage += "\n" + JSON.stringify(parsed, null, 2);
          } catch {
            // Not JSON, just append as-is
            logMessage += "\n" + arg;
          }
        } else {
          logMessage += "\n" + JSON.stringify(arg, null, 2);
        }
      }
    }

    // Always capture to buffer
    this.buffer.push(logMessage);
  }
}

interface LogEntry {
  level: LogLevel;
  timestamp: string;
  message: string;
}

/**
 * Logger implementation for the daemon's internal operations.
 * Writes to the daemon's log file and maintains recent logs in memory.
 */
class DaemonLogger implements Logger {
  private logStream: fs.WriteStream;
  private recentLogs: LogEntry[] = [];
  private readonly maxRecentLogs = 1000;

  constructor(
    private logFilePath: string,
    private level: LogLevel
  ) {
    // Create log directory if needed
    const logDir = path.dirname(this.logFilePath);
    fs.mkdirSync(logDir, { recursive: true });

    // Delete log file if it's too large (> 1MB)
    const maxLogSize = 1024 * 1024; // 1MB
    try {
      const stats = fs.statSync(this.logFilePath);
      if (stats.size > maxLogSize) {
        fs.unlinkSync(this.logFilePath);
      }
    } catch {
      // File doesn't exist yet, that's fine
    }

    // Create log stream with append mode
    this.logStream = fs.createWriteStream(this.logFilePath, {
      flags: "a",
      encoding: "utf8",
    });
  }

  /**
   * Get filtered logs based on log level
   * @param requestedLevel The minimum log level to include ("error", "info", or "debug")
   * @param lines Maximum number of lines to return
   */
  getFilteredLogs(requestedLevel: string, lines: number): { logs: string[], totalCount: number } {
    // Filter logs based on requested level
    let filteredLogs = this.recentLogs;

    if (requestedLevel !== "debug") {
      const minLevel = requestedLevel === "error" ? LogLevel.ERROR : LogLevel.INFO;
      filteredLogs = this.recentLogs.filter(entry => entry.level <= minLevel);
    }

    // Get the last N lines from filtered logs
    const startIndex = Math.max(0, filteredLogs.length - lines);
    const selectedLogs = filteredLogs.slice(startIndex);

    // Format logs as strings for output
    const formattedLogs = selectedLogs.map(entry =>
      `[${entry.timestamp}] [${LogLevel[entry.level]}] ${entry.message}`
    );

    return {
      logs: formattedLogs,
      totalCount: filteredLogs.length
    };
  }

  /**
   * Close the log stream
   */
  close(): void {
    this.logStream.end();
  }

  error(message: string, ...args: any[]): void {
    this.log(LogLevel.ERROR, message, ...args);
  }

  info(message: string, ...args: any[]): void {
    this.log(LogLevel.INFO, message, ...args);
  }

  debug(message: string, ...args: any[]): void {
    this.log(LogLevel.DEBUG, message, ...args);
  }

  private log(level: LogLevel, message: string, ...args: any[]): void {
    const now = new Date();
    const timestamp = now.toTimeString().split(' ')[0]; // HH:MM:SS format
    const levelName = LogLevel[level];

    // Format the full message
    let fullMessage = message;
    if (args.length > 0) {
      const formattedArgs = args.map(arg =>
        typeof arg === 'object' ? JSON.stringify(arg) : String(arg)
      ).join(' ');
      fullMessage += ' ' + formattedArgs;
    }

    // Add to recent logs
    this.addToRecentLogs({
      level: level,
      timestamp: timestamp,
      message: fullMessage
    });

    // Only write to file if the message level is at or below the configured level
    if (level <= this.level) {
      // Format as "Aug 14 15:13:12.080" using locale string
      const dateStr = now.toLocaleDateString('en-US', { month: 'short', day: '2-digit' });
      const timeStr = now.toLocaleTimeString('en-US', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });
      const ms = now.getMilliseconds().toString().padStart(3, '0');
      const fullTimestamp = `${dateStr} ${timeStr}.${ms}`;
      let fileLogMessage = `[${fullTimestamp}] [${levelName}] ${message}`;
      if (args.length > 0) {
        const formattedArgs = args.map(arg =>
          typeof arg === 'object' ? JSON.stringify(arg) : String(arg)
        ).join(' ');
        fileLogMessage += ' ' + formattedArgs;
      }

      this.logStream.write(fileLogMessage + "\n");
      // Force flush to ensure logs are immediately visible on disk
      this.logStream.cork();
      this.logStream.uncork();
    }
  }

  private addToRecentLogs(entry: LogEntry): void {
    this.recentLogs.push(entry);

    // Maintain circular buffer size
    if (this.recentLogs.length > this.maxRecentLogs) {
      this.recentLogs.shift();
    }
  }
}

class ClangdDaemon {
  private projectRoot: string;
  private socketPath: string;
  private lockFilePath: string;
  private logFilePath: string;
  private server: net.Server | null = null;
  private clangdClient: ClangdClient | null = null;
  private startTime: number = Date.now();
  private requestCount: number = 0;
  private lastRequestTime: number = Date.now();
  private idleTimer: NodeJS.Timeout | null = null;
  private idleTimeoutMs: number;
  private connections: Set<net.Socket> = new Set();
  private logLevel: LogLevel;
  private requestLogBuffer: string[] | null = null; // Buffer for request-specific logs
  private fileWatcher: FileWatcher | null = null;
  private logger!: DaemonLogger; // Daemon-wide logger instance (initialized in constructor)

  constructor(projectRoot: string) {
    this.projectRoot = path.resolve(projectRoot);
    this.socketPath = generateSocketPath(this.projectRoot);
    this.lockFilePath = generateLockFilePath(this.projectRoot);
    this.logFilePath = getLogFilePath(this.projectRoot);

    // Get idle timeout from environment or use default (30 minutes)
    const timeoutSeconds = parseInt(process.env.CLANGD_DAEMON_TIMEOUT || "1800", 10);
    this.idleTimeoutMs = timeoutSeconds * 1000;

    // Set default log level to INFO
    // This captures important events without too much noise
    this.logLevel = LogLevel.INFO;

    // Create the daemon-wide logger (non-null from this point on)
    this.logger = new DaemonLogger(this.logFilePath, this.logLevel);
  }




  private initializeLogging(): void {
    this.logger.info(`Daemon starting for project: ${this.projectRoot}`);
    this.logger.info(`PID: ${process.pid}`);
    this.logger.info(`Socket path: ${this.socketPath}`);
    this.logger.info(`Idle timeout: ${this.idleTimeoutMs / 1000} seconds`);
  }

  private resetIdleTimer(): void {
    if (this.idleTimer) {
      clearTimeout(this.idleTimer);
    }

    this.idleTimer = setTimeout(() => {
      this.logger.info("Idle timeout reached, shutting down");
      this.shutdown().catch((error) => {
        this.logger.error("Error during idle shutdown", error);
        process.exit(1);
      });
    }, this.idleTimeoutMs);
  }

  private async checkExistingDaemon(): Promise<boolean> {
    // Clean up stale lock files first
    const wasStale = cleanupStaleLockFile(this.lockFilePath);
    if (wasStale) {
      this.logger.info("Cleaned up stale lock file");
    }

    // Check if lock file still exists
    const lockData = readLockFile(this.lockFilePath);
    if (lockData) {
      this.logger.error(`Another daemon is already running (PID: ${lockData.pid})`);
      return true;
    }

    return false;
  }

  private createLockFile(): void {
    const lockData: LockFileData = {
      pid: process.pid,
      socketPath: this.socketPath,
      startTime: this.startTime,
      projectRoot: this.projectRoot,
      buildTimestamp: calculateBuildTimestamp(import.meta.url),
    };

    writeLockFile(this.lockFilePath, lockData);
    this.logger.info("Lock file created");
  }

  private removeLockFile(): void {
    try {
      fs.unlinkSync(this.lockFilePath);
      this.logger.info("Lock file removed");
    } catch (error) {
      this.logger.error("Failed to remove lock file", error);
    }
  }

  private removeSocketFile(): void {
    try {
      if (fs.existsSync(this.socketPath)) {
        fs.unlinkSync(this.socketPath);
        this.logger.info("Socket file removed");
      }
    } catch (error) {
      this.logger.error("Failed to remove socket file", error);
    }
  }

  private async initializeClangd(): Promise<void> {
    this.logger.info("Initializing clangd");

    this.clangdClient = new ClangdClient(this.projectRoot, {
      clangdPath: process.env.CLANGD_PATH,
      logger: this.logger!,
    });

    await this.clangdClient.start();
    this.logger.info("Clangd initialized successfully");
  }

  /**
   * Handle file change events from the file watcher
   */
  private async handleFileChanges(changes: FileEvent[]): Promise<void> {
    if (!this.clangdClient) {
      this.logger.error("Cannot handle file changes: clangd client not initialized");
      return;
    }

    try {
      // Convert our FileEvent type to LSP FileEvent type
      const lspFileEvents = changes.map(change => ({
        uri: change.uri,
        type: change.type,
      }));

      await this.clangdClient.sendFileChangeNotification(lspFileEvents);

      this.logger.info(`Notified clangd about ${changes.length} file changes`);
      this.logger.debug("File change details:", lspFileEvents);

      // Check if compile_commands.json changed
      const hasCompileCommandsChanged = changes.some(
        change => change.uri.endsWith("/compile_commands.json")
      );

      if (hasCompileCommandsChanged) {
        this.logger.info("compile_commands.json changed - clangd should reindex the project");
        // clangd should handle this automatically when notified
      }
    } catch (error) {
      this.logger.error("Failed to notify clangd about file changes", error);
    }
  }

  /**
   * Initialize the file watcher
   */
  private async initializeFileWatcher(): Promise<void> {
    this.logger.info("Initializing file watcher");

    this.fileWatcher = new FileWatcher({
      projectRoot: this.projectRoot,
      onFileChanges: (changes) => this.handleFileChanges(changes),
      logger: {
        error: (msg, ...args) => this.logger.error(msg, ...args),
        info: (msg, ...args) => this.logger.info(msg, ...args),
        debug: (msg, ...args) => this.logger.debug(msg, ...args),
      },
      debounceMs: 500,
    });

    await this.fileWatcher.start();
    this.logger.info("File watcher initialized successfully");
  }

  private async handleRequest(request: JsonRpcRequest): Promise<JsonRpcResponse> {
    this.requestCount++;
    this.lastRequestTime = Date.now();
    this.resetIdleTimer();

    // Initialize request log buffer to capture all logs during this request
    this.requestLogBuffer = [];
    const requestLogger = new RequestLogger(this.requestLogBuffer);

    const response: JsonRpcResponse = {
      jsonrpc: "2.0",
      id: request.id,
    };

    try {
      switch (request.method) {
        case "searchSymbols": {
          const { query, limit } = request.params || {};
          if (!query) {
            throw new Error("Missing required parameter: query");
          }
          const result = await commands.searchSymbolsAsText(this.clangdClient!, query, limit, requestLogger);
          response.result = { text: result };
          break;
        }

        case "viewSourceCode": {
          const { query } = request.params || {};
          if (!query) {
            throw new Error("Missing required parameter: query");
          }
          const result = await commands.viewSourceCodeAsText(this.clangdClient!, query, requestLogger);
          response.result = { text: result };
          break;
        }

        case "findReferences": {
          const { location } = request.params || {};
          if (!location) {
            throw new Error("Missing required parameter: location");
          }
          const result = await commands.findReferencesAsText(this.clangdClient!, location, requestLogger);
          response.result = { text: result };
          break;
        }

        case "findReferencesToSymbol": {
          const { symbolName } = request.params || {};
          if (!symbolName) {
            throw new Error("Missing required parameter: symbolName");
          }
          const result = await commands.findReferencesToSymbolAsText(this.clangdClient!, symbolName, requestLogger);
          response.result = { text: result };
          break;
        }

        case "getTypeHierarchy": {
          const { className } = request.params || {};
          if (!className) {
            throw new Error("Missing required parameter: className");
          }
          const result = await commands.getTypeHierarchyAsText(this.clangdClient!, className, requestLogger);
          response.result = { text: result };
          break;
        }

        case "getSignature": {
          const { functionName } = request.params || {};
          if (!functionName) {
            throw new Error("Missing required parameter: functionName");
          }
          const result = await commands.getSignatureAsText(this.clangdClient!, functionName, requestLogger);
          response.result = { text: result };
          break;
        }

        case "getInterface": {
          const { className } = request.params || {};
          if (!className) {
            throw new Error("Missing required parameter: className");
          }
          const result = await commands.getInterfaceAsText(this.clangdClient!, className, requestLogger);
          response.result = { text: result };
          break;
        }

        case "getShow": {
          const { symbol } = request.params || {};
          if (!symbol) {
            throw new Error("Missing required parameter: symbol");
          }
          const result = await commands.getShowAsText(this.clangdClient!, symbol, requestLogger);
          response.result = { text: result };
          break;
        }

        case "getContext": {
          // Keep for backward compatibility but redirect to show
          const { symbol } = request.params || {};
          if (!symbol) {
            throw new Error("Missing required parameter: symbol");
          }
          const result = await commands.getShowAsText(this.clangdClient!, symbol, requestLogger);
          response.result = { text: result };
          break;
        }

        case "ping": {
          response.result = { status: "ok", timestamp: Date.now() };
          break;
        }

        case "getStatus": {
          const memUsage = process.memoryUsage();
          const status: DaemonStatus = {
            uptime: Date.now() - this.startTime,
            projectRoot: this.projectRoot,
            memory: {
              heapUsed: `${Math.round(memUsage.heapUsed / 1024 / 1024)}MB`,
              heapTotal: `${Math.round(memUsage.heapTotal / 1024 / 1024)}MB`,
              rss: `${Math.round(memUsage.rss / 1024 / 1024)}MB`,
            },
            requestCount: this.requestCount,
            lastRequestTime: this.lastRequestTime,
            indexingComplete: true, // We wait for indexing during startup
          };
          response.result = status;
          break;
        }

        case "getRecentLogs": {
          const lines = request.params?.lines || 100;
          const requestedLevel = request.params?.logLevel || "debug";

          // Get filtered logs from logger
          const result = this.logger.getFilteredLogs(requestedLevel, lines);

          response.result = {
            logs: result.logs,
            totalCount: result.totalCount,
            returnedCount: result.logs.length
          };
          break;
        }

        case "shutdown": {
          response.result = { status: "shutting down" };
          // Schedule shutdown after sending response
          setTimeout(() => {
            this.shutdown().catch((error) => {
              this.logger.error("Error during shutdown", error);
              process.exit(1);
            });
          }, 100);
          break;
        }

        default:
          response.error = {
            code: -32601,
            message: `Method not found: ${request.method}`,
          };
      }
    } catch (error) {
      this.logger.error(`Error handling request ${request.method}`, error);
      response.error = {
        code: -32000,
        message: error instanceof Error ? error.message : "Unknown error",
        data: error instanceof Error ? { stack: error.stack } : undefined,
      };
    }

    // Include captured logs in the response
    if (this.requestLogBuffer !== null && this.requestLogBuffer.length > 0) {
      response.logs = this.requestLogBuffer;
    }

    // Clear request log buffer
    this.requestLogBuffer = null;

    return response;
  }

  private handleConnection(socket: net.Socket): void {
    this.logger.debug("New client connected");
    this.connections.add(socket);

    let buffer = "";

    socket.on("data", async (data) => {
      buffer += data.toString();

      // Process complete JSON messages (newline-delimited)
      let lines = buffer.split("\n");
      buffer = lines.pop() || ""; // Keep incomplete line in buffer

      for (const line of lines) {
        if (!line.trim()) continue;

        try {
          const request = JSON.parse(line) as JsonRpcRequest;
          this.logger.debug(`Received request: ${request.method}`, request.params);

          const response = await this.handleRequest(request);
          socket.write(JSON.stringify(response) + "\n");

          this.logger.debug(`Sent response for request ${request.id}`);
        } catch (error) {
          this.logger.error("Failed to parse request", error);
          const errorResponse: JsonRpcResponse = {
            jsonrpc: "2.0",
            id: 0,
            error: {
              code: -32700,
              message: "Parse error",
            },
          };
          socket.write(JSON.stringify(errorResponse) + "\n");
        }
      }
    });

    socket.on("error", (error) => {
      this.logger.error("Socket error", error);
    });

    socket.on("close", () => {
      this.logger.debug("Client disconnected");
      this.connections.delete(socket);
    });
  }

  async start(): Promise<void> {
    // Initialize logging first
    this.initializeLogging();

    // Check for existing daemon
    if (await this.checkExistingDaemon()) {
      throw new Error("Another daemon is already running");
    }

    // Create lock file
    this.createLockFile();

    // Clean up socket file if it exists
    this.removeSocketFile();

    // Initialize clangd
    await this.initializeClangd();

    // Initialize file watcher after clangd is ready
    await this.initializeFileWatcher();

    // Create socket server
    this.server = net.createServer((socket) => this.handleConnection(socket));

    // Set socket permissions (owner read/write only)
    this.server.listen(this.socketPath, () => {
      fs.chmodSync(this.socketPath, 0o600);
      this.logger.info(`Daemon listening on ${this.socketPath}`);
    });

    // Start idle timer
    this.resetIdleTimer();

    // Handle graceful shutdown
    process.on("SIGTERM", () => {
      this.logger.info("Received SIGTERM, shutting down");
      this.shutdown().catch((error) => {
        this.logger.error("Error during shutdown", error);
        process.exit(1);
      });
    });

    process.on("SIGINT", () => {
      this.logger.info("Received SIGINT, shutting down");
      this.shutdown().catch((error) => {
        this.logger.error("Error during shutdown", error);
        process.exit(1);
      });
    });

    // Handle uncaught errors
    process.on("uncaughtException", (error) => {
      this.logger.error("Uncaught exception", error);
      this.shutdown().then(() => {
        process.exit(1);
      });
    });

    process.on("unhandledRejection", (reason, promise) => {
      this.logger.error("Unhandled rejection", reason);
      this.shutdown().then(() => {
        process.exit(1);
      });
    });
  }

  async shutdown(): Promise<void> {
    this.logger.info("Starting shutdown sequence");

    // Clear idle timer
    if (this.idleTimer) {
      clearTimeout(this.idleTimer);
      this.idleTimer = null;
    }

    // Close all connections
    for (const socket of this.connections) {
      socket.destroy();
    }
    this.connections.clear();

    // Close server
    if (this.server) {
      await new Promise<void>((resolve) => {
        this.server!.close(() => {
          this.logger.info("Server closed");
          resolve();
        });
      });
    }

    // Stop file watcher
    if (this.fileWatcher) {
      await this.fileWatcher.stop();
      this.logger.info("File watcher stopped");
    }

    // Stop clangd
    if (this.clangdClient) {
      await this.clangdClient.stop();
      this.logger.info("Clangd stopped");
    }

    // Remove lock file and socket
    this.removeLockFile();
    this.removeSocketFile();

    // Log final message before closing logger
    this.logger.info("Shutdown complete");

    // Close logger
    this.logger.close();

    process.exit(0);
  }
}

// Main entry point
async function main() {
  const projectRoot = process.argv[2];

  if (!projectRoot) {
    console.error("Usage: clangd-daemon <project-root>");
    process.exit(1);
  }

  const daemon = new ClangdDaemon(projectRoot);

  try {
    await daemon.start();
  } catch (error) {
    console.error("Failed to start daemon:", error);
    process.exit(1);
  }
}

// Run if executed directly
if (import.meta.url === `file://${process.argv[1]}`) {
  main().catch((error) => {
    console.error("Fatal error:", error);
    process.exit(1);
  });
}