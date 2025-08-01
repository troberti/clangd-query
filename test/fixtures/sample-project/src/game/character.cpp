#include "game/character.h"

#include <algorithm>
#include <format>
#include <iostream>

namespace game_engine {

Character::Character(const std::string& name) : GameObject(name) {
}

Character::~Character() = default;

int Character::TakeDamage(int damage) {
  if (damage <= 0 || !IsAlive()) {
    return 0;
  }

  int actual_damage = std::min(damage, health_);
  health_ -= actual_damage;

  if (!IsAlive()) {
    OnDeath();
  }

  return actual_damage;
}

int Character::Heal(int amount) {
  if (amount <= 0 || !IsAlive()) {
    return 0;
  }

  int actual_heal = std::min(amount, max_health_ - health_);
  health_ += actual_heal;
  
  return actual_heal;
}

void Character::Move(const Vector3& direction) {
  if (!IsActive() || !IsAlive()) {
    return;
  }

  // Calculate movement based on speed
  Vector3 movement = direction * move_speed_;
  GetTransform().Translate(movement);
}

void Character::AddExperience(int amount) {
  if (amount <= 0) {
    return;
  }

  experience_ += amount;
  
  // Check for level up
  while (experience_ >= experience_to_next_level_) {
    experience_ -= experience_to_next_level_;
    level_++;
    
    // Increase exp required for next level
    experience_to_next_level_ = static_cast<int>(experience_to_next_level_ * 1.5f);
    
    // Increase stats on level up
    max_health_ += 10;
    health_ = max_health_;  // Full heal on level up
    
    OnLevelUp();
    
    std::cout << std::format("{} leveled up to level {}!\n", GetName(), level_);
  }
}

void Character::OnUpdate(float delta_time) {
  // Base character update logic
  // Could handle regeneration, status effects, etc.
  (void)delta_time;
}

}  // namespace game_engine