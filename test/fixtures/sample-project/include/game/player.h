#ifndef GAME_PLAYER_H_
#define GAME_PLAYER_H_

#include <optional>

#include "game/character.h"

namespace game_engine {

// Represents the player character in the game
class Player : public Character {
 public:
  explicit Player(const std::string& name);
  ~Player() override;

  // Player-specific jump ability
  void Jump();
  float GetJumpForce() const { return jump_force_; }
  void SetJumpForce(float force) { jump_force_ = force; }

  // Player state
  bool IsGrounded() const { return is_grounded_; }
  void SetGrounded(bool grounded) { is_grounded_ = grounded; }

  // Optional equipment
  void SetWeapon(const std::string& weapon) { current_weapon_ = weapon; }
  std::optional<std::string> GetWeapon() const { return current_weapon_; }
  void ClearWeapon() { current_weapon_.reset(); }

 protected:
  void OnCreate() override;
  void OnDestroy() override;

 private:
  float jump_force_ = 10.0f;
  bool is_grounded_ = true;
  
  // Optional equipment or power-up
  std::optional<std::string> current_weapon_;
};

}  // namespace game_engine

#endif  // GAME_PLAYER_H_