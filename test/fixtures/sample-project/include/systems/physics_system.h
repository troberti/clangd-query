#ifndef SYSTEMS_PHYSICS_SYSTEM_H_
#define SYSTEMS_PHYSICS_SYSTEM_H_

#include <functional>
#include <optional>
#include <vector>

#include "core/transform.h"

namespace game_engine {

class Collider;

// Contains information about a collision between two colliders, including
// the contact point, normal, and penetration depth.
struct CollisionInfo {
  Collider* collider_a = nullptr;
  Collider* collider_b = nullptr;
  Vector3 contact_point;
  Vector3 contact_normal;
  float penetration_depth = 0.0f;
};

// Manages the physics simulation for the game engine. This system handles
// collision detection, collision resolution, gravity, and raycasting.
// All physics objects must register their colliders with this system to
// participate in the simulation.
class PhysicsSystem {
 public:
  using CollisionCallback = std::function<void(const CollisionInfo&)>;

  PhysicsSystem();
  ~PhysicsSystem();

  // Initializes the physics system and prepares it for simulation.
  // Returns true if initialization was successful, false otherwise.
  // Must be called before any physics operations can be performed.
  bool Initialize();

  // Shuts down the physics system and releases all resources.
  // The system must be reinitialized before it can be used again.
  void Shutdown();

  // Advances the physics simulation by the specified time step. This method
  // applies forces, updates velocities and positions, detects collisions,
  // and resolves any collisions that occurred.
  void Update(float delta_time);

  // Sets the global gravity vector that affects all physics objects.
  // The default gravity is (0, -9.81, 0) which simulates Earth gravity
  // along the negative Y axis.
  void SetGravity(const Vector3& gravity) { gravity_ = gravity; }

  // Returns the current global gravity vector.
  const Vector3& GetGravity() const { return gravity_; }

  // Registers a collider with the physics system. Once registered, the
  // collider will participate in collision detection and resolution during
  // each physics update.
  void RegisterCollider(Collider* collider);

  // Removes a collider from the physics system. The collider will no longer
  // participate in collision detection or resolution.
  void UnregisterCollider(Collider* collider);

  // Sets a callback function that will be invoked whenever a collision
  // occurs between two colliders. This allows game logic to respond to
  // collision events.
  void SetCollisionCallback(CollisionCallback callback) {
    collision_callback_ = callback;
  }

  // Casts a ray from the specified origin in the given direction and returns
  // information about the first collision encountered, if any. The direction
  // vector should be normalized. The ray extends up to max_distance units.
  // Returns an empty optional if no collision was detected.
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