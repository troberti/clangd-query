import { spawn } from 'child_process';
import * as path from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// Path to the clangd-query executable
const CLANGD_QUERY = path.join(__dirname, '..', 'bin', 'clangd-query');

// Path to the sample project
const SAMPLE_PROJECT = path.join(__dirname, 'fixtures', 'sample-project');

/**
 * Runs a clangd-query command and returns the output
 * @param {string[]} args - Command arguments
 * @param {Object} options - Options
 * @returns {Promise<{stdout: string, stderr: string, exitCode: number}>}
 */
export async function runClangdQuery(args, options = {}) {
  const cwd = options.cwd || SAMPLE_PROJECT;
  const timeout = options.timeout || 30000;

  return new Promise((resolve, reject) => {
    const child = spawn('node', [CLANGD_QUERY, ...args], {
      cwd,
      timeout,
    });

    let stdout = '';
    let stderr = '';

    child.stdout.on('data', (data) => {
      stdout += data.toString();
    });

    child.stderr.on('data', (data) => {
      stderr += data.toString();
    });

    child.on('close', (exitCode) => {
      resolve({ stdout, stderr, exitCode });
    });

    child.on('error', (error) => {
      reject(error);
    });
  });
}

/**
 * Simple assertion helper
 */
export function assert(condition, message) {
  if (!condition) {
    throw new Error(`Assertion failed: ${message}`);
  }
}

/**
 * Check if output contains expected text
 */
export function assertContains(output, expected, message) {
  assert(
    output.includes(expected),
    message || `Expected output to contain "${expected}"\nActual output:\n${output}`
  );
}

/**
 * Check if output matches regex
 */
export function assertMatches(output, regex, message) {
  assert(
    regex.test(output),
    message || `Expected output to match ${regex}\nActual output:\n${output}`
  );
}

/**
 * Count occurrences of a string
 */
export function countOccurrences(str, substr) {
  return str.split(substr).length - 1;
}

/**
 * Wait for daemon to be ready (for first test)
 */
export async function waitForDaemonReady() {
  // Try a simple status command with extended timeout
  const result = await runClangdQuery(['status'], { timeout: 60000 });
  return result.exitCode === 0;
}

/**
 * Shutdown daemon after tests
 */
export async function shutdownDaemon() {
  try {
    await runClangdQuery(['shutdown']);
  } catch (e) {
    // Ignore errors during shutdown
  }
}