# clangd-query

Agents tend to have a hard time exploring C++ codebases and waste a lot tokens in their context window searching for declarations, definitions and source code. The header/source file structure of C-style languages does not help here.

This tool is designed to help agents explore C++ code bases. It is a fast command-line tool and returns output in human/agent-readable format. It outputs both source code and `file:line:column` locations which the agents can then use to easily continue editing.

`clangd-query` is a command-line tool and not an MCP, as agents seem to have an easier time using command-line tools. It uses a client/server architecture with the excellent clangd LSP to make it fast and keeps output to a minimum to save tokens.

## Examples

### Searching for Symbols

```bash
# Find the file+line position of a symbol
$ clangd-query search GameObject
Found 7 symbols matching "GameObject":

- `game_engine::GameObject` at include/core/game_object.h:26:7 [class]
- `game_engine::GameObject::GameObject` at src/core/game_object.cpp:12:13 [constructor]
- `game_engine::Engine::game_objects_` at include/core/engine.h:120:44 [field]
- `game_engine::Engine::CreateGameObject` at src/core/engine.cpp:136:37 [method]
- `game_engine::Engine::DestroyGameObject` at src/core/engine.cpp:142:14 [method]
- `game_engine::Engine::GetGameObjects` at include/core/engine.h:94:51 [method]
- `game_engine::GameObject::~GameObject` at src/core/game_object.cpp:17:13 [constructor]
```

### Show the complete source code of a Class or function

``````bash
# Show the full source code of a class
$ clangd-query show GameObject
Found class 'game_engine::GameObject' (7 matches total, showing most relevant)

From include/core/game_object.h:26:7
```cpp
/**
 * @brief Base class for all game objects in the engine
 *
 * GameObject represents any entity in the game world. It can contain
 * multiple components that define its behavior and properties.
 */
class GameObject : public Updatable,
                   public Renderable,
                   public std::enable_shared_from_this<GameObject> {
 public:
  explicit GameObject(const std::string& name);
  virtual ~GameObject();

  // Updatable interface
  void Update(float delta_time) override;
  bool IsActive() const override { return active_; }

  // Renderable interface
  void Render(float interpolation) override;
  int GetRenderPriority() const override { return render_priority_; }
  bool IsVisible() const override { return visible_; }

  <<<< Middle part omitted for brevity  >>>>

 private:
  static uint64_t next_id_;

  uint64_t id_;
  std::string name_;
  bool active_ = true;
  bool visible_ = true;
  int render_priority_ = 0;
  Transform transform_;
  std::vector<std::shared_ptr<Component>> components_;
};
```
``````


``````bash
# Shows the declaration AND definition of a function in one invocation, together
# with the file locations.
$ clangd-query show GameObject::Update
Found method 'game_engine::GameObject::Update' (2 matches total, showing most relevant)

From include/core/game_object.h:34:8 (declaration)
```cpp
  // Updatable interface
  void Update(float delta_time) override;
```

From src/core/game_object.cpp:21:18 (definition)
```cpp
void GameObject::Update(float delta_time) {
  if (!IsActive()) {
    return;
  }

  // Call virtual update method
  OnUpdate(delta_time);

  // Update all components
  for (auto& component : components_) {
    if (component->IsActive()) {
      component->Update(delta_time);
    }
  }
}
```
``````


### Viewing Class Hierarchies
```bash
# Show inheritance hierarchy for a class
$ clangd-query hierarchy GameObject
Inherits from:
├── Renderable - include/core/interfaces.h:41
└── Updatable - include/core/interfaces.h:18

GameObject - include/core/game_object.h:26
└── Character - include/game/character.h:9
    ├── Enemy - include/game/enemy.h:9
    └── Player - include/game/player.h:11
```

### Find Usages of a Symbol

```bash
# Find where a symbol is used in a code base
$ clangd-query usages Transform::Translate
Selected symbol: game_engine::Transform::Translate
Found 3 references:

- include/core/transform.h:84:8
- src/components/rigidbody.cpp:45:13
- src/game/character.cpp:47:18
```

## Reading the public interface of a class or struct
```bash
# View all public methods of a class and their comments
$ clangd-query interface RenderSystem
class game_engine::RenderSystem - include/systems/render_system.h:15:7

Public Interface:

RenderSystem()

~RenderSystem()

bool Initialize(int width, int height, const std::string& title)
  Initializes the render system with the specified window dimensions and title.
  Returns true if the render system was successfully initialized, false
  otherwise. This must be called before any rendering operations can be
  performed.

void Shutdown()
  Shuts down the render system and releases all associated resources. After
  calling this, the render system must be reinitialized before use.

void Render(float interpolation)
  Renders all registered renderable objects to the screen. The interpolation
  factor is used for smooth rendering between physics updates, allowing visual
  positions to be interpolated for smoother motion.

void RegisterRenderable(Renderable* renderable)
  Registers a renderable object with the system. Once registered, the object
  will be drawn during each render pass until it is unregistered.

void UnregisterRenderable(Renderable* renderable)
  Unregisters a renderable object from the system. The object will no longer be
  drawn in subsequent render passes.

void SetActiveCamera(std::shared_ptr<Camera> camera)
  Sets the camera that will be used for rendering. All renderable objects will
  be transformed and projected using this camera's view and projection matrices.

std::shared_ptr<Camera> GetActiveCamera() const
  Returns the currently active camera used for rendering. May return nullptr if
  no camera has been set.

int GetWindowWidth() const
  Returns the current window width in pixels.

int GetWindowHeight() const
  Returns the current window height in pixels.
```


## Requirements

- Go 1.21 or higher for building
- `clangd` must be installed on your system. Version 15+ recommended for full feature support
- CMake-based C++ project (for compile_commands.json generation).
- Your C++ project must have `CMakeLists.txt` at the project root. The tool automatically detects your C++ project by looking for `CMakeLists.txt` in parent directories. Run `clangd-query` from anywhere within your project tree.

## Installation

1. Clone this repository
2. Make sure you have Go 1.21+ installed
3. Build the binary:
   ```bash
   ./build.sh
   ```
4. The binary will be available at `bin/clangd-query`. Alternatively, you can use one of the prebuilt binaries in bin/releases (for macOS+Apple Silicon and Linux+Intel).
5. I highly recommend creating a `clangd-query` symlink in your project root to the compiled binary, then @-link your agents to the AGENT.md file in this repository for instructions on how to use the tool.

## Other Commands

```bash
# Check daemon status
clangd-query status

# Show all logs of the daemon. Use --verbose, --info (the default) or --error to
# filter on log entries.
clangd-query logs

# Quits the daemon process. This is not required as the daemon shutdown
# automatically after idling for too long.
clangd-query shutdown

# Shows help output of the tool.
clangd-query --help
```

### Technical Details

On first run, `clangd-query` starts a background daemon for your project. The tool looks for `CMakeLists.txt` the current directory and all its ancestor directories. The first one it finds is used as the project root. It then creates a `compile_commands.json` from the `CMakeLists.txt` and starts `clangd` to index the codebase.

Subsequent runs of the tool are fast as the daemon is already running. The daemon shuts down automatically after 30 minutes of being idle.

```
┌─────────────┐       JSON-RPC        ┌──────────────┐
│clangd-query ├──────────────────────►│clangd-daemon │
│  (client)   │◄──────────────────────┤  (server)    │
└─────────────┘    Unix Socket        └──────┬───────┘
                                             │
                                             ▼
                                       ┌──────────┐
                                       │  clangd  │
                                       │   LSP    │
                                       └──────────┘
```

#### Compilation Database

The tool will build a `compile_commands.json` from the `CMakeLists.txt`, which must be in the project root. This is used by clangd to index the codebase. The database is stored in `.cache/clangd-query/build/compile_commands.json`.

### Index
The clangd index is stored in `.cache/clangd-query/build/.cache/clangd`

### Lock Files

The daemon uses a lock file `<project-root>/.clangd-query.lock`. These are automatically cleaned when the daemon shuts down.

### Daemon Log File
Stored in  `.cache/clangd-query/daemon.log`. Can also be directly accessed using the `clangd-query logs` command as long as the daemon is running.

## License

MIT License - see [LICENSE](LICENSE) file for details.

## In Development

This tool is under active development and suggestions and feedback is welcome.

## Acknowledgments

Built on top of the excellent [clangd](https://clangd.llvm.org/) language server.
