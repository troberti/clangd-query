#include "components/mesh_renderer.h"

#include <iostream>

namespace game_engine {

MeshRenderer::MeshRenderer() : Component("MeshRenderer") {
}

MeshRenderer::~MeshRenderer() = default;

void MeshRenderer::OnUpdate(float delta_time) {
  // In a real implementation, this might update animation states
  // or other rendering-related logic
  (void)delta_time;
  
  if (!mesh_ || !material_) {
    return;
  }
  
  // Would typically register with render system here
}

}  // namespace game_engine