#include "systems/render_system.h"

#include <algorithm>
#include <format>
#include <iostream>

#include "core/interfaces.h"

namespace game_engine {

RenderSystem::RenderSystem() = default;
RenderSystem::~RenderSystem() = default;

bool RenderSystem::Initialize(int width, int height, const std::string& title) {
  if (initialized_) {
    return true;
  }

  window_width_ = width;
  window_height_ = height;
  window_title_ = title;

  std::cout << std::format("Render system initialized: {}x{} - {}\n",
                          width, height, title);

  initialized_ = true;
  return true;
}

void RenderSystem::Shutdown() {
  if (!initialized_) {
    return;
  }

  renderables_.clear();
  active_camera_.reset();
  
  std::cout << "Render system shut down\n";
  initialized_ = false;
}

void RenderSystem::Render(float interpolation) {
  if (!initialized_) {
    return;
  }

  // Sort renderables if needed
  if (needs_sorting_) {
    SortRenderables();
    needs_sorting_ = false;
  }

  // Render all objects
  for (auto* renderable : renderables_) {
    if (renderable && renderable->IsVisible()) {
      renderable->Render(interpolation);
    }
  }
}

void RenderSystem::RegisterRenderable(Renderable* renderable) {
  if (renderable) {
    renderables_.push_back(renderable);
    needs_sorting_ = true;
  }
}

void RenderSystem::UnregisterRenderable(Renderable* renderable) {
  auto it = std::find(renderables_.begin(), renderables_.end(), renderable);
  if (it != renderables_.end()) {
    renderables_.erase(it);
  }
}

void RenderSystem::SortRenderables() {
  std::sort(renderables_.begin(), renderables_.end(),
            [](const Renderable* a, const Renderable* b) {
              return a->GetRenderPriority() < b->GetRenderPriority();
            });
}

}  // namespace game_engine