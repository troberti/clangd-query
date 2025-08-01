#ifndef RENDERING_CAMERA_H_
#define RENDERING_CAMERA_H_

#include <span>

#include "core/component.h"
#include "core/transform.h"

namespace game_engine {

// Camera component for viewing the game world
class Camera : public Component {
 public:
  enum class ProjectionType {
    Perspective,
    Orthographic
  };

  Camera();
  ~Camera() override;

  // Projection settings
  void SetProjectionType(ProjectionType type) { projection_type_ = type; }
  ProjectionType GetProjectionType() const { return projection_type_; }

  // Field of view (for perspective projection)
  void SetFieldOfView(float fov) { field_of_view_ = fov; }
  float GetFieldOfView() const { return field_of_view_; }

  // Near/far clipping planes
  void SetNearPlane(float near_plane) { near_plane_ = near_plane; }
  float GetNearPlane() const { return near_plane_; }

  void SetFarPlane(float far_plane) { far_plane_ = far_plane; }
  float GetFarPlane() const { return far_plane_; }

  // Viewport
  void SetViewport(int x, int y, int width, int height) {
    viewport_x_ = x;
    viewport_y_ = y;
    viewport_width_ = width;
    viewport_height_ = height;
  }

  // Gets the aspect ratio
  float GetAspectRatio() const {
    return viewport_height_ > 0 
        ? static_cast<float>(viewport_width_) / viewport_height_ 
        : 1.0f;
  }

  // Camera matrices (simplified - would be 4x4 in real engine)
  struct Matrix4x4 {
    float data[16] = {
      1, 0, 0, 0,
      0, 1, 0, 0,
      0, 0, 1, 0,
      0, 0, 0, 1
    };
    
    // Access data as a span
    std::span<float, 16> AsSpan() { return std::span<float, 16>(data); }
    std::span<const float, 16> AsSpan() const { return std::span<const float, 16>(data); }
  };

  Matrix4x4 GetViewMatrix() const;
  Matrix4x4 GetProjectionMatrix() const;

 protected:
  void OnUpdate(float delta_time) override;

 private:
  ProjectionType projection_type_ = ProjectionType::Perspective;
  float field_of_view_ = 60.0f;
  float near_plane_ = 0.1f;
  float far_plane_ = 1000.0f;
  
  int viewport_x_ = 0;
  int viewport_y_ = 0;
  int viewport_width_ = 1280;
  int viewport_height_ = 720;
};

// Free function to create a look-at matrix
Camera::Matrix4x4 LookAt(const Vector3& eye, const Vector3& target, const Vector3& up);

}  // namespace game_engine

#endif  // RENDERING_CAMERA_H_