/**
 * File Watcher for clangd automatic reindexing
 *
 * This module monitors the project's C++ source files and notifies clangd about changes
 * so that it can maintain an up-to-date index. This is crucial for accurate code
 * intelligence features like symbol search, references, and type hierarchy.
 *
 * Why this is needed:
 * - When files are modified outside of clangd's editor context, it doesn't automatically
 *   detect these changes
 * - Without file watching, the index becomes stale and queries return outdated results
 * - This ensures that clangd's understanding of the codebase stays synchronized with
 *   the actual file system state
 *
 * Implementation notes:
 * - Uses chokidar v4 for efficient file system monitoring with minimal dependencies
 * - Filters to only watch C++ source files (*.cpp, *.cc, *.h, *.hpp, etc.)
 * - Respects .gitignore patterns and common build directories
 * - Debounces file change events to batch notifications and reduce overhead
 *
 * Reindexing workaround:
 * - The LSP workspace/didChangeWatchedFiles notification alone doesn't reliably trigger
 *   reindexing in clangd
 * - We work around this by forcibly closing and reopening changed files, which makes
 *   clangd re-read them from disk and update its index
 * - This workaround is implemented in ClangdClient.sendFileChangeNotification()
 */

import * as chokidar from "chokidar";
import * as path from "path";
import * as fs from "fs";
import { pathToFileURL } from "url";
import { Logger } from "./logger.js";

// LSP FileChangeType enum values
export enum FileChangeType {
  Created = 1,
  Changed = 2,
  Deleted = 3,
}

export interface FileEvent {
  uri: string;
  type: FileChangeType;
}

export interface FileWatcherOptions {
  /**
   * The project root directory to watch
   */
  projectRoot: string;

  /**
   * Callback function to be called when file changes are detected
   */
  onFileChanges: (changes: FileEvent[]) => void;

  /**
   * Logger instance for debugging
   */
  logger?: Logger;

  /**
   * Debounce delay in milliseconds (default: 500)
   */
  debounceMs?: number;
}

/**
 * FileWatcher manages file system monitoring for C++ source files and notifies
 * about changes via batched, debounced callbacks.
 */
export class FileWatcher {
  private watcher: chokidar.FSWatcher | null = null;
  private changedFilesBuffer: FileEvent[] = [];
  private debounceTimer: NodeJS.Timeout | null = null;
  private readonly projectRoot: string;
  private readonly onFileChanges: (changes: FileEvent[]) => void;
  private readonly logger?: Logger;
  private readonly debounceMs: number;

  // Common C++ file extensions (including the dot)
  private static readonly CPP_EXTENSIONS = new Set([
    ".cpp", ".cc", ".cxx", ".c",
    ".h", ".hpp", ".hh", ".hxx",
    ".C", ".H", // Some codebases use uppercase
  ]);

  constructor(options: FileWatcherOptions) {
    this.projectRoot = options.projectRoot;
    this.onFileChanges = options.onFileChanges;
    this.logger = options.logger;
    this.debounceMs = options.debounceMs ?? 500;
  }

  /**
   * Start watching for file changes
   */
  async start(): Promise<void> {
    if (this.watcher) {
      throw new Error("File watcher is already running");
    }


    // Create ignored callback for chokidar v4
    // This is called for every path, so we optimize for performance
    const ignoredCallback = (filePath: string, stats?: fs.Stats): boolean => {
      // Fast path: if stats say it's a file, check extension immediately
      if (stats?.isFile()) {
        return !this.isCppFile(filePath);
      }

      // For directories or when stats aren't available, check if path contains
      // ignored directory names. We check the full path for efficiency.
      // Common patterns to ignore:
      if (filePath.includes('/.git/') || 
          filePath.includes('/build/') || 
          filePath.includes('/node_modules/') ||
          filePath.includes('/.cache/') ||
          filePath.includes('/dist/') ||
          filePath.includes('/out/') ||
          filePath.includes('/CMakeFiles/') ||
          filePath.includes('/cmake-build-') ||
          /\/\.[^/]+\//.test(filePath)) { // Any hidden directory (e.g., /.vscode/)
        return true;
      }

      // If no stats and has extension, it's likely a file - check if it's C++
      const ext = path.extname(filePath);
      if (ext) {
        return !this.isCppFile(filePath);
      }

      // No extension and no stats - likely a directory, don't ignore
      return false;
    };

    this.logger?.info("Starting file watcher", {
      projectRoot: this.projectRoot,
      debounceMs: this.debounceMs,
    });

    this.watcher = chokidar.watch(this.projectRoot, {
      ignored: ignoredCallback,
      persistent: true,
      ignoreInitial: true, // Don't emit events for existing files on startup
      followSymlinks: false,
      awaitWriteFinish: {
        stabilityThreshold: 100,
        pollInterval: 100,
      },
    });

    // Set up event handlers
    (this.watcher as any)
      .on("add", (filePath: string) => this.onFileChange(filePath, FileChangeType.Created))
      .on("change", (filePath: string) => this.onFileChange(filePath, FileChangeType.Changed))
      .on("unlink", (filePath: string) => this.onFileChange(filePath, FileChangeType.Deleted))
      .on("error", (error: Error) => this.logger?.error("File watcher error", error));

    // Wait for initial scan to complete
    await new Promise<void>((resolve) => {
      (this.watcher as any).on("ready", () => {
        this.logger?.info("File watcher ready");
        resolve();
      });
    });
  }

  /**
   * Stop watching for file changes
   */
  async stop(): Promise<void> {
    if (this.watcher) {
      await this.watcher.close();
      this.watcher = null;
      this.logger?.info("File watcher stopped");
    }

    if (this.debounceTimer) {
      clearTimeout(this.debounceTimer);
      this.debounceTimer = null;
    }
  }

  /**
   * Check if a file has a C++ extension
   */
  private isCppFile(filePath: string): boolean {
    const ext = path.extname(filePath).toLowerCase();
    return FileWatcher.CPP_EXTENSIONS.has(ext);
  }

  /**
   * Handle a file change event
   */
  private onFileChange(filePath: string, eventType: FileChangeType): void {
    const absolutePath = path.isAbsolute(filePath)
      ? filePath
      : path.join(this.projectRoot, filePath);

    // Double-check that this is actually a C++ file
    if (!this.isCppFile(absolutePath)) {
      // Don't log for initial scan to reduce noise
      return;
    }

    const uri = pathToFileURL(absolutePath).toString();

    this.logger?.debug(`File ${FileChangeType[eventType].toLowerCase()}: ${filePath}`);

    // Add to buffer, avoiding duplicates
    const existingIndex = this.changedFilesBuffer.findIndex(
      (event) => event.uri === uri
    );

    if (existingIndex !== -1) {
      // Update existing event (latest change type wins)
      this.changedFilesBuffer[existingIndex].type = eventType;
    } else {
      this.changedFilesBuffer.push({ uri, type: eventType });
    }

    // Debounce the notification
    this.debounceSendNotification();
  }

  /**
   * Debounced function to send file change notifications
   */
  private debounceSendNotification(): void {
    if (this.debounceTimer) {
      clearTimeout(this.debounceTimer);
    }

    this.debounceTimer = setTimeout(() => {
      if (this.changedFilesBuffer.length > 0) {
        const changes = [...this.changedFilesBuffer];
        this.changedFilesBuffer = [];

        this.logger?.info(`Sending ${changes.length} file change notifications`);
        this.onFileChanges(changes);
      }
    }, this.debounceMs);
  }

  /**
   * Check if compile_commands.json has changed in the recent file events
   */
  hasCompileCommandsChanged(): boolean {
    return this.changedFilesBuffer.some((event) =>
      event.uri.endsWith("/compile_commands.json")
    );
  }
}