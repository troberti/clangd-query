import {
  createMessageConnection,
  StreamMessageReader,
  StreamMessageWriter,
  MessageConnection,
  RequestType,
} from "vscode-jsonrpc/node.js";
import {
  InitializeRequest,
  InitializeParams,
  DefinitionRequest,
  DeclarationRequest,
  ReferencesRequest,
  WorkspaceSymbolRequest,
  TextDocumentIdentifier,
  Position,
  Location,
  SymbolInformation,
  SymbolKind,
  ReferenceParams,
  TextDocumentPositionParams,
  WorkspaceSymbolParams,
  DidOpenTextDocumentNotification,
  DidOpenTextDocumentParams,
  DidCloseTextDocumentNotification,
  DidCloseTextDocumentParams,
  TextDocumentItem,
  HoverRequest,
  HoverParams,
  Hover,
  MarkupContent,
  MarkupKind,
  DocumentSymbolRequest,
  DocumentSymbolParams,
  DocumentSymbol,
  FoldingRangeRequest,
  FoldingRangeParams,
  FoldingRange,
  FoldingRangeKind,
  TypeHierarchyPrepareRequest,
  TypeHierarchyPrepareParams,
  TypeHierarchySupertypesRequest,
  TypeHierarchySupertypesParams,
  TypeHierarchySubtypesRequest,
  TypeHierarchySubtypesParams,
  TypeHierarchyItem,
  DidChangeWatchedFilesNotification,
  DidChangeWatchedFilesParams,
  FileEvent as LSPFileEvent,
} from "vscode-languageserver-protocol";
import { execa, ExecaChildProcess } from "execa";
import * as path from "path";
import * as fs from "fs";
import { pathToFileURL } from "url";
import { ensureCompileCommands } from "./compilation-db.js";
import { Logger } from "./logger.js";

export interface ClangdClientOptions {
  /** Path to the clangd executable. Defaults to "clangd" in PATH. */
  clangdPath?: string;
  /** Logger instance for all clangd client operations */
  logger?: Logger;
}

/**
 * Structured representation of parsed documentation from clangd hover responses.
 * This interface captures all the useful information extracted from the markdown
 * hover documentation in a structured format.
 */
export interface ParsedDocumentation {
  /**
   * The raw markdown content from clangd hover response.
   *
   * WARNING: This field is for debugging purposes only!
   * Do not use this field to implement parsing hacks or extract information.
   * All necessary information should be properly extracted into the other fields.
   * If information is missing, improve the getDocumentation() parsing logic instead.
   *
   * @internal For debugging only - not part of the stable API
   */
  _raw: string;

  /**
   * Cleaned description text without technical details like size/offset/parameters.
   * This is the human-readable documentation text.
   */
  description?: string;

  /**
   * For classes/structs: the inheritance chain (e.g., "public BaseClass, private Interface")
   * Extracted from class declarations in code blocks.
   */
  inheritance?: string;

  /**
   * Access level of the member/method within its containing class.
   * Determined by looking for access specifiers in code blocks.
   */
  accessLevel?: "public" | "private" | "protected";

  /**
   * For methods/functions: the complete signature including return type and parameters.
   * Example: "void setValue(int x, int y)"
   */
  signature?: string;

  /**
   * For fields/members: the type of the field.
   * Example: "const View*" or "std::vector<int>"
   */
  type?: string;

  /**
   * For fields with default values: the default initialization value.
   * Example: "nullptr" or "42" or "{1, 2, 3}"
   */
  defaultValue?: string;

  /**
   * Return type for methods/functions.
   * Extracted from the "→ Type" notation in hover documentation.
   */
  returnType?: string;

  /**
   * Raw parameter text for methods/functions if available.
   * This is the full "Parameters:" section as a single string.
   */
  parametersText?: string;

  /**
   * Method/function modifiers extracted from the signature.
   * Examples: "static", "virtual", "override", "const", "explicit", "inline", "noexcept"
   * Also includes special markers: "pure virtual" (for = 0), "deleted" (for = delete), "defaulted" (for = default)
   */
  modifiers?: string[];

  /**
   * Template parameters if this is a template function/method/class.
   * Example: "<typename T, typename U>" or "<class T, int N>"
   */
  templateParams?: string;
}

export class ClangdClient {
  private projectRoot: string;
  private clangdPath: string;
  private clangdProcess?: ExecaChildProcess;
  private connection?: MessageConnection;
  private logger!: Logger; // Always provided by daemon
  // Tracks which documents we've sent textDocument/didOpen notifications for.
  // This prevents redundant file reads and notifications when multiple LSP operations
  // are performed on the same file (e.g., getClassSummary needs to access the same file
  // dozens of times for documentation of each method).
  private openedDocuments = new Set<string>();

  // Indexing state tracking
  // Why: Clangd performs background indexing of the entire project when it starts.
  // Until indexing is complete, workspace-wide queries (like searchSymbols) will
  // return incomplete or empty results. We need to track the indexing state and
  // wait for it to complete before executing queries.
  //
  // indexingInProgress: True while clangd is actively indexing files
  // indexingComplete: True once indexing has finished (stays true for the lifetime of the client)
  // indexingPromise: A promise that resolves when indexing completes, allowing async waiting
  // indexingResolve: The resolver function for indexingPromise, called when indexing finishes
  // indexingTimeout: NodeJS timer for indexing timeout
  private indexingInProgress = false;
  private indexingComplete = false;
  private indexingPromise?: Promise<void>;
  private indexingResolve?: () => void;
  private indexingTimeout?: NodeJS.Timeout;

  /**
   * Creates a new ClangdClient instance.
   * @param projectRoot - The root directory of the C++ project (where CMakeLists.txt is located)
   * @param options - Optional configuration for the client
   * @throws Error if the project root does not exist
   */
  constructor(projectRoot: string, options: ClangdClientOptions = {}) {
    this.projectRoot = path.resolve(projectRoot);
    this.clangdPath = options.clangdPath || "clangd";
    this.logger = options.logger!;

    // Validate project root
    if (!fs.existsSync(this.projectRoot)) {
      throw new Error(`Project root does not exist: ${this.projectRoot}`);
    }
  }

  /**
   * Starts the clangd language server and initializes the LSP connection.
   * This will ensure compile_commands.json exists (generating it if needed) and
   * spawn the clangd process with background indexing enabled.
   * @throws Error if compile_commands.json cannot be ensured or clangd fails to start
   */
  async start(): Promise<void> {
    // Ensure compile_commands.json exists in .clangd-query/build
    let compileCommandsDir: string;
    try {
      compileCommandsDir = await ensureCompileCommands(this.projectRoot);
      this.logger.info(
        `Using compile_commands.json from: ${compileCommandsDir}`,
      );
    } catch (error) {
      throw new Error(`Failed to ensure compile_commands.json: ${error}`);
    }

    // Spawn clangd process pointing to our build directory
    // clangd will create its index at .clangd-query/build/.cache/clangd/index/
    this.clangdProcess = execa(
      this.clangdPath,
      [
        "--background-index", // Enable indexing of the entire project
        "--log=info", // Show info and error messages
        "--pretty",
        `--compile-commands-dir=${compileCommandsDir}`, // Point to our build directory
      ],
      {
        cwd: this.projectRoot,
        stdio: ["pipe", "pipe", "pipe"],
      },
    );

    // Create message connection
    const reader = new StreamMessageReader(this.clangdProcess.stdout!);
    const writer = new StreamMessageWriter(this.clangdProcess.stdin!);
    this.connection = createMessageConnection(reader, writer);

    // Capture clangd stderr
    let stderrBuffer = "";
    this.clangdProcess.stderr?.on("data", (chunk) => {
      stderrBuffer += chunk.toString();

      // Process complete lines
      const lines = stderrBuffer.split("\n");
      stderrBuffer = lines.pop() || ""; // Keep incomplete line in buffer

      for (const line of lines) {
        const trimmed = line.trim();
        if (!trimmed) continue;

        // Clean up clangd log format - extract just the message
        // Format is like: I[17:57:37.987] <-- workspace/symbol(1)
        const match = trimmed.match(/^([IVWED])\[[\d:\.]+\]\s+(.*)$/);
        if (match) {
          const level = match[1];
          const message = match[2];

          // Log at appropriate level based on clangd's level
          if (level === 'E') {
            this.logger.error(`[CLANGD] ${message}`);
          } else if (level === 'W') {
            this.logger.info(`[CLANGD] ${message}`);
          } else if (level === 'V' || level === 'D') {
            this.logger.debug(`[CLANGD] ${message}`);
          } else {
            this.logger.info(`[CLANGD] ${message}`);
          }
        } else {
          // For non-standard format, just log as-is
          this.logger.debug(`[CLANGD] ${trimmed}`);
        }
      }
    });

    // Handle window/workDoneProgress/create requests
    this.connection.onRequest(
      "window/workDoneProgress/create",
      (params: any) => {
        // Just acknowledge the request silently
        return null;
      },
    );

    // Handle progress notifications for logging and indexing state
    this.connection.onNotification("$/progress", (params: any) => {
      const { token, value } = params;

      if (value.kind === "begin") {
        // Check if this is background indexing
        if (token === "backgroundIndexProgress") {
          // Clear the timeout since we now have confirmation that indexing is happening
          // This prevents the timeout from firing and prematurely marking indexing as complete
          if (this.indexingTimeout) {
            clearTimeout(this.indexingTimeout);
            this.indexingTimeout = undefined;
          }
          this.logger.info(`Clangd started indexing: ${value.title}`);
        } else {
          this.logger.debug(`Progress begin [${token}]: ${value.title}`);
        }
      } else if (value.kind === "report") {
        if (token === "backgroundIndexProgress") {
          // Show indexing progress
          if (value.percentage !== undefined) {
            this.logger.info(`Indexing: ${value.percentage}%`);
          } else if (value.message) {
            this.logger.info(`Indexing: ${value.message}`);
          }
        } else {
          if (value.message) {
            this.logger.debug(`Progress [${token}]: ${value.message}`);
          }
          if (value.percentage !== undefined) {
            this.logger.debug(`Progress [${token}]: ${value.percentage}%`);
          }
        }
      } else if (value.kind === "end") {
        if (token === "backgroundIndexProgress") {
          this.indexingComplete = true;
          this.indexingInProgress = false;
          this.logger.info(`Clangd finished indexing`);
          // Resolve the indexing promise
          if (this.indexingResolve) {
            this.indexingResolve();
            this.indexingResolve = undefined;
          }
        } else {
          this.logger.debug(`Progress end [${token}]`);
        }
      }
    });

    // Start listening
    this.connection.listen();

    // CRITICAL: Initialize the indexing promise immediately to prevent race conditions
    //
    // Problem: Clangd performs background indexing asynchronously after startup. If a client
    // sends a query (like searchSymbols) before indexing completes, it will get empty or
    // incomplete results. This is especially problematic immediately after daemon startup.
    //
    // The race condition occurs because:
    // 1. Clangd only sends progress notifications AFTER indexing starts
    // 2. Indexing only starts AFTER we open the first document
    // 3. There's a window where neither indexingInProgress nor indexingComplete is true
    // 4. During this window, waitForIndexing() would return immediately without waiting
    //
    // Solution: Create the indexing promise immediately on startup, before any progress
    // notifications arrive. This ensures that waitForIndexing() will always wait for
    // either indexing completion or timeout, preventing empty results.
    this.indexingPromise = new Promise((resolve) => {
      this.indexingResolve = resolve;
    });
    this.indexingInProgress = true;

    // Set a timeout for indexing completion
    //
    // Why: Not all projects need indexing. Small projects or those with pre-built indexes
    // might never send backgroundIndexProgress notifications. Without this timeout, queries
    // would wait indefinitely. The 5-second timeout ensures responsiveness while giving
    // enough time for indexing to start on larger projects.
    this.indexingTimeout = setTimeout(() => {
      if (!this.indexingComplete && this.indexingResolve) {
        this.logger.info(
          "Indexing timeout - assuming indexing is not needed or already complete",
        );
        this.indexingComplete = true;
        this.indexingInProgress = false;
        this.indexingResolve();
        this.indexingResolve = undefined;
      }
    }, 5000);

    // Initialize LSP
    const initParams: InitializeParams = {
      processId: process.pid,
      rootUri: pathToFileURL(this.projectRoot).toString(),
      capabilities: {
        textDocument: {
          definition: {
            dynamicRegistration: false,
          },
          references: {
            dynamicRegistration: false,
          },
          hover: {
            dynamicRegistration: false,
            contentFormat: ["markdown", "plaintext"],
          },
          documentSymbol: {
            dynamicRegistration: false,
            hierarchicalDocumentSymbolSupport: true,
          },
        },
        workspace: {
          symbol: {
            dynamicRegistration: false,
            symbolKind: {
              valueSet: [
                1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18,
                19, 20, 21, 22, 23, 24, 25, 26,
              ],
            },
          },
          didChangeWatchedFiles: {
            dynamicRegistration: true,
            relativePatternSupport: true,
          },
        },
        window: {
          workDoneProgress: true, // This tells clangd we can handle progress notifications
        },
      },
      initializationOptions: {},
      workspaceFolders: [
        {
          uri: pathToFileURL(this.projectRoot).toString(),
          name: path.basename(this.projectRoot),
        },
      ],
    };

    try {
      const initResult = await this.sendRequestWithTimeout(
        InitializeRequest.type,
        initParams,
      );
      // Show clangd capabilities
      this.logger.debug(
        "Clangd initialized with capabilities:",
        JSON.stringify(initResult, null, 2),
      );

      // Log capabilities for debugging
      this.logger.info("=== CLANGD CAPABILITIES ===");
      this.logger.info(
        "Registered capabilities:",
        Object.keys(initResult.capabilities).join(", "),
      );

      // Send initialized notification to complete the LSP handshake
      // This tells clangd we've processed its capabilities and are ready to start
      await this.connection.sendNotification("initialized", {});

      // Open a source file to trigger indexing
      // Clangd requires at least one document to be opened before it starts background indexing.
      // Without this, workspace/symbol queries will return empty results.
      // See: https://github.com/clangd/clangd/discussions/1339
      const compileCommandsPath = path.join(
        compileCommandsDir,
        "compile_commands.json",
      );
      const firstSourceFile =
        await this.getFirstSourceFile(compileCommandsPath);

      if (firstSourceFile) {
        try {
          await this.ensureDocumentOpen(firstSourceFile);
        } catch (error) {
          this.logger.error(`Failed to open first source file: ${error}`);
        }
      } else {
        this.logger.error(
          `Warning: Could not find a source file to open from compile_commands.json`,
        );
      }
    } catch (error) {
      throw new Error(`Failed to initialize clangd: ${error}`);
    }
  }

  /**
   * Stops the clangd language server and closes the connection.
   * Safe to call multiple times.
   */
  async stop(): Promise<void> {
    // Clear any pending timeout
    if (this.indexingTimeout) {
      clearTimeout(this.indexingTimeout);
      this.indexingTimeout = undefined;
    }

    if (this.connection) {
      this.connection.dispose();
    }
    if (this.clangdProcess) {
      this.clangdProcess.kill();
    }
    this.openedDocuments.clear();
  }

  /**
   * Formats a location with relative path and 1-indexed line/column numbers.
   * @param absolutePath - The absolute path to the file
   * @param line - Line number (0-indexed)
   * @param column - Column number (0-indexed)
   * @returns Formatted string like "path/to/file.cpp:42:15"
   */
  formatLocation(absolutePath: string, line: number, column: number): string {
    const relativePath = this.toRelativePath(absolutePath);
    return `${relativePath}:${line + 1}:${column + 1}`;
  }

  /**
   * Formats a URI location as a relative path with line number.
   * @param uri - The file URI (e.g., "file:///path/to/file.cpp")
   * @param line - Line number (0-indexed)
   * @returns Formatted string like "path/to/file.cpp:42"
   */
  formatUriLocation(uri: string, line: number): string {
    const absolutePath = this.pathFromFileUri(uri);
    const relativePath = this.toRelativePath(absolutePath);
    return `${relativePath}:${line + 1}`;
  }

  /**
   * Gets the parsed documentation from clangd hover response at a specific location.
   * This extracts and structures all useful information from the hover response including
   * descriptions, types, signatures, default values, and access levels.
   *
   * @param file - Path to the file (relative to project root or absolute)
   * @param line - Line number (0-indexed)
   * @param column - Column number (0-indexed)
   * @returns Structured documentation data, or null if no documentation is available
   * @throws Error if the request fails or times out
   */
  async getDocumentation(
    file: string,
    line: number,
    column: number,
  ): Promise<ParsedDocumentation | null> {
    try {
      const hover = await this.getHoverRaw(file, line, column);
      const raw = this.extractDocumentationFromHover(hover);

      if (!raw) {
        return null;
      }

      const result: ParsedDocumentation = { _raw: raw };

      // Extract the documentation part between --- markers
      const docMatch = raw.match(/---\n([\s\S]*?)\n---/);
      if (docMatch && docMatch[1]) {
        let content = docMatch[1].trim();

        // Remove "Size: X bytes" line if present
        content = content.replace(/^Size:[^\n]*\n/, "");

        // Extract type information for fields
        const typeMatch = content.match(/^Type:\s*`([^`]+)`/m);
        if (typeMatch) {
          result.type = typeMatch[1];
        }

        // Extract description lines (skip technical details)
        const lines = content.split("\n");
        const description = [];
        let inParameters = false;
        let parametersText = "";

        for (const line of lines) {
          const trimmedLine = line.trim();

          // Skip return type indicators (→ type)
          if (trimmedLine.startsWith("→")) {
            let returnType = trimmedLine.substring(1).trim();
            // Remove backticks if present
            returnType = returnType.replace(/`([^`]+)`/, "$1");
            result.returnType = returnType;
            continue;
          }

          // Skip Type/Offset/Size/alignment information for members
          if (
            trimmedLine.startsWith("Type:") ||
            trimmedLine.startsWith("Offset:") ||
            trimmedLine.startsWith("Size:") ||
            trimmedLine.includes("alignment")
          )
            continue;

          // Handle Parameters section
          if (trimmedLine.startsWith("Parameters:")) {
            inParameters = true;
            parametersText = trimmedLine + "\n";
            continue;
          }

          // Collect parameter lines
          if (inParameters) {
            if (trimmedLine.startsWith("-")) {
              parametersText += "  " + trimmedLine + "\n";
            } else if (trimmedLine) {
              // End of parameters section
              inParameters = false;
              description.push(trimmedLine);
            }
          } else if (trimmedLine) {
            // Collect description lines
            description.push(trimmedLine);
          }
        }

        if (description.length > 0) {
          result.description = description.join(" ");
        }

        if (parametersText) {
          result.parametersText = parametersText.trim();
        }
      }

      // Extract code block information (supports any language identifier)
      const codeBlockMatch = raw.match(/```[a-zA-Z0-9_+-]*\n([^`]+)\n```/);
      if (codeBlockMatch) {
        const codeBlock = codeBlockMatch[1];

        // Extract access level
        const accessMatch = codeBlock.match(/^(public|private|protected):/m);
        if (accessMatch) {
          result.accessLevel = accessMatch[1] as
            | "public"
            | "private"
            | "protected";
        }

        // Extract inheritance for classes
        const inheritanceMatch = codeBlock.match(/class\s+\w+\s*:\s*([^{]+)\{/);
        if (inheritanceMatch) {
          result.inheritance = inheritanceMatch[1].trim();
        }

        // Extract method signature
        if (result.accessLevel) {
          // Try to match the full signature, including multi-line declarations
          // This regex captures from the access level until we find a ) not inside <>
          // First, let's try a simpler approach: match everything until the end of the code block
          const fullMethodMatch = codeBlock.match(
            new RegExp(`${result.accessLevel}:\\s*(.+)$`, "s"),
          );
          const sigMatch = fullMethodMatch;
          if (sigMatch) {
            let sig = sigMatch[1].trim();
            // Clean up multiline signatures by normalizing whitespace
            sig = sig.replace(/\s+/g, " ").trim();
            // Remove return type indicators
            sig = sig.replace(/^→\s*\S+\s*/, "");

            if (sig.includes("(") && sig.includes(")")) {
              result.signature = sig;
            }
          }
        }

        // For static methods, try to extract from the code block directly
        if (!result.signature && codeBlock.includes("static ")) {
          // Look for static method declarations
          const staticMatch = codeBlock.match(/static\s+[^;{]+/);
          if (staticMatch) {
            let sig = staticMatch[0].trim();
            // Clean up multiline signatures
            sig = sig.replace(/\s+/g, " ").trim();
            // Add semicolon if not present
            if (!sig.endsWith(";")) {
              sig += ";";
            }
            result.signature = sig;
          }
        }

        // For free functions (non-member functions), extract signature directly
        if (!result.signature && !result.accessLevel) {
          // Look for function-like patterns in the code block
          // Match template functions: template<...> returnType name(params)
          // or regular functions: returnType name(params)
          const functionMatch = codeBlock.match(/(?:template\s*<[^>]+>\s*)?(?:[\w:]+\s+)*(\w+)\s*\([^)]*\)(?:\s*const)?(?:\s*noexcept)?(?:\s*->\s*[\w:]+)?/);
          if (functionMatch) {
            let sig = functionMatch[0].trim();
            // Clean up multiline signatures
            sig = sig.replace(/\s+/g, " ").trim();
            // Remove template prefix if it exists (it's captured separately)
            if (sig.startsWith("template")) {
              sig = sig.replace(/^template\s*<[^>]+>\s*/, "");
            }
            result.signature = sig;
          }
        }

        // Extract default value for fields
        const defaultMatch = codeBlock.match(/(\w+)\s*=\s*([^;\n]+)/);
        if (defaultMatch) {
          result.defaultValue = defaultMatch[2].trim();
        }

        // Extract template parameters
        const templateMatch = codeBlock.match(/template\s*<([^>]+)>/);
        if (templateMatch) {
          result.templateParams = `<${templateMatch[1]}>`;

          // If we have a signature that starts with template, remove it
          if (result.signature && result.signature.startsWith("template")) {
            const withoutTemplate = result.signature.replace(
              /^template\s*<[^>]+>\s*/,
              "",
            );
            if (withoutTemplate !== result.signature) {
              result.signature = withoutTemplate;
            }
          }
        }

        // Extract method modifiers
        const modifiers: string[] = [];
        if (codeBlock.includes(" static ")) modifiers.push("static");
        if (codeBlock.includes(" virtual ")) modifiers.push("virtual");
        if (codeBlock.includes(" override")) modifiers.push("override");
        if (codeBlock.includes(" const ")) modifiers.push("const");
        if (codeBlock.includes(" explicit ")) modifiers.push("explicit");
        if (codeBlock.includes(" inline ")) modifiers.push("inline");
        if (codeBlock.includes(" noexcept")) modifiers.push("noexcept");
        if (codeBlock.includes(" = 0")) modifiers.push("pure virtual");
        if (codeBlock.includes(" = delete")) modifiers.push("deleted");
        if (codeBlock.includes(" = default")) modifiers.push("defaulted");

        if (modifiers.length > 0) {
          result.modifiers = modifiers;
        }
      }

      return result;
    } catch (error) {
      if (error instanceof Error && error.message === "Request timeout") {
        throw error;
      }
      throw new Error(`Failed to get documentation: ${error}`);
    }
  }

  // ========== Private Implementation ==========

  /**
   * Converts a Location object from clangd to a relative path format.
   * @param location - The Location object from clangd
   * @returns Object with file (relative path), line, and column
   */
  private locationToRelative(location: Location): any {
    const absolutePath = this.pathFromFileUri(location.uri);
    const relativePath = this.toRelativePath(absolutePath);

    return {
      file: relativePath,
      line: location.range.start.line,
      column: location.range.start.character,
    };
  }

  private ensureStarted(): void {
    if (!this.connection) {
      throw new Error(`Clangd client is not started`);
    }
  }

  /**
   * Waits for clangd to finish indexing the project.
   * This method ensures that workspace-wide queries (like searchSymbols) return complete results.
   *
   * The key insight is that we create the indexing promise immediately on startup, not when
   * we receive the first progress notification. This prevents a race condition where early
   * queries could bypass the wait if they arrive before indexing notifications.
   *
   * @returns Promise that resolves when indexing is complete or times out
   */
  private async waitForIndexing(): Promise<void> {
    // If indexing is already complete, return immediately
    if (this.indexingComplete) {
      return;
    }

    // Otherwise wait for the indexing promise to resolve
    // This promise is created immediately on startup and will resolve when:
    // 1. Indexing completes successfully (backgroundIndexProgress end notification), or
    // 2. The timeout expires (meaning indexing wasn't needed or is taking too long)
    //
    // This ensures that ALL queries wait appropriately, even if they arrive before
    // clangd has started indexing or sent any progress notifications.
    if (this.indexingPromise) {
      await this.indexingPromise;
    }
  }

  /**
   * Reads the first source file from compile_commands.json. We need this to
   * trigger indexing.
   *
   * @param compileCommandsPath - Path to the compile_commands.json file
   * @returns The path of the first source file found, or null if none found
   */
  private async getFirstSourceFile(
    compileCommandsPath: string,
  ): Promise<string | null> {
    try {
      const content = await fs.promises.readFile(compileCommandsPath, "utf-8");
      const commands = JSON.parse(content);

      if (!Array.isArray(commands) || commands.length === 0) {
        return null;
      }

      // Find the first .cc or .cpp file (skip headers)
      for (const entry of commands) {
        if (
          entry.file &&
          (entry.file.endsWith(".cc") || entry.file.endsWith(".cpp"))
        ) {
          return entry.file;
        }
      }

      // If no implementation files found, just use the first file
      return commands[0].file || null;
    } catch (error) {
      this.logger.error(`Failed to read compile_commands.json: ${error}`);
      return null;
    }
  }

  /**
   * Sends a request to clangd with a timeout.
   * @param type - The request type
   * @param params - The request parameters
   * @param timeoutMs - Timeout in milliseconds (default: 30 seconds)
   * @returns The response from clangd
   * @throws Error if the request times out or fails
   */
  private async sendRequestWithTimeout<P, R>(
    type: RequestType<P, R, any>,
    params: P,
    timeoutMs: number = 30000,
  ): Promise<R> {
    this.ensureStarted();

    return Promise.race([
      this.connection!.sendRequest(type, params),
      new Promise<R>((_, reject) =>
        setTimeout(() => reject(new Error("Request timeout")), timeoutMs),
      ),
    ]);
  }

  toAbsolutePath(relativePath: string): string {
    if (path.isAbsolute(relativePath)) {
      return relativePath;
    }
    return path.join(this.projectRoot, relativePath);
  }

  private toRelativePath(absolutePath: string): string {
    return path.relative(this.projectRoot, absolutePath);
  }

  private fileUriFromPath(filePath: string): string {
    const absolutePath = this.toAbsolutePath(filePath);
    return pathToFileURL(absolutePath).toString();
  }

  pathFromFileUri(uri: string): string {
    if (uri.startsWith("file://")) {
      // Properly decode file URI - this handles URL encoding like %2B for +
      const url = new URL(uri);
      return decodeURIComponent(url.pathname);
    }
    return uri;
  }

  /**
   * Ensures that a document is opened in clangd by sending a textDocument/didOpen notification.
   *
   * This method is critical for two main purposes:
   * 1. Triggering background indexing: Clangd won't start indexing until at least one file is opened
   * 2. Making file content available for analysis: Before calling findDefinition/findReferences on a file,
   *    it must be opened so clangd has access to its content
   *
   * The method reads the file content from disk and sends it to clangd via LSP protocol.
   * Once a file is opened, clangd will parse it and make its symbols available for queries.
   *
   * @param file - Path to the file (relative to project root or absolute)
   * @throws Error if the file cannot be read or if the notification fails
   */
  async ensureDocumentOpen(file: string): Promise<void> {
    const uri = this.fileUriFromPath(file);

    if (this.openedDocuments.has(uri)) {
      return; // Document already open, nothing to do
    }

    const absolutePath = this.toAbsolutePath(file);

    try {
      const content = await fs.promises.readFile(absolutePath, "utf-8");

      const params: DidOpenTextDocumentParams = {
        textDocument: {
          uri,
          languageId: file.endsWith(".h") ? "cpp" : "cpp",
          version: 1,
          text: content,
        },
      };

      await this.connection!.sendNotification(
        DidOpenTextDocumentNotification.type,
        params,
      );
      this.openedDocuments.add(uri);
    } catch (error) {
      throw new Error(`Failed to open document ${file}: ${error}`);
    }
  }

  /**
   * Sends file change notifications to clangd to trigger reindexing.
   * This should be called when external file changes are detected.
   *
   * This method handles the complete workflow for notifying clangd about file changes,
   * including a workaround for changed files where we force reindexing by closing
   * and reopening them (since clangd doesn't automatically reindex on
   * didChangeWatchedFiles notifications alone).
   *
   * @param fileEvents - Array of file events (created, changed, deleted)
   * @throws Error if the notification fails
   */
  async sendFileChangeNotification(fileEvents: LSPFileEvent[]): Promise<void> {
    this.ensureStarted();

    if (fileEvents.length === 0) {
      return; // Nothing to notify
    }

    const params: DidChangeWatchedFilesParams = {
      changes: fileEvents,
    };

    try {
      // Send the standard LSP notification - this is important for:
      // 1. Notifying about file creation/deletion (our workaround only handles changes)
      // 2. Following LSP spec which might trigger other internal clangd behavior
      // 3. Future compatibility if clangd improves its handling of these notifications
      await this.connection!.sendNotification(
        DidChangeWatchedFilesNotification.type,
        params,
      );

      this.logger.info(
        `Notified clangd about ${fileEvents.length} file changes`,
      );

      // WORKAROUND: For changed files (not created/deleted), force reindex
      // because clangd doesn't reliably reindex on didChangeWatchedFiles alone.
      // This is a known limitation where the notification doesn't trigger re-parsing
      // of files that aren't currently open in the editor context.
      const changedFiles = fileEvents.filter((event) => event.type === 2); // FileChangeType.Changed
      for (const event of changedFiles) {
        try {
          const filePath = new URL(event.uri).pathname;
          await this.forceReindexFile(filePath);
        } catch (error) {
          // Log but don't throw - we still want to process other files
          this.logger.error(`Failed to force reindex of ${event.uri}: ${error}`);
        }
      }
    } catch (error) {
      throw new Error(`Failed to send file change notification: ${error}`);
    }
  }

  /**
   * Forces clangd to reindex a file by closing and reopening it.
   * This is a workaround for when didChangeWatchedFiles doesn't trigger reindexing.
   *
   * @param filePath - Path to the file to reindex
   * @throws Error if the operation fails
   * @private
   */
  private async forceReindexFile(filePath: string): Promise<void> {
    this.ensureStarted();

    const uri = this.fileUriFromPath(filePath);

    // If the document is open, close it first
    if (this.openedDocuments.has(uri)) {
      const closeParams: DidCloseTextDocumentParams = {
        textDocument: {
          uri,
        },
      };

      await this.connection!.sendNotification(
        DidCloseTextDocumentNotification.type,
        closeParams,
      );
      this.openedDocuments.delete(uri);

      this.logger.info(`Closed document for reindexing: ${filePath}`);
    }

    // Now reopen it to force clangd to re-read from disk
    await this.ensureDocumentOpen(filePath);

    this.logger.info(`Forced reindex of: ${filePath}`);
  }

  /**
   * Gets the raw hover response from clangd for a symbol at the specified location.
   * This includes all fields that clangd returns, including potential AccessSpecifier.
   *
   * @param file - Path to the file (relative to project root or absolute)
   * @param line - Line number (0-indexed)
   * @param column - Column number (0-indexed)
   * @returns The raw hover response object, or null if no hover is available
   * @throws Error if the request fails or times out
   */
  private async getHoverRaw(
    file: string,
    line: number,
    column: number,
  ): Promise<any | null> {
    this.ensureStarted();

    await this.ensureDocumentOpen(file);

    const params: HoverParams = {
      textDocument: TextDocumentIdentifier.create(this.fileUriFromPath(file)),
      position: Position.create(line, column),
    };

    try {
      const result = await this.sendRequestWithTimeout(
        HoverRequest.type,
        params,
      );
      return result || null;
    } catch (error) {
      if (error instanceof Error && error.message === "Request timeout") {
        throw error;
      }
      throw new Error(`Failed to get hover: ${error}`);
    }
  }

  /**
   * Extracts documentation content from a hover response.
   * @param hover - The hover response from clangd
   * @returns Documentation content as a string, or null if no documentation
   */
  private extractDocumentationFromHover(hover: any): string | null {
    if (!hover || !hover.contents) {
      return null;
    }

    let documentation = "";

    if (MarkupContent.is(hover.contents)) {
      documentation = hover.contents.value;
    } else if (typeof hover.contents === "string") {
      documentation = hover.contents;
    } else if (Array.isArray(hover.contents)) {
      // Handle MarkedString[] format (legacy)
      documentation = hover.contents
        .map((item: any) => (typeof item === "string" ? item : item.value))
        .join("\n\n");
    }

    return documentation || null;
  }

  /**
   * Searches for symbols in the entire workspace matching the query.
   * @param query - The symbol name or pattern to search for
   * @param limit - Maximum number of results to return (default: 20)
   * @returns Array of symbol information including name, kind, and location
   * @throws Error if the request fails or times out
   */
  async searchSymbols(
    query: string,
    limit: number = 20,
    logger: Logger,
  ): Promise<SymbolInformation[]> {
    this.ensureStarted();

    await this.waitForIndexing();

    // Clangd supports a non-standard 'limit' parameter.
    // We use TypeScript intersection types (&) to add the optional limit property
    // to the standard WorkspaceSymbolParams type.
    const params: WorkspaceSymbolParams & { limit?: number } = {
      query,
    };

    // Add limit if specified
    if (limit > 0) {
      params.limit = limit;
    }

    logger.debug(`Sending workspace/symbol request with params:`, params);

    try {
      const result = await this.sendRequestWithTimeout(
        WorkspaceSymbolRequest.type,
        params,
      );

      logger.debug(`Symbol search raw result:`, result);

      if (!result || !Array.isArray(result)) {
        logger.debug(`Result is not an array or is null/undefined`);
        return [];
      }

      return result as SymbolInformation[];
    } catch (error) {
      logger.error(`Error during symbol search:`, error);
      if (error instanceof Error && error.message === "Request timeout") {
        throw error;
      }
      throw new Error(`Failed to search symbols: ${error}`);
    }
  }

  /**
   * Finds all references to a symbol at the specified location.
   * @param file - Path to the file (relative to project root or absolute)
   * @param line - Line number (0-indexed)
   * @param column - Column number (0-indexed)
   * @returns Array of locations where the symbol is referenced (includes the declaration)
   * @throws Error if the request fails or times out
   */
  async findReferences(
    file: string,
    line: number,
    column: number,
    logger: Logger,
  ): Promise<Location[]> {
    this.ensureStarted();

    // Wait for indexing to complete for accurate results
    await this.waitForIndexing();

    await this.ensureDocumentOpen(file);

    const params: ReferenceParams = {
      textDocument: TextDocumentIdentifier.create(this.fileUriFromPath(file)),
      position: Position.create(line, column),
      context: {
        includeDeclaration: true,
      },
    };

    try {
      const result = await this.sendRequestWithTimeout(
        ReferencesRequest.type,
        params,
      );

      if (!result || !Array.isArray(result)) {
        return [];
      }

      return result as Location[];
    } catch (error) {
      if (error instanceof Error && error.message === "Request timeout") {
        throw error;
      }
      throw new Error(`Failed to find references: ${error}`);
    }
  }

  /**
   * Find the definition location(s) of a symbol at a specific position.
   * For C++ this typically returns both declaration and definition locations.
   *
   * @param file - Path to the file (relative to project root or absolute)
   * @param line - Line number (0-indexed)
   * @param column - Column number (0-indexed)
   * @param logger - Logger instance for debugging
   * @returns Array of locations where the symbol is defined (may include both declaration and definition)
   * @throws Error if the request fails or times out
   */
  async getDefinition(
    file: string,
    line: number,
    column: number,
    logger: Logger,
  ): Promise<Location[]> {
    this.ensureStarted();

    // Wait for indexing to complete for accurate results
    await this.waitForIndexing();

    await this.ensureDocumentOpen(file);

    const params: TextDocumentPositionParams = {
      textDocument: TextDocumentIdentifier.create(this.fileUriFromPath(file)),
      position: Position.create(line, column),
    };

    try {
      const result = await this.sendRequestWithTimeout(
        DefinitionRequest.type,
        params,
      );

      if (!result) {
        return [];
      }

      // Result can be Location | Location[] | LocationLink[]
      if (Array.isArray(result)) {
        return result as Location[];
      } else {
        return [result as Location];
      }
    } catch (error) {
      if (error instanceof Error && error.message === "Request timeout") {
        throw error;
      }
      throw new Error(`Failed to find definition: ${error}`);
    }
  }

  /**
   * Find the declaration location(s) of a symbol at a specific position.
   * For C++ this typically returns the location in the header file where something is declared.
   *
   * @param file - Path to the file (relative to project root or absolute)
   * @param line - Line number (0-indexed)
   * @param column - Column number (0-indexed)
   * @param logger - Logger instance for debugging
   * @returns Array of locations where the symbol is declared
   * @throws Error if the request fails or times out
   */
  async getDeclaration(
    file: string,
    line: number,
    column: number,
    logger: Logger,
  ): Promise<Location[]> {
    this.ensureStarted();

    // Wait for indexing to complete for accurate results
    await this.waitForIndexing();

    await this.ensureDocumentOpen(file);

    const params: TextDocumentPositionParams = {
      textDocument: TextDocumentIdentifier.create(this.fileUriFromPath(file)),
      position: Position.create(line, column),
    };

    try {
      const result = await this.sendRequestWithTimeout(
        DeclarationRequest.type,
        params,
      );

      if (!result) {
        return [];
      }

      // Result can be Location | Location[] | LocationLink[]
      if (Array.isArray(result)) {
        return result as Location[];
      } else {
        return [result as Location];
      }
    } catch (error) {
      if (error instanceof Error && error.message === "Request timeout") {
        throw error;
      }
      throw new Error(`Failed to find declaration: ${error}`);
    }
  }

  /**
   * Gets folding ranges for a document. This returns ranges of foldable regions like
   * function bodies, class definitions, etc. This is useful for getting the full
   * extent of functions in implementation files.
   *
   * @param file - Path to the file (relative to project root or absolute)
   * @returns Array of folding ranges
   * @throws Error if the request fails or times out
   */
  async getFoldingRanges(
    file: string,
    logger: Logger,
  ): Promise<FoldingRange[]> {
    this.ensureStarted();

    await this.ensureDocumentOpen(file);

    const params: FoldingRangeParams = {
      textDocument: TextDocumentIdentifier.create(this.fileUriFromPath(file)),
    };

    try {
      const result = await this.sendRequestWithTimeout(
        FoldingRangeRequest.type,
        params,
      );

      if (!result || !Array.isArray(result)) {
        return [];
      }

      return result as FoldingRange[];
    } catch (error) {
      if (error instanceof Error && error.message === "Request timeout") {
        throw error;
      }
      throw new Error(`Failed to get folding ranges: ${error}`);
    }
  }

  /**
   * Gets all symbols in a document (functions, classes, methods, etc).
   * Returns hierarchical document symbols with full range information.
   *
   * @param file - Path to the file (relative to project root or absolute)
   * @returns Array of document symbols with ranges
   * @throws Error if the request fails or times out
   */
  async getDocumentSymbols(file: string): Promise<DocumentSymbol[]> {
    this.ensureStarted();

    await this.ensureDocumentOpen(file);

    const params: DocumentSymbolParams = {
      textDocument: TextDocumentIdentifier.create(this.fileUriFromPath(file)),
    };

    try {
      const result = await this.sendRequestWithTimeout(
        DocumentSymbolRequest.type,
        params,
      );

      if (!result || !Array.isArray(result)) {
        return [];
      }

      // Check if we got DocumentSymbol[] or SymbolInformation[]
      // DocumentSymbol has 'children' property, SymbolInformation doesn't
      if (result.length > 0 && "children" in result[0]) {
        return result as DocumentSymbol[];
      }

      // If we got SymbolInformation[], we can't get full ranges
      return [];
    } catch (error) {
      if (error instanceof Error && error.message === "Request timeout") {
        throw error;
      }
      throw new Error(`Failed to get document symbols: ${error}`);
    }
  }

  /**
   * Prepares type hierarchy information for a symbol at the given location.
   * This is the first step in getting type hierarchy - it identifies the symbol
   * and returns initial hierarchy items.
   * @param file - Path to the file
   * @param line - Line number (0-indexed)
   * @param column - Column number (0-indexed)
   * @returns Array of TypeHierarchyItem or null if not available
   */
  async prepareTypeHierarchy(
    file: string,
    line: number,
    column: number,
  ): Promise<TypeHierarchyItem[] | null> {
    this.ensureStarted();

    // Type hierarchy needs the index for cross-file relationships
    await this.waitForIndexing();

    await this.ensureDocumentOpen(file);

    const params: TypeHierarchyPrepareParams = {
      textDocument: TextDocumentIdentifier.create(this.fileUriFromPath(file)),
      position: Position.create(line, column),
    };

    try {
      const result = await this.sendRequestWithTimeout(
        TypeHierarchyPrepareRequest.type,
        params,
      );
      return result || null;
    } catch (error) {
      if (error instanceof Error && error.message === "Request timeout") {
        throw error;
      }
      throw new Error(`Failed to prepare type hierarchy: ${error}`);
    }
  }

  /**
   * Gets the supertypes (base classes) for a type hierarchy item.
   * @param item - The TypeHierarchyItem to get supertypes for
   * @returns Array of TypeHierarchyItem representing base classes
   */
  async getTypeHierarchySupertypes(
    item: TypeHierarchyItem,
  ): Promise<TypeHierarchyItem[]> {
    this.ensureStarted();

    const params: TypeHierarchySupertypesParams = { item };

    try {
      const result = await this.sendRequestWithTimeout(
        TypeHierarchySupertypesRequest.type,
        params,
      );
      return result || [];
    } catch (error) {
      if (error instanceof Error && error.message === "Request timeout") {
        throw error;
      }
      throw new Error(`Failed to get type hierarchy supertypes: ${error}`);
    }
  }

  /**
   * Gets the subtypes (derived classes) for a type hierarchy item.
   * @param item - The TypeHierarchyItem to get subtypes for
   * @returns Array of TypeHierarchyItem representing derived classes
   */
  async getTypeHierarchySubtypes(
    item: TypeHierarchyItem,
  ): Promise<TypeHierarchyItem[]> {
    this.ensureStarted();

    const params: TypeHierarchySubtypesParams = { item };

    try {
      const result = await this.sendRequestWithTimeout(
        TypeHierarchySubtypesRequest.type,
        params,
      );
      return result || [];
    } catch (error) {
      if (error instanceof Error && error.message === "Request timeout") {
        throw error;
      }
      throw new Error(`Failed to get type hierarchy subtypes: ${error}`);
    }
  }
}
