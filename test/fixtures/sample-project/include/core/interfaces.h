#ifndef CORE_INTERFACES_H_
#define CORE_INTERFACES_H_

#include <string>
#include <vector>
#include <memory>
#include <span>
#include <chrono>

namespace game_engine {

/**
 * @brief Base interface for objects that can be updated in the game loop
 * 
 * This interface defines the contract for any object that needs to
 * participate in the game's update cycle.
 */
class Updatable {
 public:
  virtual ~Updatable() = default;
  
  /**
   * @brief Updates the object state
   * @param delta_time Time elapsed since last update in seconds
   */
  virtual void Update(float delta_time) = 0;
  
  /**
   * @brief Checks if the object should be updated
   * @return true if the object is active and should be updated
   */
  virtual bool IsActive() const = 0;
};

/**
 * @brief Interface for objects that can be rendered
 * 
 * Provides a contract for drawable game objects. Implementations
 * should handle their own rendering logic.
 */
class Renderable {
 public:
  virtual ~Renderable() = default;
  
  /**
   * @brief Renders the object to the screen
   * @param interpolation Interpolation factor for smooth rendering
   */
  virtual void Render(float interpolation) = 0;
  
  /**
   * @brief Gets the render priority (lower values render first)
   * @return The render priority value
   */
  virtual int GetRenderPriority() const = 0;
  
  /**
   * @brief Checks if the object is visible
   * @return true if the object should be rendered
   */
  virtual bool IsVisible() const = 0;
};

/**
 * @brief Interface for objects that can be serialized
 * 
 * Allows objects to save and load their state. This is useful
 * for game saves, network synchronization, and debugging.
 */
class Serializable {
 public:
  virtual ~Serializable() = default;
  
  /**
   * @brief Serializes the object to a byte array
   * @return Vector of bytes representing the serialized object
   */
  virtual std::vector<uint8_t> Serialize() const = 0;
  
  /**
   * @brief Deserializes the object from a byte array
   * @param data Span of bytes to deserialize from
   * @return true if deserialization was successful
   */
  virtual bool Deserialize(std::span<const uint8_t> data) = 0;
  
  /**
   * @brief Gets the serialization version for this object type
   * @return Version number for compatibility checking
   */
  virtual uint32_t GetSerializationVersion() const = 0;
};

/**
 * @brief Interface for objects that can handle events
 */
class EventHandler {
 public:
  virtual ~EventHandler() = default;
  
  /**
   * @brief Handles an event
   * @param event_type Type identifier for the event
   * @param event_data Optional data associated with the event
   */
  virtual void HandleEvent(uint32_t event_type, void* event_data) = 0;
  
  /**
   * @brief Gets the types of events this handler is interested in
   * @return Span of event type identifiers
   */
  virtual std::span<const uint32_t> GetHandledEventTypes() const = 0;
};

}  // namespace game_engine

#endif  // CORE_INTERFACES_H_