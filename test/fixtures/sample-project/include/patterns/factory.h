#ifndef PATTERNS_FACTORY_H_
#define PATTERNS_FACTORY_H_

#include <functional>
#include <memory>
#include <string>
#include <unordered_map>
#include <vector>

#include "core/component.h"
#include "core/transform.h"
#include "game/enemy.h"

namespace game_engine {

// Factory pattern for creating game objects
template <typename Base>
class Factory {
 public:
  using Creator = std::function<std::unique_ptr<Base>()>;

  // Registers a creator function for a type
  void Register(const std::string& type_name, Creator creator) {
    creators_[type_name] = creator;
  }

  // Creates an instance of the specified type
  std::unique_ptr<Base> Create(const std::string& type_name) const {
    auto it = creators_.find(type_name);
    if (it != creators_.end()) {
      return it->second();
    }
    return nullptr;
  }

  // Checks if a type is registered
  bool IsRegistered(const std::string& type_name) const {
    return creators_.find(type_name) != creators_.end();
  }

  // Gets all registered type names
  std::vector<std::string> GetRegisteredTypes() const {
    std::vector<std::string> types;
    types.reserve(creators_.size());
    for (const auto& [type, creator] : creators_) {
      types.push_back(type);
    }
    return types;
  }

 private:
  std::unordered_map<std::string, Creator> creators_;
};

// Component factory
class ComponentFactory : public Factory<Component> {
 public:
  static ComponentFactory& GetInstance() {
    static ComponentFactory instance;
    return instance;
  }

 private:
  ComponentFactory() = default;
};

// Enemy factory
class EnemyFactory : public Factory<Enemy> {
 public:
  static EnemyFactory& GetInstance() {
    static EnemyFactory instance;
    return instance;
  }

  // Convenience method to create and configure an enemy
  std::shared_ptr<Enemy> CreateEnemy(const std::string& type_name,
                                     const std::string& name,
                                     const Vector3& position) {
    auto enemy = Create(type_name);
    if (enemy) {
      auto shared_enemy = std::shared_ptr<Enemy>(enemy.release());
      shared_enemy->GetTransform().SetPosition(position);
      return shared_enemy;
    }
    return nullptr;
  }

 private:
  EnemyFactory();  // Will register default enemy types
};

}  // namespace game_engine

#endif  // PATTERNS_FACTORY_H_