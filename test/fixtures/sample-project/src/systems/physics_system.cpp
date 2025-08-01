#include "systems/physics_system.h"

#include <algorithm>
#include <cmath>
#include <format>
#include <iostream>

namespace game_engine {

PhysicsSystem::PhysicsSystem() = default;
PhysicsSystem::~PhysicsSystem() = default;

bool PhysicsSystem::Initialize() {
  if (initialized_) {
    return true;
  }

  std::cout << std::format("Physics system initialized with gravity: {}\n",
                          gravity_.ToString());
  
  initialized_ = true;
  return true;
}

void PhysicsSystem::Shutdown() {
  if (!initialized_) {
    return;
  }

  colliders_.clear();
  collisions_.clear();
  
  std::cout << "Physics system shut down\n";
  initialized_ = false;
}

void PhysicsSystem::Update(float delta_time) {
  if (!initialized_) {
    return;
  }

  // Clear previous frame's collisions
  collisions_.clear();

  // Detect collisions
  DetectCollisions();

  // Resolve collisions
  ResolveCollisions();

  // Notify collision callbacks
  if (collision_callback_) {
    for (const auto& collision : collisions_) {
      collision_callback_(collision);
    }
  }

  // Update physics simulation
  (void)delta_time;  // Would update positions/velocities here
}

void PhysicsSystem::RegisterCollider(Collider* collider) {
  if (collider) {
    colliders_.push_back(collider);
  }
}

void PhysicsSystem::UnregisterCollider(Collider* collider) {
  auto it = std::find(colliders_.begin(), colliders_.end(), collider);
  if (it != colliders_.end()) {
    colliders_.erase(it);
  }
}

std::optional<CollisionInfo> PhysicsSystem::Raycast(
    const Vector3& origin,
    const Vector3& direction,
    float max_distance) const {
  
  // Simple raycast implementation placeholder
  (void)origin;
  (void)direction;
  (void)max_distance;
  
  // In a real implementation, this would test ray against all colliders
  return std::nullopt;
}

void PhysicsSystem::DetectCollisions() {
  // Simple brute force collision detection
  for (size_t i = 0; i < colliders_.size(); ++i) {
    for (size_t j = i + 1; j < colliders_.size(); ++j) {
      // In a real implementation, would check if colliders[i] and 
      // colliders[j] are colliding and add to collisions_ vector
    }
  }
}

void PhysicsSystem::ResolveCollisions() {
  // Resolve all detected collisions
  for (auto& collision : collisions_) {
    // In a real implementation, would separate overlapping objects
    // and apply collision response forces
    (void)collision;
  }
}

}  // namespace game_engine