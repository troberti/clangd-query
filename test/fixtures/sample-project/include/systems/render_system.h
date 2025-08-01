#ifndef SYSTEMS_RENDER_SYSTEM_H_
#define SYSTEMS_RENDER_SYSTEM_H_

#include <memory>
#include <string>
#include <vector>

namespace game_engine {

class Renderable;
class Camera;

/**
 * @brief Manages rendering of all game objects
 */
class RenderSystem {
 public:
  RenderSystem();
  ~RenderSystem();

  /**
   * @brief Initializes the render system
   * @param width Window width
   * @param height Window height
   * @param title Window title
   * @return true if initialization succeeded
   */
  bool Initialize(int width, int height, const std::string& title);

  /**
   * @brief Shuts down the render system
   */
  void Shutdown();

  /**
   * @brief Renders all registered renderable objects
   * @param interpolation Interpolation factor for smooth rendering
   */
  void Render(float interpolation);

  /**
   * @brief Registers a renderable object
   * @param renderable The object to register
   */
  void RegisterRenderable(Renderable* renderable);

  /**
   * @brief Unregisters a renderable object
   * @param renderable The object to unregister
   */
  void UnregisterRenderable(Renderable* renderable);

  /**
   * @brief Sets the active camera
   * @param camera The camera to use for rendering
   */
  void SetActiveCamera(std::shared_ptr<Camera> camera) {
    active_camera_ = camera;
  }

  /**
   * @brief Gets the active camera
   * @return The current camera
   */
  std::shared_ptr<Camera> GetActiveCamera() const {
    return active_camera_;
  }

  /**
   * @brief Gets the window width
   * @return Width in pixels
   */
  int GetWindowWidth() const { return window_width_; }

  /**
   * @brief Gets the window height
   * @return Height in pixels
   */
  int GetWindowHeight() const { return window_height_; }

 private:
  void SortRenderables();

  int window_width_ = 0;
  int window_height_ = 0;
  std::string window_title_;
  
  std::vector<Renderable*> renderables_;
  std::shared_ptr<Camera> active_camera_;
  
  bool needs_sorting_ = false;
  bool initialized_ = false;
};

}  // namespace game_engine

#endif  // SYSTEMS_RENDER_SYSTEM_H_