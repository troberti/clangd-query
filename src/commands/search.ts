import { Logger } from "../logger.js";
import { ClangdClient } from "../clangd-client.js";
import { SymbolInformation } from "vscode-languageserver-protocol";
import { formatSymbolWithType, formatMultiWordQueryHint } from "./utils.js";

/**
 * Searches for symbols in the entire workspace and returns formatted text output.
 * Uses fuzzy matching to find symbols - partial names and case variations will match.
 * Results are scored by relevance with the best matches returned first.
 *
 * Example output:
 * ```
 * Found 6 symbols matching "Manager":
 *
 * - `class flare::ResourceManager` at clients/flare/src/resource/resourcemanager.cc:45:7
 * - `flare::ResourceManager::Get()` at clients/flare/src/resource/resourcemanager.cc:123:14
 * - `scene::GameScene::Update()` at clients/src/phoenix2/scenes/gamescene.cpp:89:5
 * - `struct RenderData` at clients/src/phoenix2/rendering/renderdata.h:12:8
 * - `enum flare::BlendingMode` at clients/flare/include/flare/render/blending.h:8:6
 * - `namespace phoenix2` at clients/src/phoenix2/namespace.h:1:11
 * ```
 *
 * @param client - The ClangdClient instance
 * @param query - The symbol name or pattern to search for (fuzzy matching)
 * @param limit - Maximum number of results to return (default: 20)
 * @param logger - Logger for debug output
 * @returns Human-readable text with each symbol on its own line, sorted by relevance
 * @throws Error if the request fails or times out
 */
export async function searchSymbolsAsText(
  client: ClangdClient,
  query: string,
  limit: number = 20,
  logger: Logger
): Promise<string> {
  logger.info(`Searching for symbols matching: ${query} (limit: ${limit})`);
  const symbols = await client.searchSymbols(query, limit, logger);
  logger.debug(`Found ${symbols.length} symbols`);

  if (symbols.length === 0) {
    // Check if query has multiple words
    if (query.includes(' ')) {
      return formatMultiWordQueryHint(query, 'search') + 
             `\nThen use interface command to see all its methods and members.`;
    }
    return `No symbols found matching "${query}"`;
  }

  let output = `Found ${symbols.length} symbols matching "${query}":\n\n`;

  for (const symbol of symbols) {
    // Build the fully qualified name with type prefix
    const fullName = formatSymbolWithType(symbol);

    // Get relative path
    const absolutePath = client.pathFromFileUri(symbol.location.uri);
    const formattedLocation = client.formatLocation(
      absolutePath,
      symbol.location.range.start.line,
      symbol.location.range.start.character
    );

    // Format with bullet point, backticks, and "at" prefix
    output += `- \`${fullName}\` at ${formattedLocation}\n`;
  }

  // Remove trailing newline
  return output.trimEnd();
}

