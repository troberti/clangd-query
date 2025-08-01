#include "game/player.h"

#include <format>
#include <iostream>

#include "components/rigidbody.h"

namespace game_engine {

Player::Player(const std::string& name) : Character(name) {
  // Players have more health than regular characters
  SetMaxHealth(150);
  SetHealth(150);
}

Player::~Player() = default;

void Player::Jump() {
  if (!IsGrounded() || !IsAlive()) {
    return;
  }

  // Apply jump force using rigidbody if available
  if (auto rigidbody = GetComponent<Rigidbody>()) {
    if (auto rb = rigidbody.value()) {
      rb->AddImpulse(Vector3{0.0f, jump_force_, 0.0f});
      is_grounded_ = false;
    }
  }
}

void Player::OnCreate() {
  std::cout << std::format("Player '{}' created\n", GetName());
  
  // Add a rigidbody component
  auto rigidbody = std::make_shared<Rigidbody>();
  rigidbody->SetMass(70.0f);  // 70kg player
  AddComponent(rigidbody);
}

void Player::OnDestroy() {
  std::cout << std::format("Player '{}' destroyed\n", GetName());
  
  if (current_weapon_) {
    std::cout << std::format("Dropping weapon: {}\n", *current_weapon_);
  }
}

}  // namespace game_engine