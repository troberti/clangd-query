#ifndef GAME_ENEMY_H_
#define GAME_ENEMY_H_

#include "game/character.h"

namespace game_engine {

// Base class for all enemy types
class Enemy : public Character {
 public:
  enum class EnemyType {
    Zombie,
    Skeleton,
    Dragon,
    Boss
  };

  Enemy(const std::string& name, EnemyType type);
  ~Enemy() override;

  // Gets the enemy type
  EnemyType GetEnemyType() const { return enemy_type_; }

  // AI behavior
  void SetTarget(std::weak_ptr<GameObject> target) { target_ = target; }
  std::weak_ptr<GameObject> GetTarget() const { return target_; }

  // Attack behavior
  void Attack();
  bool CanAttack() const;

  // Gets the damage this enemy deals
  int GetAttackDamage() const { return attack_damage_; }
  void SetAttackDamage(int damage) { attack_damage_ = damage; }

  // Gets the attack range
  float GetAttackRange() const { return attack_range_; }
  void SetAttackRange(float range) { attack_range_ = range; }

  // Update AI behavior
  void UpdateAI(float delta_time);

 protected:
  void OnCreate() override;
  void OnUpdate(float delta_time) override;

 private:
  EnemyType enemy_type_;
  std::weak_ptr<GameObject> target_;
  
  int attack_damage_ = 10;
  float attack_range_ = 2.0f;
  float attack_cooldown_ = 1.0f;
  float time_since_last_attack_ = 0.0f;
  
  // AI state
  enum class AIState {
    Idle,
    Patrolling,
    Chasing,
    Attacking
  };
  
  AIState current_state_ = AIState::Idle;
  Vector3 patrol_target_;
};

}  // namespace game_engine

#endif  // GAME_ENEMY_H_