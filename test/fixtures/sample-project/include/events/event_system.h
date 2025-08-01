#ifndef EVENTS_EVENT_SYSTEM_H_
#define EVENTS_EVENT_SYSTEM_H_

#include <functional>
#include <memory>
#include <typeindex>
#include <unordered_map>
#include <vector>

namespace game_engine {

// Base event class
class Event {
 public:
  virtual ~Event() = default;
  
  // Gets the event type name
  virtual const char* GetTypeName() const = 0;
};

// Macro to declare event type
#define DECLARE_EVENT_TYPE(EventClass) \
  const char* GetTypeName() const override { return #EventClass; }

// Event listener handle
class EventListenerHandle {
 public:
  EventListenerHandle() = default;
  EventListenerHandle(size_t id, std::type_index type)
      : id_(id), type_(type), valid_(true) {}

  bool IsValid() const { return valid_; }
  void Invalidate() { valid_ = false; }
  
  size_t GetId() const { return id_; }
  std::type_index GetType() const { return type_; }

 private:
  size_t id_ = 0;
  std::type_index type_ = typeid(void);
  bool valid_ = false;
};

// Event dispatcher
class EventDispatcher {
 public:
  // Singleton access
  static EventDispatcher& GetInstance() {
    static EventDispatcher instance;
    return instance;
  }

  // Subscribe to an event type
  template <typename T>
  EventListenerHandle Subscribe(std::function<void(const T&)> callback) {
    static_assert(std::is_base_of_v<Event, T>, "T must derive from Event");
    
    auto type_index = std::type_index(typeid(T));
    auto& listeners = listeners_[type_index];
    
    size_t id = next_listener_id_++;
    listeners.emplace_back(id, [callback](const Event& event) {
      callback(static_cast<const T&>(event));
    });
    
    return EventListenerHandle(id, type_index);
  }

  // Unsubscribe from events
  void Unsubscribe(EventListenerHandle& handle) {
    if (!handle.IsValid()) {
      return;
    }

    auto it = listeners_.find(handle.GetType());
    if (it != listeners_.end()) {
      auto& listeners = it->second;
      listeners.erase(
          std::remove_if(listeners.begin(), listeners.end(),
                         [&handle](const auto& listener) {
                           return listener.first == handle.GetId();
                         }),
          listeners.end());
    }
    
    handle.Invalidate();
  }

  // Dispatch an event
  template <typename T>
  void Dispatch(const T& event) {
    static_assert(std::is_base_of_v<Event, T>, "T must derive from Event");
    
    auto type_index = std::type_index(typeid(T));
    auto it = listeners_.find(type_index);
    
    if (it != listeners_.end()) {
      // Copy listeners in case callbacks modify the list
      auto listeners_copy = it->second;
      for (const auto& [id, callback] : listeners_copy) {
        callback(event);
      }
    }
  }

 private:
  EventDispatcher() = default;
  
  using ListenerCallback = std::function<void(const Event&)>;
  using ListenerPair = std::pair<size_t, ListenerCallback>;
  
  std::unordered_map<std::type_index, std::vector<ListenerPair>> listeners_;
  size_t next_listener_id_ = 1;
};

// Example event types
class CollisionEvent : public Event {
 public:
  DECLARE_EVENT_TYPE(CollisionEvent)
  
  CollisionEvent(uint64_t object_a, uint64_t object_b)
      : object_a_id_(object_a), object_b_id_(object_b) {}
  
  uint64_t GetObjectA() const { return object_a_id_; }
  uint64_t GetObjectB() const { return object_b_id_; }

 private:
  uint64_t object_a_id_;
  uint64_t object_b_id_;
};

class LevelUpEvent : public Event {
 public:
  DECLARE_EVENT_TYPE(LevelUpEvent)
  
  LevelUpEvent(uint64_t character_id, int new_level)
      : character_id_(character_id), new_level_(new_level) {}
  
  uint64_t GetCharacterId() const { return character_id_; }
  int GetNewLevel() const { return new_level_; }

 private:
  uint64_t character_id_;
  int new_level_;
};

}  // namespace game_engine

#endif  // EVENTS_EVENT_SYSTEM_H_