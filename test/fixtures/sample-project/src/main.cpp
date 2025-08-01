#include <format>
#include <iostream>

#include "core/engine.h"
#include "game/player.h"

using namespace game_engine;

int main() {
  std::cout << std::format("Starting {} Engine v1.0\n", "Sample Game");
  
  // Get engine instance
  auto& engine = Engine::GetInstance();
  
  // Initialize engine
  if (!engine.Initialize()) {
    std::cerr << "Failed to initialize engine\n";
    return 1;
  }
  
  // Create a player
  auto player = std::make_shared<Player>("Player1");
  player->SetHealth(100);
  player->SetLevel(1);
  
  // Print player info using std::format
  std::cout << std::format("Created player: {} (Level {}, Health: {}/{})\n",
                          player->GetName(),
                          player->GetLevel(),
                          player->GetHealth(),
                          player->GetMaxHealth());
  
  // Demonstrate optional usage
  if (auto weapon = player->GetWeapon()) {
    std::cout << std::format("Player has weapon: {}\n", *weapon);
  } else {
    std::cout << "Player has no weapon equipped\n";
  }
  
  // Set a weapon
  player->SetWeapon("Iron Sword");
  std::cout << std::format("Equipped weapon: {}\n", *player->GetWeapon());
  
  // Demonstrate three-way comparison
  auto player2 = std::make_shared<Player>("Player2");
  if (player <=> player2 == std::strong_ordering::less) {
    std::cout << "Player1 was created before Player2\n";
  }
  
  // Clean shutdown
  engine.Shutdown();
  
  std::cout << "Engine shutdown complete\n";
  return 0;
}