/**
 * Socket utilities for clangd-query client-server architecture
 * 
 * This module provides shared utilities for managing Unix domain sockets and lock files
 * used by both the clangd-query client and clangd-daemon server. It handles:
 * - Generating consistent socket and lock file paths based on project root
 * - Reading and writing lock files with daemon metadata
 * - Detecting and cleaning up stale lock files from crashed daemons
 * - Process existence checking via signal 0
 */

import * as crypto from "node:crypto";
import * as fs from "node:fs";
import * as path from "node:path";
import * as os from "node:os";
import { fileURLToPath } from "node:url";

/**
 * Interface for the lock file contents
 */
export interface LockFileData {
  /** Process ID of the daemon */
  pid: number;
  /** Path to the Unix domain socket */
  socketPath: string;
  /** When the daemon was started */
  startTime: number;
  /** Absolute path to the project root */
  projectRoot: string;
  /** Timestamp of newest .js file in dist/ when daemon started */
  buildTimestamp?: number;
}

/**
 * Generate socket path from project root using MD5 hash
 * @param projectRoot Absolute path to the project root
 * @returns Path to the Unix domain socket
 */
export function generateSocketPath(projectRoot: string): string {
  const hash = crypto.createHash("md5").update(projectRoot).digest("hex");
  return path.join(os.tmpdir(), `clangd-daemon-${hash}.sock`);
}

/**
 * Generate lock file path from project root
 * @param projectRoot Absolute path to the project root
 * @returns Path to the lock file
 */
export function generateLockFilePath(projectRoot: string): string {
  return path.join(projectRoot, ".clangd-query.lock");
}

/**
 * Read lock file and parse its contents
 * @param lockFilePath Path to the lock file
 * @returns Parsed lock file data or null if file doesn't exist or is invalid
 */
export function readLockFile(lockFilePath: string): LockFileData | null {
  try {
    if (!fs.existsSync(lockFilePath)) {
      return null;
    }

    const content = fs.readFileSync(lockFilePath, "utf-8");
    const data = JSON.parse(content) as LockFileData;

    // Validate required fields
    if (
      typeof data.pid !== "number" ||
      typeof data.socketPath !== "string" ||
      typeof data.startTime !== "number" ||
      typeof data.projectRoot !== "string"
    ) {
      return null;
    }

    return data;
  } catch (error) {
    // File exists but is corrupted or invalid JSON
    return null;
  }
}

/**
 * Write lock file with daemon information
 * @param lockFilePath Path to the lock file
 * @param data Lock file data to write
 */
export function writeLockFile(lockFilePath: string, data: LockFileData): void {
  const content = JSON.stringify(data, null, 2);
  
  // Write atomically by writing to temp file first
  const tempPath = `${lockFilePath}.tmp`;
  fs.writeFileSync(tempPath, content, { mode: 0o644 });
  fs.renameSync(tempPath, lockFilePath);
}

/**
 * Check if a process with given PID is running
 * @param pid Process ID to check
 * @returns true if process is running, false otherwise
 */
export function isProcessRunning(pid: number): boolean {
  try {
    // Send signal 0 to check if process exists
    // This doesn't actually send a signal, just checks if we can
    process.kill(pid, 0);
    return true;
  } catch (error: any) {
    // ESRCH means no such process
    if (error.code === "ESRCH") {
      return false;
    }
    // EPERM means process exists but we don't have permission
    // This shouldn't happen for our own daemon, but handle it
    if (error.code === "EPERM") {
      return true;
    }
    // Other errors are unexpected
    throw error;
  }
}

/**
 * Clean up stale lock files where the process is no longer running
 * @param lockFilePath Path to the lock file
 * @returns true if lock file was cleaned up, false if it's still valid
 */
export function cleanupStaleLockFile(lockFilePath: string): boolean {
  const lockData = readLockFile(lockFilePath);
  
  if (!lockData) {
    // No lock file or invalid lock file
    return false;
  }

  if (!isProcessRunning(lockData.pid)) {
    // Process is dead, clean up the lock file
    try {
      fs.unlinkSync(lockFilePath);
      
      // Also try to clean up the socket file if it exists
      if (fs.existsSync(lockData.socketPath)) {
        try {
          fs.unlinkSync(lockData.socketPath);
        } catch {
          // Ignore errors cleaning up socket file
        }
      }
      
      return true;
    } catch {
      // Ignore errors during cleanup
      return false;
    }
  }

  // Process is still running, lock file is valid
  return false;
}

/**
 * Get log file path for a project
 * @param projectRoot Absolute path to the project root
 * @returns Path to the log file
 */
export function getLogFilePath(projectRoot: string): string {
  // Use .cache/clangd-query directory in the project root for logs
  return path.join(projectRoot, ".cache", "clangd-query", "daemon.log");
}

/**
 * Calculate the build timestamp by finding the newest .js file in dist/
 * This is used to detect when the daemon code has been rebuilt and needs restart.
 * 
 * @param importMetaUrl The import.meta.url from the calling module (daemon or client)
 * @returns Timestamp of the newest .js file in milliseconds
 */
export function calculateBuildTimestamp(importMetaUrl: string): number {
  // ES modules don't have __dirname, so we need to calculate it
  const __filename = fileURLToPath(importMetaUrl);
  const __dirname = path.dirname(__filename);
  const distPath = __dirname;
  let newestTime = 0;
  
  function scanDirectory(dir: string): void {
    try {
      const entries = fs.readdirSync(dir, { withFileTypes: true });
      
      for (const entry of entries) {
        const fullPath = path.join(dir, entry.name);
        
        if (entry.isDirectory()) {
          // Recursively scan subdirectories
          scanDirectory(fullPath);
        } else if (entry.isFile() && entry.name.endsWith('.js')) {
          // Check modification time of .js files
          const stats = fs.statSync(fullPath);
          const mtime = stats.mtime.getTime();
          if (mtime > newestTime) {
            newestTime = mtime;
          }
        }
      }
    } catch (error) {
      // Ignore errors (e.g., permission issues)
    }
  }
  
  scanDirectory(distPath);
  return newestTime;
}