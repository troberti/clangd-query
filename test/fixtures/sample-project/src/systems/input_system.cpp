#include "systems/input_system.h"

#include <iostream>

namespace game_engine {

InputSystem::InputSystem() {
  // Initialize all states to false
  key_states_.fill(false);
  key_just_pressed_.fill(false);
  key_just_released_.fill(false);
  mouse_button_states_.fill(false);
}

InputSystem::~InputSystem() = default;

void InputSystem::Update() {
  // Clear just pressed/released states
  key_just_pressed_.fill(false);
  key_just_released_.fill(false);
  
  // Update mouse delta
  mouse_dx_ = mouse_x_ - last_mouse_x_;
  mouse_dy_ = mouse_y_ - last_mouse_y_;
  last_mouse_x_ = mouse_x_;
  last_mouse_y_ = mouse_y_;
}

bool InputSystem::IsKeyPressed(KeyCode key) const {
  size_t index = static_cast<size_t>(key);
  if (index < key_states_.size()) {
    return key_states_[index];
  }
  return false;
}

bool InputSystem::IsKeyJustPressed(KeyCode key) const {
  size_t index = static_cast<size_t>(key);
  if (index < key_just_pressed_.size()) {
    return key_just_pressed_[index];
  }
  return false;
}

bool InputSystem::IsKeyJustReleased(KeyCode key) const {
  size_t index = static_cast<size_t>(key);
  if (index < key_just_released_.size()) {
    return key_just_released_[index];
  }
  return false;
}

bool InputSystem::IsMouseButtonPressed(MouseButton button) const {
  size_t index = static_cast<size_t>(button);
  if (index < mouse_button_states_.size()) {
    return mouse_button_states_[index];
  }
  return false;
}

void InputSystem::OnKeyEvent(KeyCode key, bool pressed) {
  size_t index = static_cast<size_t>(key);
  if (index >= key_states_.size()) {
    return;
  }

  bool was_pressed = key_states_[index];
  key_states_[index] = pressed;

  if (pressed && !was_pressed) {
    key_just_pressed_[index] = true;
  } else if (!pressed && was_pressed) {
    key_just_released_[index] = true;
  }

  // Notify callbacks
  for (auto& callback : key_callbacks_) {
    callback(key, pressed);
  }
}

void InputSystem::OnMouseButtonEvent(MouseButton button, bool pressed) {
  size_t index = static_cast<size_t>(button);
  if (index >= mouse_button_states_.size()) {
    return;
  }

  mouse_button_states_[index] = pressed;

  // Notify callbacks
  for (auto& callback : mouse_button_callbacks_) {
    callback(button, pressed);
  }
}

void InputSystem::OnMouseMove(float x, float y) {
  mouse_x_ = x;
  mouse_y_ = y;

  // Notify callbacks
  for (auto& callback : mouse_move_callbacks_) {
    callback(x, y);
  }
}

}  // namespace game_engine