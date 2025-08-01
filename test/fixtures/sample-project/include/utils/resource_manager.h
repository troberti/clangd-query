#ifndef UTILS_RESOURCE_MANAGER_H_
#define UTILS_RESOURCE_MANAGER_H_

#include <format>
#include <memory>
#include <string>
#include <typeinfo>
#include <unordered_map>

namespace game_engine {

// Base class for all resources
class Resource {
 public:
  explicit Resource(const std::string& path) : path_(path) {}
  virtual ~Resource() = default;

  const std::string& GetPath() const { return path_; }
  
  // Loads the resource from disk
  virtual bool Load() = 0;
  
  // Unloads the resource
  virtual void Unload() = 0;
  
  // Checks if the resource is loaded
  virtual bool IsLoaded() const = 0;

 private:
  std::string path_;
};

// Manages loading and caching of game resources
class ResourceManager {
 public:
  // Gets the singleton instance
  static ResourceManager& GetInstance() {
    static ResourceManager instance;
    return instance;
  }

  // Loads a resource of the specified type
  template <typename T>
  std::shared_ptr<T> Load(const std::string& path) {
    static_assert(std::is_base_of_v<Resource, T>, 
                  "T must be derived from Resource");
    
    // Generate a unique key for this resource type and path
    std::string key = std::format("{}:{}", typeid(T).name(), path);
    
    // Check if already loaded
    auto it = resources_.find(key);
    if (it != resources_.end()) {
      return std::static_pointer_cast<T>(it->second);
    }
    
    // Create and load new resource
    auto resource = std::make_shared<T>(path);
    if (resource->Load()) {
      resources_[key] = resource;
      return resource;
    }
    
    return nullptr;
  }

  // Unloads a specific resource
  void Unload(const std::string& path) {
    auto it = resources_.begin();
    while (it != resources_.end()) {
      if (it->second->GetPath() == path) {
        it->second->Unload();
        it = resources_.erase(it);
      } else {
        ++it;
      }
    }
  }

  // Unloads all resources
  void UnloadAll() {
    for (auto& [key, resource] : resources_) {
      resource->Unload();
    }
    resources_.clear();
  }

  // Gets the number of loaded resources
  size_t GetResourceCount() const { return resources_.size(); }

 private:
  ResourceManager() = default;
  ~ResourceManager() { UnloadAll(); }

  std::unordered_map<std::string, std::shared_ptr<Resource>> resources_;
};

}  // namespace game_engine

#endif  // UTILS_RESOURCE_MANAGER_H_