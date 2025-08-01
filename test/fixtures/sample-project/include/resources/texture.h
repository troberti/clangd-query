#ifndef RESOURCES_TEXTURE_H_
#define RESOURCES_TEXTURE_H_

#include <cstdint>
#include <vector>

#include "utils/resource_manager.h"

namespace game_engine {

// Represents a 2D texture resource
class Texture : public Resource {
 public:
  explicit Texture(const std::string& path);
  ~Texture() override;

  // Resource interface
  bool Load() override;
  void Unload() override;
  bool IsLoaded() const override { return data_ != nullptr; }

  // Gets texture dimensions
  int GetWidth() const { return width_; }
  int GetHeight() const { return height_; }
  int GetChannels() const { return channels_; }

  // Gets the raw pixel data
  const uint8_t* GetData() const { return data_; }

  // Gets the size in bytes
  size_t GetSizeInBytes() const {
    return width_ * height_ * channels_;
  }

 private:
  int width_ = 0;
  int height_ = 0;
  int channels_ = 0;
  uint8_t* data_ = nullptr;
};

}  // namespace game_engine

#endif  // RESOURCES_TEXTURE_H_