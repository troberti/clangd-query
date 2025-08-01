#ifndef GAME_CHARACTER_H_
#define GAME_CHARACTER_H_

#include "core/game_object.h"

namespace game_engine {

// Base class for all characters (players, NPCs, enemies)
class Character : public GameObject {
 public:
  explicit Character(const std::string& name);
  ~Character() override;

  // Health management
  virtual void SetHealth(int health) { health_ = health; }
  virtual int GetHealth() const { return health_; }
  
  virtual void SetMaxHealth(int max_health) { max_health_ = max_health; }
  virtual int GetMaxHealth() const { return max_health_; }

  // Takes damage and returns the actual damage dealt
  virtual int TakeDamage(int damage);

  // Heals the character and returns the actual amount healed
  virtual int Heal(int amount);

  // Checks if the character is alive
  virtual bool IsAlive() const { return health_ > 0; }

  // Movement
  virtual void Move(const Vector3& direction);
  
  // Gets the character's movement speed
  float GetMoveSpeed() const { return move_speed_; }
  void SetMoveSpeed(float speed) { move_speed_ = speed; }

  // Level and experience
  int GetLevel() const { return level_; }
  void SetLevel(int level) { level_ = level; }

  int GetExperience() const { return experience_; }
  void AddExperience(int amount);

 protected:
  // Called when the character dies
  virtual void OnDeath() {}
  
  // Called when the character levels up
  virtual void OnLevelUp() {}

  void OnUpdate(float delta_time) override;

 protected:
  int health_ = 100;
  int max_health_ = 100;
  float move_speed_ = 5.0f;
  
  int level_ = 1;
  int experience_ = 0;
  int experience_to_next_level_ = 100;
};

}  // namespace game_engine

#endif  // GAME_CHARACTER_H_