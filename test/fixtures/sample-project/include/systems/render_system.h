#ifndef SYSTEMS_RENDER_SYSTEM_H_
#define SYSTEMS_RENDER_SYSTEM_H_

#include <memory>
#include <string>
#include <vector>

namespace game_engine {

class Renderable;
class Camera;

// Manages rendering of all game objects in the scene. The render system maintains
// a list of renderable objects and draws them each frame using the active camera.
class RenderSystem {
 public:
  RenderSystem();
  ~RenderSystem();

  // Initializes the render system with the specified window dimensions and title.
  // Returns true if the render system was successfully initialized, false otherwise.
  // This must be called before any rendering operations can be performed.
  bool Initialize(int width, int height, const std::string& title);

  // Shuts down the render system and releases all associated resources.
  // After calling this, the render system must be reinitialized before use.
  void Shutdown();

  // Renders all registered renderable objects to the screen. The interpolation
  // factor is used for smooth rendering between physics updates, allowing visual
  // positions to be interpolated for smoother motion.
  void Render(float interpolation);

  // Registers a renderable object with the system. Once registered, the object
  // will be drawn during each render pass until it is unregistered.
  void RegisterRenderable(Renderable* renderable);

  // Unregisters a renderable object from the system. The object will no longer
  // be drawn in subsequent render passes.
  void UnregisterRenderable(Renderable* renderable);

  // Sets the camera that will be used for rendering. All renderable objects
  // will be transformed and projected using this camera's view and projection
  // matrices.
  void SetActiveCamera(std::shared_ptr<Camera> camera) {
    active_camera_ = camera;
  }

  // Returns the currently active camera used for rendering. May return nullptr
  // if no camera has been set.
  std::shared_ptr<Camera> GetActiveCamera() const {
    return active_camera_;
  }

  // Returns the current window width in pixels.
  int GetWindowWidth() const { return window_width_; }

  // Returns the current window height in pixels.
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