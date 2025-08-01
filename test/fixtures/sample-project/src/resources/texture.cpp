#include "resources/texture.h"

#include <cstring>
#include <format>
#include <iostream>

namespace game_engine {

Texture::Texture(const std::string& path) : Resource(path) {
}

Texture::~Texture() {
  Unload();
}

bool Texture::Load() {
  if (IsLoaded()) {
    return true;
  }

  // In a real implementation, would load image data from file
  // For now, create a dummy texture
  width_ = 256;
  height_ = 256;
  channels_ = 4;  // RGBA
  
  size_t size = GetSizeInBytes();
  data_ = new uint8_t[size];
  
  // Create a simple checkerboard pattern
  for (int y = 0; y < height_; ++y) {
    for (int x = 0; x < width_; ++x) {
      int index = (y * width_ + x) * channels_;
      bool checker = ((x / 32) + (y / 32)) % 2;
      
      data_[index + 0] = checker ? 255 : 0;      // R
      data_[index + 1] = checker ? 255 : 0;      // G
      data_[index + 2] = checker ? 255 : 0;      // B
      data_[index + 3] = 255;                    // A
    }
  }
  
  std::cout << std::format("Loaded texture: {} ({}x{}, {} channels)\n",
                          GetPath(), width_, height_, channels_);
  
  return true;
}

void Texture::Unload() {
  if (data_) {
    delete[] data_;
    data_ = nullptr;
    width_ = 0;
    height_ = 0;
    channels_ = 0;
    
    std::cout << std::format("Unloaded texture: {}\n", GetPath());
  }
}

}  // namespace game_engine