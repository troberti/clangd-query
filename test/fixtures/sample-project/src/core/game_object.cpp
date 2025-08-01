#include "core/game_object.h"

#include <algorithm>
#include <format>

#include "core/component.h"

namespace game_engine {

uint64_t GameObject::next_id_ = 1;

GameObject::GameObject(const std::string& name)
    : id_(next_id_++), name_(name) {
  OnCreate();
}

GameObject::~GameObject() {
  OnDestroy();
}

void GameObject::Update(float delta_time) {
  if (!IsActive()) {
    return;
  }

  // Call virtual update method
  OnUpdate(delta_time);

  // Update all components
  for (auto& component : components_) {
    if (component->IsActive()) {
      component->Update(delta_time);
    }
  }
}

void GameObject::Render(float interpolation) {
  if (!IsVisible()) {
    return;
  }

  // In a real engine, this would render the object
  // For now, this is just a placeholder
  (void)interpolation;
}

void GameObject::AddComponent(std::shared_ptr<Component> component) {
  if (component) {
    component->SetOwner(weak_from_this());
    components_.push_back(component);
  }
}

}  // namespace game_engine