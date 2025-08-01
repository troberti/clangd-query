#ifndef SYSTEMS_INPUT_SYSTEM_H_
#define SYSTEMS_INPUT_SYSTEM_H_

#include <array>
#include <functional>
#include <optional>
#include <unordered_map>

namespace game_engine {

/**
 * @brief Keyboard key codes
 */
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

/**
 * @brief Mouse button codes
 */
enum class MouseButton {
  Left = 0,
  Right,
  Middle,
  Count
};

/**
 * @brief Manages input from keyboard and mouse
 */
class InputSystem {
 public:
  using KeyCallback = std::function<void(KeyCode, bool)>;
  using MouseButtonCallback = std::function<void(MouseButton, bool)>;
  using MouseMoveCallback = std::function<void(float, float)>;

  InputSystem();
  ~InputSystem();

  /**
   * @brief Updates the input system
   */
  void Update();

  /**
   * @brief Checks if a key is currently pressed
   * @param key The key to check
   * @return true if the key is pressed
   */
  bool IsKeyPressed(KeyCode key) const;

  /**
   * @brief Checks if a key was just pressed this frame
   * @param key The key to check
   * @return true if the key was just pressed
   */
  bool IsKeyJustPressed(KeyCode key) const;

  /**
   * @brief Checks if a key was just released this frame
   * @param key The key to check
   * @return true if the key was just released
   */
  bool IsKeyJustReleased(KeyCode key) const;

  /**
   * @brief Checks if a mouse button is pressed
   * @param button The button to check
   * @return true if the button is pressed
   */
  bool IsMouseButtonPressed(MouseButton button) const;

  /**
   * @brief Gets the mouse position
   * @return Pair of (x, y) coordinates
   */
  std::pair<float, float> GetMousePosition() const {
    return {mouse_x_, mouse_y_};
  }

  /**
   * @brief Gets the mouse delta movement
   * @return Pair of (dx, dy) movement
   */
  std::pair<float, float> GetMouseDelta() const {
    return {mouse_dx_, mouse_dy_};
  }

  /**
   * @brief Registers a key callback
   * @param callback Function to call on key events
   */
  void RegisterKeyCallback(KeyCallback callback) {
    key_callbacks_.push_back(callback);
  }

  /**
   * @brief Registers a mouse button callback
   * @param callback Function to call on mouse button events
   */
  void RegisterMouseButtonCallback(MouseButtonCallback callback) {
    mouse_button_callbacks_.push_back(callback);
  }

  /**
   * @brief Registers a mouse move callback
   * @param callback Function to call on mouse movement
   */
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