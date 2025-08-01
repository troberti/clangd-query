#include "patterns/factory.h"

#include "components/mesh_renderer.h"
#include "components/rigidbody.h"
#include "game/enemy.h"
#include "rendering/camera.h"

namespace game_engine {

// Register default enemy types
EnemyFactory::EnemyFactory() {
  // Register zombie creator
  Register("zombie", []() {
    return std::make_unique<Enemy>("Zombie", Enemy::EnemyType::Zombie);
  });
  
  // Register skeleton creator
  Register("skeleton", []() {
    return std::make_unique<Enemy>("Skeleton", Enemy::EnemyType::Skeleton);
  });
  
  // Register dragon creator
  Register("dragon", []() {
    return std::make_unique<Enemy>("Dragon", Enemy::EnemyType::Dragon);
  });
  
  // Register boss creator
  Register("boss", []() {
    return std::make_unique<Enemy>("Boss", Enemy::EnemyType::Boss);
  });
}

// Static initialization to register component types
namespace {
  struct ComponentRegistrar {
    ComponentRegistrar() {
      auto& factory = ComponentFactory::GetInstance();
      
      factory.Register("MeshRenderer", []() {
        return std::make_unique<MeshRenderer>();
      });
      
      factory.Register("Rigidbody", []() {
        return std::make_unique<Rigidbody>();
      });
      
      factory.Register("Camera", []() {
        return std::make_unique<Camera>();
      });
    }
  };
  
  // This will run at program startup
  static ComponentRegistrar registrar;
}

}  // namespace game_engine