# AGENT.md - AI Agent Instructions

This file provides instructions for AI agents using clangd-query to explore C++ codebases. If you are an AI agent, use this tool instead of grep/find for C++ code exploration.

## What is clangd-query?

clangd-query is a fast C++ code intelligence CLI tool designed specifically for AI agents. It provides semantic understanding of C++ code through the clangd language server, offering precise results without false positives from text-based searching.

## Key Benefits for AI Agents

1. **Token-optimized output** - Concise, relevant results without unnecessary context
2. **Semantic accuracy** - Understands C++ namespaces, overloads, templates, and inheritance
3. **Fast responses** - Persistent daemon keeps clangd warm for instant queries
4. **Show Source** - Show source code from classes and functions. For functions, it shows both declarations and definitions.

## Essential Commands

### `search` - Find symbols by name
```bash
clangd-query search GameObject
clangd-query search Update
clangd-query search Transform --limit 10
```
Use this to discover what exists in the codebase. Supports fuzzy matching.

**Important**: Search accepts only single symbol names. No regex patterns, wildcards, or multiple words (e.g., "Character Update" won't work). For compound searches, run multiple queries.

### `show` - See complete implementation
```bash
clangd-query show GameObject::Update
clangd-query show Transform
clangd-query show Engine
```
Shows the full source code. For methods, shows BOTH declaration (from .h) and definition (from .cpp).

### `usages` - Find all references
```bash
clangd-query usages GameObject
clangd-query usages Update
clangd-query usages src/game.cpp:45:10
```
Critical for understanding impact before making changes. Shows every place a symbol is used.

### `hierarchy` - Understand inheritance
```bash
clangd-query hierarchy Character
clangd-query hierarchy Component
```
Shows what a class inherits from and what inherits from it.

### `signature` - Get function signatures
```bash
clangd-query signature Update
clangd-query signature CreateGameObject
```
Shows all overloads with parameters and return types.

### `interface` - Public API only
```bash
clangd-query interface Engine
clangd-query interface GameObject
```
Shows only public methods and members - what users of the class can access.

## Best Practices for AI Agents

### 1. Start with search
When asked about something, first search for it:
```bash
clangd-query search Player
```

### 2. Use show for understanding
Once you find a symbol, use show to see its implementation:
```bash
clangd-query show Player
```

### 3. Check usages before changes
Before modifying anything, see where it's used:
```bash
clangd-query usages Player
```

### 4. Understand the hierarchy
For classes, check what they inherit and what inherits from them:
```bash
clangd-query hierarchy Player
```

## Common Patterns

### Understanding a class
```bash
clangd-query search GameObject       # Find it
clangd-query show GameObject         # See full implementation
clangd-query hierarchy GameObject    # See inheritance
clangd-query interface GameObject    # See public API
```

### Understanding a method
```bash
clangd-query search Update           # Find all Update methods
clangd-query show GameObject::Update # See specific implementation
clangd-query usages GameObject::Update # See where it's called
```

### Finding implementations
```bash
clangd-query search Update           # Find all Update methods
clangd-query search Factory          # Find factory classes
clangd-query search Create           # Find creation methods
```

## Search Limitations

The `search` command has important constraints:
- **Single words only** - `search Update` works, `search "Update Game"` does NOT
- **No regex** - No patterns like `Update.*` or `Get[A-Z]*`
- **No wildcards** - Cannot use `*`, `?`, or other glob patterns
- **No composite searches** - Cannot search for "classes with Update method"

For complex searches, combine multiple queries:
```bash
# Wrong: clangd-query search "Character Update"
# Right:
clangd-query search Character
clangd-query search Update
```

## Tips

1. **No quotes needed** - Just use the symbol name directly
2. **Use :: for specific methods** - `GameObject::Update` vs just `Update`
3. **Fuzzy matching works** - `search gamobj` finds GameObject
4. **Case sensitive** - C++ is case sensitive, so is the tool
5. **Check exit codes** - Non-zero means error

## Handling C++ Complexity

### Templates
```bash
clangd-query show GameObject::GetComponent  # Shows template implementation
clangd-query search Component               # Finds Component-related symbols
```

### Namespaces
```bash
clangd-query search game_engine::    # Find everything in namespace
clangd-query show GameObject         # Show a class from the namespace
```

### Overloads
```bash
clangd-query signature Update        # Shows all Update overloads
clangd-query show GameObject::Update # Shows specific overload
```

## Integration Tips

1. **Parse structured output** - Results have consistent format
2. **Handle "No symbols found"** - Common for typos or non-existent symbols
3. **Use --limit** - Control result count for large codebases
4. **Check daemon status** - Use `clangd-query status` if issues arise

## Example Workflow

User asks: "How does the player character work?"

```bash
# 1. Search for player-related classes
clangd-query search Player

# 2. Found Player class, let's see it
clangd-query show Player

# 3. Player inherits from Character, check that
clangd-query show Character

# 4. See what methods Player has
clangd-query interface Player

# 5. Check what components GameObject can have
clangd-query search Component
clangd-query interface Component
```

## Important Notes

- The tool requires a `CMakeLists.txt` file in the project root to create a `compile_commands.json` file.
- First run in a project may be slower while clangd indexes
- The daemon runs per-project and auto-manages itself. No need to explicitly shut it down.

Remember: This tool gives you semantic understanding of C++ code. Use it instead of grep/find for any C++ exploration tasks!