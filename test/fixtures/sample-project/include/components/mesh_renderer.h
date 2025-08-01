#ifndef COMPONENTS_MESH_RENDERER_H_
#define COMPONENTS_MESH_RENDERER_H_

#include <memory>
#include <string>

#include "core/component.h"

namespace game_engine {

class Mesh;
class Material;

// Component that renders a 3D mesh
class MeshRenderer : public Component {
 public:
  MeshRenderer();
  ~MeshRenderer() override;

  // Sets the mesh to render
  void SetMesh(std::shared_ptr<Mesh> mesh) { mesh_ = mesh; }
  
  // Gets the current mesh
  std::shared_ptr<Mesh> GetMesh() const { return mesh_; }

  // Sets the material to use for rendering
  void SetMaterial(std::shared_ptr<Material> material) { 
    material_ = material; 
  }
  
  // Gets the current material
  std::shared_ptr<Material> GetMaterial() const { return material_; }

  // Component update logic
  void OnUpdate(float delta_time) override;

 private:
  std::shared_ptr<Mesh> mesh_;
  std::shared_ptr<Material> material_;
};

}  // namespace game_engine

#endif  // COMPONENTS_MESH_RENDERER_H_