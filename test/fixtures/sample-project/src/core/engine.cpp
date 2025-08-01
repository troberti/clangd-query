#include "core/engine.h"

#include <algorithm>
#include <format>
#include <iostream>

#include "core/game_object.h"
#include "systems/render_system.h"
#include "systems/physics_system.h"
#include "systems/input_system.h"

namespace game_engine {

Engine& Engine::GetInstance() {
  static Engine instance;
  return instance;
}

Engine::~Engine() {
  if (initialized_) {
    Shutdown();
  }
}

bool Engine::Initialize(std::optional<std::string> config_file) {
  if (initialized_) {
    return true;
  }

  std::cout << "Initializing engine...\n";
  
  if (config_file) {
    std::cout << std::format("Loading config from: {}\n", *config_file);
  }

  // Create subsystems
  render_system_ = std::make_unique<RenderSystem>();
  physics_system_ = std::make_unique<PhysicsSystem>();
  input_system_ = std::make_unique<InputSystem>();

  // Initialize subsystems
  if (!render_system_->Initialize(1280, 720, "Game Engine")) {
    std::cerr << "Failed to initialize render system\n";
    return false;
  }

  if (!physics_system_->Initialize()) {
    std::cerr << "Failed to initialize physics system\n";
    return false;
  }

  start_time_ = std::chrono::steady_clock::now();
  last_frame_time_ = start_time_;
  
  initialized_ = true;
  std::cout << "Engine initialized successfully\n";
  return true;
}

void Engine::Shutdown() {
  if (!initialized_) {
    return;
  }

  std::cout << "Shutting down engine...\n";
  
  // Clear all game objects
  game_objects_.clear();
  objects_to_destroy_.clear();

  // Shutdown subsystems
  if (render_system_) {
    render_system_->Shutdown();
    render_system_.reset();
  }

  if (physics_system_) {
    physics_system_->Shutdown();
    physics_system_.reset();
  }

  input_system_.reset();
  
  initialized_ = false;
  std::cout << "Engine shutdown complete\n";
}

void Engine::Run() {
  if (!initialized_) {
    std::cerr << "Cannot run - engine not initialized\n";
    return;
  }

  running_ = true;
  
  while (running_) {
    auto current_time = std::chrono::steady_clock::now();
    float delta_time = std::chrono::duration<float>(
        current_time - last_frame_time_).count();
    last_frame_time_ = current_time;

    // Update FPS counter
    UpdateFPS(delta_time);

    // Process input
    input_system_->Update();

    // Fixed timestep for physics
    accumulator_ += delta_time;
    while (accumulator_ >= kFixedTimeStep) {
      physics_system_->Update(kFixedTimeStep);
      accumulator_ -= kFixedTimeStep;
    }

    // Update game objects
    for (auto& object : game_objects_) {
      if (object->IsActive()) {
        object->Update(delta_time);
      }
    }

    // Clean up destroyed objects
    CleanupDestroyedObjects();

    // Render
    float interpolation = accumulator_ / kFixedTimeStep;
    render_system_->Render(interpolation);
  }
}

float Engine::GetTime() const {
  auto current_time = std::chrono::steady_clock::now();
  return std::chrono::duration<float>(current_time - start_time_).count();
}

std::shared_ptr<GameObject> Engine::CreateGameObject(const std::string& name) {
  auto object = std::make_shared<GameObject>(name);
  game_objects_.push_back(object);
  return object;
}

void Engine::DestroyGameObject(std::shared_ptr<GameObject> object) {
  if (object) {
    objects_to_destroy_.push_back(object);
  }
}

void Engine::UpdateFPS(float delta_time) {
  frame_count_++;
  fps_update_timer_ += delta_time;
  
  if (fps_update_timer_ >= 1.0f) {
    fps_ = static_cast<float>(frame_count_) / fps_update_timer_;
    frame_count_ = 0;
    fps_update_timer_ = 0.0f;
  }
}

void Engine::CleanupDestroyedObjects() {
  for (auto& object : objects_to_destroy_) {
    auto it = std::find(game_objects_.begin(), game_objects_.end(), object);
    if (it != game_objects_.end()) {
      game_objects_.erase(it);
    }
  }
  objects_to_destroy_.clear();
}

}  // namespace game_engine