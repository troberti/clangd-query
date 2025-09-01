#ifndef SYSTEMS_INPUT_SYSTEM_H_
#define SYSTEMS_INPUT_SYSTEM_H_

#include <array>
#include <functional>
#include <optional>
#include <unordered_map>

namespace game_engine {

// Enumeration of all supported keyboard keys. These codes are used to
// identify which key triggered an input event.
enum class KeyCode {
  Unknown = 0,
  A, B, C, D, E, F, G, H, I, J, K, L, M,
  N, O, P, Q, R, S, T, U, V, W, X, Y, Z,
  Num0, Num1, Num2, Num3, Num4, Num5, Num6, Num7, Num8, Num9,
  Space, Enter, Escape, Tab, Backspace, Delete,
  Left, Right, Up, Down,
  LeftShift, RightShift, LeftCtrl, RightCtrl, LeftAlt, RightAlt,
  F1, F2, F3, F4, F5, F6, F7, F8, F9, F10, F11, F12,
  Count
};

// Enumeration of mouse button identifiers.
enum class MouseButton {
  Left = 0,
  Right,
  Middle,
  Count
};

// Manages all keyboard and mouse input for the game engine. This system
// tracks the current state of all keys and mouse buttons, detects state
// changes (just pressed/released), and provides callback mechanisms for
// input event handling.
class InputSystem {
 public:
  using KeyCallback = std::function<void(KeyCode, bool)>;
  using MouseButtonCallback = std::function<void(MouseButton, bool)>;
  using MouseMoveCallback = std::function<void(float, float)>;

  InputSystem();
  ~InputSystem();

  // Updates the input system's internal state. This should be called once
  // per frame to properly track just-pressed and just-released states.
  void Update();

  // Checks whether the specified key is currently being held down.
  // Returns true as long as the key remains pressed.
  bool IsKeyPressed(KeyCode key) const;

  // Checks whether the specified key was pressed down during this frame.
  // Returns true only on the frame when the key transitions from released
  // to pressed state.
  bool IsKeyJustPressed(KeyCode key) const;

  // Checks whether the specified key was released during this frame.
  // Returns true only on the frame when the key transitions from pressed
  // to released state.
  bool IsKeyJustReleased(KeyCode key) const;

  // Checks whether the specified mouse button is currently being held down.
  // Returns true as long as the button remains pressed.
  bool IsMouseButtonPressed(MouseButton button) const;

  // Returns the current mouse cursor position as a pair of (x, y) coordinates
  // in screen space.
  std::pair<float, float> GetMousePosition() const {
    return {mouse_x_, mouse_y_};
  }

  // Returns the mouse movement delta since the last frame as a pair of
  // (dx, dy) values. Useful for implementing camera controls or dragging.
  std::pair<float, float> GetMouseDelta() const {
    return {mouse_dx_, mouse_dy_};
  }

  // Registers a callback function that will be invoked whenever a key
  // state changes. The callback receives the key code and whether it
  // was pressed or released.
  void RegisterKeyCallback(KeyCallback callback) {
    key_callbacks_.push_back(callback);
  }

  // Registers a callback function that will be invoked whenever a mouse
  // button state changes. The callback receives the button identifier
  // and whether it was pressed or released.
  void RegisterMouseButtonCallback(MouseButtonCallback callback) {
    mouse_button_callbacks_.push_back(callback);
  }

  // Registers a callback function that will be invoked whenever the mouse
  // cursor moves. The callback receives the new x and y coordinates.
  void RegisterMouseMoveCallback(MouseMoveCallback callback) {
    mouse_move_callbacks_.push_back(callback);
  }

  // Internal methods for the engine to update input state
  void OnKeyEvent(KeyCode key, bool pressed);
  void OnMouseButtonEvent(MouseButton button, bool pressed);
  void OnMouseMove(float x, float y);

 private:
  std::array<bool, static_cast<size_t>(KeyCode::Count)> key_states_{};
  std::array<bool, static_cast<size_t>(KeyCode::Count)> key_just_pressed_{};
  std::array<bool, static_cast<size_t>(KeyCode::Count)> key_just_released_{};
  
  std::array<bool, static_cast<size_t>(MouseButton::Count)> mouse_button_states_{};
  
  float mouse_x_ = 0.0f;
  float mouse_y_ = 0.0f;
  float mouse_dx_ = 0.0f;
  float mouse_dy_ = 0.0f;
  float last_mouse_x_ = 0.0f;
  float last_mouse_y_ = 0.0f;

  std::vector<KeyCallback> key_callbacks_;
  std::vector<MouseButtonCallback> mouse_button_callbacks_;
  std::vector<MouseMoveCallback> mouse_move_callbacks_;
};

}  // namespace game_engine

#endif  // SYSTEMS_INPUT_SYSTEM_H_