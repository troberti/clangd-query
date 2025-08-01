#ifndef CORE_GAME_OBJECT_H_
#define CORE_GAME_OBJECT_H_

#include <memory>
#include <optional>
#include <string>
#include <vector>

#include "core/transform.h"
#include "core/interfaces.h"

namespace game_engine {

class Component;

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

  /**
   * @brief Gets the object's unique identifier
   * @return The object's ID
   */
  uint64_t GetId() const { return id_; }

  /**
   * @brief Gets the object's name
   * @return The object's name
   */
  const std::string& GetName() const { return name_; }

  /**
   * @brief Sets the object's active state
   * @param active Whether the object should be active
   */
  void SetActive(bool active) { active_ = active; }

  /**
   * @brief Sets the object's visibility
   * @param visible Whether the object should be visible
   */
  void SetVisible(bool visible) { visible_ = visible; }

  /**
   * @brief Adds a component to this game object
   * @param component The component to add
   */
  void AddComponent(std::shared_ptr<Component> component);

  /**
   * @brief Gets a component by type
   * @tparam T The component type to retrieve
   * @return Optional containing the component if found
   */
  template <typename T>
  std::optional<std::shared_ptr<T>> GetComponent() const;

  /**
   * @brief Gets the object's transform
   * @return Reference to the transform
   */
  Transform& GetTransform() { return transform_; }
  const Transform& GetTransform() const { return transform_; }

  // Three-way comparison operator
  auto operator<=>(const GameObject& other) const {
    return id_ <=> other.id_;
  }

  bool operator==(const GameObject& other) const {
    return id_ == other.id_;
  }

 protected:
  /**
   * @brief Called when the object is first created
   * 
   * Override this to perform initialization logic.
   */
  virtual void OnCreate() {}

  /**
   * @brief Called when the object is about to be destroyed
   * 
   * Override this to perform cleanup logic.
   */
  virtual void OnDestroy() {}

  /**
   * @brief Called during the update cycle
   * 
   * Override this to add custom update logic.
   */
  virtual void OnUpdate(float delta_time) { (void)delta_time; }

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

// Template implementation
template <typename T>
std::optional<std::shared_ptr<T>> GameObject::GetComponent() const {
  for (const auto& component : components_) {
    if (auto typed_component = std::dynamic_pointer_cast<T>(component)) {
      return typed_component;
    }
  }
  return std::nullopt;
}

}  // namespace game_engine

#endif  // CORE_GAME_OBJECT_H_