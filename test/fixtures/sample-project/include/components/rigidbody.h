#ifndef COMPONENTS_RIGIDBODY_H_
#define COMPONENTS_RIGIDBODY_H_

#include "core/component.h"
#include "core/transform.h"

namespace game_engine {

// Component that adds physics behavior to a game object
class Rigidbody : public Component {
 public:
  Rigidbody();
  ~Rigidbody() override;

  // Physics properties
  void SetMass(float mass) { mass_ = mass; }
  float GetMass() const { return mass_; }

  void SetVelocity(const Vector3& velocity) { velocity_ = velocity; }
  const Vector3& GetVelocity() const { return velocity_; }

  void SetAngularVelocity(const Vector3& angular_velocity) {
    angular_velocity_ = angular_velocity;
  }
  const Vector3& GetAngularVelocity() const { return angular_velocity_; }

  // Applies a force to the rigidbody
  void AddForce(const Vector3& force) {
    force_accumulator_ += force;
  }

  // Applies an impulse (instant velocity change)
  void AddImpulse(const Vector3& impulse) {
    if (mass_ > 0.0f) {
      velocity_ += impulse / mass_;
    }
  }

  // Sets whether this rigidbody is affected by gravity
  void SetUseGravity(bool use_gravity) { use_gravity_ = use_gravity; }
  bool GetUseGravity() const { return use_gravity_; }

  // Sets whether this is a kinematic body (controlled by code, not physics)
  void SetKinematic(bool kinematic) { kinematic_ = kinematic; }
  bool IsKinematic() const { return kinematic_; }

  // Component update
  void OnUpdate(float delta_time) override;

 private:
  float mass_ = 1.0f;
  Vector3 velocity_{};
  Vector3 angular_velocity_{};
  Vector3 force_accumulator_{};
  
  bool use_gravity_ = true;
  bool kinematic_ = false;
  
  // Drag coefficients
  float linear_drag_ = 0.1f;
  float angular_drag_ = 0.1f;
};

}  // namespace game_engine

#endif  // COMPONENTS_RIGIDBODY_H_