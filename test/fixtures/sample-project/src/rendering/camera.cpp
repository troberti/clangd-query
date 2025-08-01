#include "rendering/camera.h"

#include <cmath>

#include "core/game_object.h"

namespace game_engine {

Camera::Camera() : Component("Camera") {
}

Camera::~Camera() = default;

Camera::Matrix4x4 Camera::GetViewMatrix() const {
  auto owner = GetOwner().lock();
  if (!owner) {
    return Matrix4x4{};
  }

  const auto& transform = owner->GetTransform();
  const auto& position = transform.GetPosition();
  const auto& rotation = transform.GetRotation();

  // Simplified view matrix calculation
  // In a real engine, this would involve proper 3D transformations
  Matrix4x4 view;
  
  // Translation part
  view.data[12] = -position.x;
  view.data[13] = -position.y;
  view.data[14] = -position.z;
  
  return view;
}

Camera::Matrix4x4 Camera::GetProjectionMatrix() const {
  Matrix4x4 projection;
  
  if (projection_type_ == ProjectionType::Perspective) {
    // Simplified perspective projection
    float aspect = GetAspectRatio();
    float fov_rad = field_of_view_ * (3.14159f / 180.0f);
    float f = 1.0f / std::tan(fov_rad * 0.5f);
    
    projection.data[0] = f / aspect;
    projection.data[5] = f;
    projection.data[10] = (far_plane_ + near_plane_) / (near_plane_ - far_plane_);
    projection.data[11] = -1.0f;
    projection.data[14] = (2.0f * far_plane_ * near_plane_) / (near_plane_ - far_plane_);
    projection.data[15] = 0.0f;
  } else {
    // Orthographic projection
    float width = static_cast<float>(viewport_width_);
    float height = static_cast<float>(viewport_height_);
    
    projection.data[0] = 2.0f / width;
    projection.data[5] = 2.0f / height;
    projection.data[10] = -2.0f / (far_plane_ - near_plane_);
    projection.data[14] = -(far_plane_ + near_plane_) / (far_plane_ - near_plane_);
  }
  
  return projection;
}

void Camera::OnUpdate(float delta_time) {
  // Camera-specific update logic
  (void)delta_time;
}

// Free function implementation
Camera::Matrix4x4 LookAt(const Vector3& eye, const Vector3& target, const Vector3& up) {
  // Simplified look-at matrix calculation
  Camera::Matrix4x4 result;
  
  // Calculate forward vector
  Vector3 forward = target - eye;
  float length = std::sqrt(forward.x * forward.x + 
                          forward.y * forward.y + 
                          forward.z * forward.z);
  if (length > 0.0001f) {
    forward = forward * (1.0f / length);
  }
  
  // The actual calculation would involve cross products and proper
  // orthonormalization, but keeping it simple for the example
  result.data[12] = -eye.x;
  result.data[13] = -eye.y;
  result.data[14] = -eye.z;
  
  (void)up;  // Would be used in real implementation
  
  return result;
}

}  // namespace game_engine