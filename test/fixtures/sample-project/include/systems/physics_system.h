#ifndef SYSTEMS_PHYSICS_SYSTEM_H_
#define SYSTEMS_PHYSICS_SYSTEM_H_

#include <functional>
#include <optional>
#include <vector>

#include "core/transform.h"

namespace game_engine {

class Collider;

/**
 * @brief Simple collision information
 */
struct CollisionInfo {
  Collider* collider_a = nullptr;
  Collider* collider_b = nullptr;
  Vector3 contact_point;
  Vector3 contact_normal;
  float penetration_depth = 0.0f;
};

/**
 * @brief Manages physics simulation
 */
class PhysicsSystem {
 public:
  using CollisionCallback = std::function<void(const CollisionInfo&)>;

  PhysicsSystem();
  ~PhysicsSystem();

  /**
   * @brief Initializes the physics system
   * @return true if initialization succeeded
   */
  bool Initialize();

  /**
   * @brief Shuts down the physics system
   */
  void Shutdown();

  /**
   * @brief Updates the physics simulation
   * @param delta_time Time step for the simulation
   */
  void Update(float delta_time);

  /**
   * @brief Sets the global gravity
   * @param gravity Gravity vector (default is (0, -9.81, 0))
   */
  void SetGravity(const Vector3& gravity) { gravity_ = gravity; }

  /**
   * @brief Gets the current gravity
   * @return The gravity vector
   */
  const Vector3& GetGravity() const { return gravity_; }

  /**
   * @brief Registers a collider with the physics system
   * @param collider The collider to register
   */
  void RegisterCollider(Collider* collider);

  /**
   * @brief Unregisters a collider
   * @param collider The collider to unregister
   */
  void UnregisterCollider(Collider* collider);

  /**
   * @brief Sets the collision callback
   * @param callback Function to call when collisions occur
   */
  void SetCollisionCallback(CollisionCallback callback) {
    collision_callback_ = callback;
  }

  /**
   * @brief Performs a raycast
   * @param origin Ray origin
   * @param direction Ray direction (should be normalized)
   * @param max_distance Maximum ray distance
   * @return Optional collision info if hit occurred
   */
  std::optional<CollisionInfo> Raycast(
      const Vector3& origin,
      const Vector3& direction,
      float max_distance) const;

 private:
  void DetectCollisions();
  void ResolveCollisions();

  Vector3 gravity_{0.0f, -9.81f, 0.0f};
  std::vector<Collider*> colliders_;
  std::vector<CollisionInfo> collisions_;
  CollisionCallback collision_callback_;
  
  bool initialized_ = false;
};

}  // namespace game_engine

#endif  // SYSTEMS_PHYSICS_SYSTEM_H_