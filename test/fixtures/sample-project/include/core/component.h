#ifndef CORE_COMPONENT_H_
#define CORE_COMPONENT_H_

#include <memory>
#include <string>

#include "core/interfaces.h"

namespace game_engine {

class GameObject;

/**
 * @brief Base class for all components
 * 
 * Components are modular pieces of functionality that can be
 * attached to GameObjects.
 */
class Component : public Updatable {
 public:
  explicit Component(const std::string& type_name)
      : type_name_(type_name) {}
  
  virtual ~Component() = default;

  /**
   * @brief Gets the component's type name
   * @return The type name string
   */
  const std::string& GetTypeName() const { return type_name_; }

  /**
   * @brief Gets the owning game object
   * @return Weak pointer to the owner
   */
  std::weak_ptr<GameObject> GetOwner() const { return owner_; }

  /**
   * @brief Sets the owning game object
   * @param owner The new owner
   */
  void SetOwner(std::weak_ptr<GameObject> owner) {
    owner_ = owner;
  }

  // Updatable interface
  void Update(float delta_time) override {
    if (IsActive()) {
      OnUpdate(delta_time);
    }
  }

  bool IsActive() const override { return enabled_; }

  /**
   * @brief Enables or disables the component
   * @param enabled Whether the component should be active
   */
  void SetEnabled(bool enabled) { enabled_ = enabled; }

 protected:
  /**
   * @brief Called when the component needs to update
   * @param delta_time Time since last update
   */
  virtual void OnUpdate(float delta_time) = 0;

 private:
  std::string type_name_;
  std::weak_ptr<GameObject> owner_;
  bool enabled_ = true;
};

}  // namespace game_engine

#endif  // CORE_COMPONENT_H_