#include "game/enemy.h"

#include <cmath>
#include <format>
#include <iostream>

namespace game_engine {

Enemy::Enemy(const std::string& name, EnemyType type)
    : Character(name), enemy_type_(type) {
  // Set stats based on enemy type
  switch (enemy_type_) {
    case EnemyType::Zombie:
      SetMaxHealth(50);
      SetHealth(50);
      SetMoveSpeed(2.0f);
      SetAttackDamage(5);
      break;
    case EnemyType::Skeleton:
      SetMaxHealth(30);
      SetHealth(30);
      SetMoveSpeed(4.0f);
      SetAttackDamage(8);
      break;
    case EnemyType::Dragon:
      SetMaxHealth(500);
      SetHealth(500);
      SetMoveSpeed(8.0f);
      SetAttackDamage(50);
      SetAttackRange(10.0f);
      break;
    case EnemyType::Boss:
      SetMaxHealth(1000);
      SetHealth(1000);
      SetMoveSpeed(3.0f);
      SetAttackDamage(30);
      SetAttackRange(5.0f);
      break;
  }
}

Enemy::~Enemy() = default;

void Enemy::Attack() {
  if (!CanAttack()) {
    return;
  }

  if (auto target = target_.lock()) {
    // In a real game, this would deal damage to the target
    std::cout << std::format("{} attacks for {} damage!\n", 
                            GetName(), attack_damage_);
    time_since_last_attack_ = 0.0f;
  }
}

bool Enemy::CanAttack() const {
  if (!IsAlive() || time_since_last_attack_ < attack_cooldown_) {
    return false;
  }

  if (auto target = target_.lock()) {
    // Check if target is in range
    Vector3 to_target = target->GetTransform().GetPosition() - 
                       GetTransform().GetPosition();
    float distance_sq = to_target.x * to_target.x + 
                       to_target.y * to_target.y + 
                       to_target.z * to_target.z;
    return distance_sq <= (attack_range_ * attack_range_);
  }

  return false;
}

void Enemy::UpdateAI(float delta_time) {
  if (!IsAlive()) {
    return;
  }

  time_since_last_attack_ += delta_time;

  // Simple AI state machine
  switch (current_state_) {
    case AIState::Idle:
      // Look for target
      if (target_.lock()) {
        current_state_ = AIState::Chasing;
      } else {
        // Start patrolling
        patrol_target_ = GetTransform().GetPosition() + 
                        Vector3{10.0f, 0.0f, 10.0f};
        current_state_ = AIState::Patrolling;
      }
      break;

    case AIState::Patrolling:
      // Move towards patrol target
      if (target_.lock()) {
        current_state_ = AIState::Chasing;
      }
      break;

    case AIState::Chasing:
      if (auto target = target_.lock()) {
        if (CanAttack()) {
          current_state_ = AIState::Attacking;
        } else {
          // Move towards target
          Vector3 direction = target->GetTransform().GetPosition() - 
                             GetTransform().GetPosition();
          // Normalize direction
          float length = std::sqrt(direction.x * direction.x + 
                                 direction.y * direction.y + 
                                 direction.z * direction.z);
          if (length > 0.001f) {
            direction = direction * (1.0f / length);
            Move(direction * delta_time);
          }
        }
      } else {
        current_state_ = AIState::Idle;
      }
      break;

    case AIState::Attacking:
      Attack();
      if (!CanAttack()) {
        current_state_ = AIState::Chasing;
      }
      break;
  }
}

void Enemy::OnCreate() {
  std::cout << std::format("Enemy '{}' spawned\n", GetName());
}

void Enemy::OnUpdate(float delta_time) {
  Character::OnUpdate(delta_time);
  UpdateAI(delta_time);
}

}  // namespace game_engine