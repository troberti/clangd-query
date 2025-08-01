#include "components/rigidbody.h"

#include <algorithm>

#include "core/game_object.h"
#include "core/engine.h"
#include "systems/physics_system.h"

namespace game_engine {

Rigidbody::Rigidbody() : Component("Rigidbody") {
}

Rigidbody::~Rigidbody() = default;

void Rigidbody::OnUpdate(float delta_time) {
  if (kinematic_) {
    // Kinematic bodies are controlled by code, not physics
    return;
  }

  auto owner = GetOwner().lock();
  if (!owner) {
    return;
  }

  // Apply gravity if enabled
  if (use_gravity_) {
    auto& physics = *Engine::GetInstance().GetPhysicsSystem();
    AddForce(physics.GetGravity() * mass_);
  }

  // Apply drag
  velocity_ = velocity_ * (1.0f - linear_drag_ * delta_time);
  angular_velocity_ = angular_velocity_ * (1.0f - angular_drag_ * delta_time);

  // Update velocity from forces (F = ma, so a = F/m)
  if (mass_ > 0.0f) {
    Vector3 acceleration = force_accumulator_ * (1.0f / mass_);
    velocity_ += acceleration * delta_time;
  }

  // Update position
  auto& transform = owner->GetTransform();
  transform.Translate(velocity_ * delta_time);

  // Update rotation
  transform.Rotate(angular_velocity_ * delta_time);

  // Clear force accumulator
  force_accumulator_ = Vector3{};
}

}  // namespace game_engine