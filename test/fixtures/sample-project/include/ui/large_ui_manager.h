#ifndef UI_LARGE_UI_MANAGER_H_
#define UI_LARGE_UI_MANAGER_H_

#include <memory>
#include <string>
#include <vector>
#include <map>
#include <functional>

namespace game_engine {

// Forward declarations
class GameObject;
class Transform;
class Component;

// Test class with multiple inheritance and many delegates to reproduce folding range issues

// Multiple delegate interfaces for testing
class UIEventDelegate {
 public:
  virtual ~UIEventDelegate() = default;
  virtual void OnButtonPressed(const std::string& button_id) {}
  virtual void OnButtonReleased(const std::string& button_id) {}
  virtual void OnSliderChanged(const std::string& slider_id, float value) {}
};

class WindowDelegate {
 public:
  virtual ~WindowDelegate() = default;
  virtual void OnWindowOpened(const std::string& window_id) {}
  virtual void OnWindowClosed(const std::string& window_id) {}
  virtual void OnWindowResized(int width, int height) {}
};

class MenuDelegate {
 public:
  virtual ~MenuDelegate() = default;
  virtual void OnMenuItemSelected(const std::string& item_id) {}
  virtual void OnMenuOpened(const std::string& menu_id) {}
  virtual void OnMenuClosed(const std::string& menu_id) {}
};

class DialogDelegate {
 public:
  virtual ~DialogDelegate() = default;
  virtual void OnDialogConfirmed(const std::string& dialog_id) {}
  virtual void OnDialogCancelled(const std::string& dialog_id) {}
  virtual void OnDialogTextEntered(const std::string& dialog_id, const std::string& text) {}
};

class AnimationDelegate {
 public:
  virtual ~AnimationDelegate() = default;
  virtual void OnAnimationStarted(const std::string& anim_id) {}
  virtual void OnAnimationCompleted(const std::string& anim_id) {}
};

// Large UI manager class with multiple inheritance
class LargeUIManager : public UIEventDelegate,
                        public WindowDelegate,
                        public MenuDelegate,
                        public DialogDelegate,
                        public AnimationDelegate {
 public:
  // Constructor with multi-line parameters to test signature parsing
  LargeUIManager(const std::string& name,
                 std::shared_ptr<GameObject> root_object,
                 const std::map<std::string, std::string>& initial_config,
                 std::function<void(const std::string&)> error_handler = nullptr);

  virtual ~LargeUIManager();

  // Initialization and shutdown
  void Initialize();
  void Shutdown();
  void Update(float delta_time);
  void Render();

  // Window management
  void CreateWindow(const std::string& window_id, int width, int height);
  void DestroyWindow(const std::string& window_id);
  void ShowWindow(const std::string& window_id);
  void HideWindow(const std::string& window_id);

  // Menu management
  void CreateMenu(const std::string& menu_id, const std::vector<std::string>& items);
  void ShowMenu(const std::string& menu_id);
  void HideMenu(const std::string& menu_id);

  // Dialog management
  void ShowDialog(const std::string& dialog_id, const std::string& message);
  void ShowConfirmDialog(const std::string& dialog_id,
                         const std::string& message,
                         std::function<void()> on_confirm,
                         std::function<void()> on_cancel = nullptr);

  // Complex callback registration with std::function parameters
  void RegisterButtonCallback(const std::string& button_id,
                              std::function<void()> callback);

  void RegisterComplexCallback(const std::string& id,
                               std::function<bool(const std::vector<int>&)> validator,
                               std::map<std::string, std::function<void()>> handlers);

  // Inline resource loading method
  void LoadResources() {
    if (!resources_loaded_) {
      // Load button resources
      button_normal_texture_ = LoadTexture("button_normal.png");
      button_pressed_texture_ = LoadTexture("button_pressed.png");
      button_hover_texture_ = LoadTexture("button_hover.png");
      button_disabled_texture_ = LoadTexture("button_disabled.png");

      // Load window resources
      window_background_texture_ = LoadTexture("window_bg.png");
      window_border_texture_ = LoadTexture("window_border.png");
      window_title_texture_ = LoadTexture("window_title.png");

      // Load menu resources
      menu_background_texture_ = LoadTexture("menu_bg.png");
      menu_item_texture_ = LoadTexture("menu_item.png");
      menu_separator_texture_ = LoadTexture("menu_separator.png");

      // Load dialog resources
      dialog_background_texture_ = LoadTexture("dialog_bg.png");
      dialog_button_texture_ = LoadTexture("dialog_button.png");

      // Load sound effects
      button_click_sound_ = LoadSound("button_click.wav");
      window_open_sound_ = LoadSound("window_open.wav");
      window_close_sound_ = LoadSound("window_close.wav");
      menu_select_sound_ = LoadSound("menu_select.wav");

      resources_loaded_ = true;
    }
  }

  // Getters
  bool IsInitialized() const { return initialized_; }
  bool IsVisible() const { return visible_; }
  std::string GetName() const { return name_; }

 protected:
  // UIEventDelegate overrides
  virtual void OnButtonPressed(const std::string& button_id) override;
  virtual void OnButtonReleased(const std::string& button_id) override;
  virtual void OnSliderChanged(const std::string& slider_id, float value) override;

  // WindowDelegate overrides
  virtual void OnWindowOpened(const std::string& window_id) override;
  virtual void OnWindowClosed(const std::string& window_id) override;
  virtual void OnWindowResized(int width, int height) override;

  // MenuDelegate overrides
  virtual void OnMenuItemSelected(const std::string& item_id) override;
  virtual void OnMenuOpened(const std::string& menu_id) override;
  virtual void OnMenuClosed(const std::string& menu_id) override;

  // DialogDelegate overrides
  virtual void OnDialogConfirmed(const std::string& dialog_id) override;
  virtual void OnDialogCancelled(const std::string& dialog_id) override;
  virtual void OnDialogTextEntered(const std::string& dialog_id, const std::string& text) override;

  // AnimationDelegate overrides
  virtual void OnAnimationStarted(const std::string& anim_id) override;
  virtual void OnAnimationCompleted(const std::string& anim_id) override;

 private:
  // Helper methods
  void* LoadTexture(const std::string& filename);
  void* LoadSound(const std::string& filename);
  void UpdateWindows(float delta_time);
  void UpdateMenus(float delta_time);
  void UpdateDialogs(float delta_time);
  void UpdateAnimations(float delta_time);
  void ProcessEventQueue();
  void CleanupResources();

  // Window management helpers
  struct WindowInfo {
    std::string id;
    int width;
    int height;
    bool visible;
    void* texture;
  };

  // Menu management helpers
  struct MenuInfo {
    std::string id;
    std::vector<std::string> items;
    int selected_index;
    bool visible;
  };

  // Dialog management helpers
  struct DialogInfo {
    std::string id;
    std::string message;
    std::function<void()> on_confirm;
    std::function<void()> on_cancel;
    bool visible;
  };

  // Member variables
  std::string name_;
  bool initialized_ = false;
  bool visible_ = true;
  bool resources_loaded_ = false;

  // UI element containers
  std::map<std::string, WindowInfo> windows_;
  std::map<std::string, MenuInfo> menus_;
  std::map<std::string, DialogInfo> dialogs_;
  std::vector<std::string> event_queue_;

  // Callbacks
  std::map<std::string, std::function<void()>> button_callbacks_;
  std::map<std::string, std::vector<std::function<void()>>> complex_callbacks_;
  std::function<void(const std::string&)> error_handler_;

  // Resources
  void* button_normal_texture_ = nullptr;
  void* button_pressed_texture_ = nullptr;
  void* button_hover_texture_ = nullptr;
  void* button_disabled_texture_ = nullptr;

  void* window_background_texture_ = nullptr;
  void* window_border_texture_ = nullptr;
  void* window_title_texture_ = nullptr;

  void* menu_background_texture_ = nullptr;
  void* menu_item_texture_ = nullptr;
  void* menu_separator_texture_ = nullptr;

  void* dialog_background_texture_ = nullptr;
  void* dialog_button_texture_ = nullptr;

  void* button_click_sound_ = nullptr;
  void* window_open_sound_ = nullptr;
  void* window_close_sound_ = nullptr;
  void* menu_select_sound_ = nullptr;

  // Game object references
  std::shared_ptr<GameObject> root_object_;
  std::vector<std::shared_ptr<GameObject>> ui_objects_;

  // Configuration
  std::map<std::string, std::string> config_;

  // Statistics
  struct Statistics {
    int windows_created = 0;
    int menus_created = 0;
    int dialogs_shown = 0;
    int buttons_pressed = 0;
    int total_events = 0;
  } stats_;

  // More members to make the class larger
  float update_time_accumulator_ = 0.0f;
  int frame_counter_ = 0;
  bool needs_redraw_ = false;
  std::string last_error_;
  std::vector<std::string> error_log_;
};

// Nested helper class
class UIHelper {
 public:
  UIHelper(LargeUIManager* manager);
  ~UIHelper();

  void ProcessInput();
  void UpdateLayout();

 private:
  LargeUIManager* manager_;
  bool active_ = false;
};

}  // namespace game_engine

#endif  // UI_LARGE_UI_MANAGER_H_