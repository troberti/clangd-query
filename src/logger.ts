/**
 * Simple logger interface for capturing logs during request processing.
 */
export interface Logger {
  error(message: string, ...args: any[]): void;
  info(message: string, ...args: any[]): void;
  debug(message: string, ...args: any[]): void;
}