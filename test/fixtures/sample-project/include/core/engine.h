#ifndef CORE_ENGINE_H_
#define CORE_ENGINE_H_

#include <chrono>
#include <memory>
#include <optional>
#include <string>
#include <vector>

namespace game_engine {

class GameObject;
class RenderSystem;
class PhysicsSystem;
class InputSystem;

/**
 * @brief Main engine class that manages the game loop and systems
 * 
 * This is a singleton class that coordinates all engine subsystems
 * and manages the main game loop.
 */
class Engine {
 public:
  /**
   * @brief Gets the singleton instance
   * @return Reference to the engine instance
   */
  static Engine& GetInstance();

  // Delete copy/move constructors for singleton
  Engine(const Engine&) = delete;
  Engine& operator=(const Engine&) = delete;
  Engine(Engine&&) = delete;
  Engine& operator=(Engine&&) = delete;

  /**
   * @brief Initializes the engine with given configuration
   * @param config_file Optional path to configuration file
   * @return true if initialization succeeded
   */
  bool Initialize(std::optional<std::string> config_file = std::nullopt);

  /**
   * @brief Shuts down the engine and cleans up resources
   */
  void Shutdown();

  /**
   * @brief Runs the main game loop
   */
  void Run();

  /**
   * @brief Stops the game loop
   */
  void Stop() { running_ = false; }

  /**
   * @brief Checks if the engine is running
   * @return true if the game loop is active
   */
  bool IsRunning() const { return running_; }

  /**
   * @brief Gets the current frames per second
   * @return Current FPS value
   */
  float GetFPS() const { return fps_; }

  /**
   * @brief Gets the time since engine started
   * @return Time in seconds
   */
  float GetTime() const;

  /**
   * @brief Creates a new game object
   * @param name Name for the object
   * @return Shared pointer to the created object
   */
  std::shared_ptr<GameObject> CreateGameObject(const std::string& name);

  /**
   * @brief Destroys a game object
   * @param object The object to destroy
   */
  void DestroyGameObject(std::shared_ptr<GameObject> object);

  /**
   * @brief Gets all active game objects
   * @return Vector of game objects
   */
  const std::vector<std::shared_ptr<GameObject>>& GetGameObjects() const {
    return game_objects_;
  }

  // System accessors
  RenderSystem* GetRenderSystem() { return render_system_.get(); }
  PhysicsSystem* GetPhysicsSystem() { return physics_system_.get(); }
  InputSystem* GetInputSystem() { return input_system_.get(); }

 private:
  Engine() = default;
  ~Engine();

  void UpdateFPS(float delta_time);
  void CleanupDestroyedObjects();

  bool initialized_ = false;
  bool running_ = false;
  
  std::chrono::steady_clock::time_point start_time_;
  std::chrono::steady_clock::time_point last_frame_time_;
  
  float fps_ = 0.0f;
  float fps_update_timer_ = 0.0f;
  int frame_count_ = 0;

  std::vector<std::shared_ptr<GameObject>> game_objects_;
  std::vector<std::shared_ptr<GameObject>> objects_to_destroy_;

  std::unique_ptr<RenderSystem> render_system_;
  std::unique_ptr<PhysicsSystem> physics_system_;
  std::unique_ptr<InputSystem> input_system_;

  // Fixed timestep for physics
  static constexpr float kFixedTimeStep = 1.0f / 60.0f;
  float accumulator_ = 0.0f;
};

}  // namespace game_engine

#endif  // CORE_ENGINE_H_