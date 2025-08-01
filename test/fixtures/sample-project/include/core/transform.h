#ifndef CORE_TRANSFORM_H_
#define CORE_TRANSFORM_H_

#include <format>
#include <string>

namespace game_engine {

/**
 * @brief Represents a 3D vector
 */
struct Vector3 {
  float x = 0.0f;
  float y = 0.0f;
  float z = 0.0f;

  Vector3() = default;
  Vector3(float x, float y, float z) : x(x), y(y), z(z) {}

  // Three-way comparison
  auto operator<=>(const Vector3&) const = default;

  // Arithmetic operators
  Vector3 operator+(const Vector3& other) const {
    return {x + other.x, y + other.y, z + other.z};
  }

  Vector3 operator-(const Vector3& other) const {
    return {x - other.x, y - other.y, z - other.z};
  }

  Vector3 operator*(float scalar) const {
    return {x * scalar, y * scalar, z * scalar};
  }

  Vector3 operator/(float scalar) const {
    return {x / scalar, y / scalar, z / scalar};
  }

  Vector3& operator+=(const Vector3& other) {
    x += other.x;
    y += other.y;
    z += other.z;
    return *this;
  }

  /**
   * @brief Formats the vector as a string
   * @return String representation of the vector
   */
  std::string ToString() const {
    return std::format("({:.2f}, {:.2f}, {:.2f})", x, y, z);
  }
};

/**
 * @brief Represents position, rotation and scale in 3D space
 */
class Transform {
 public:
  Transform() = default;
  
  /**
   * @brief Constructs a transform with specified position
   * @param position Initial position
   */
  explicit Transform(const Vector3& position)
      : position_(position) {}

  // Accessors
  const Vector3& GetPosition() const { return position_; }
  const Vector3& GetRotation() const { return rotation_; }
  const Vector3& GetScale() const { return scale_; }

  // Mutators
  void SetPosition(const Vector3& position) { position_ = position; }
  void SetRotation(const Vector3& rotation) { rotation_ = rotation; }
  void SetScale(const Vector3& scale) { scale_ = scale; }

  /**
   * @brief Translates the transform by the given offset
   * @param offset Translation offset
   */
  void Translate(const Vector3& offset) {
    position_ += offset;
  }

  /**
   * @brief Rotates the transform by the given angles (in degrees)
   * @param angles Rotation angles for each axis
   */
  void Rotate(const Vector3& angles) {
    rotation_ += angles;
  }

  /**
   * @brief Resets the transform to identity
   */
  void Reset() {
    position_ = Vector3{};
    rotation_ = Vector3{};
    scale_ = Vector3{1.0f, 1.0f, 1.0f};
  }

  /**
   * @brief Formats the transform as a string
   * @return String representation
   */
  std::string ToString() const {
    return std::format("Transform[pos:{}, rot:{}, scale:{}]",
                       position_.ToString(),
                       rotation_.ToString(),
                       scale_.ToString());
  }

 private:
  Vector3 position_{0.0f, 0.0f, 0.0f};
  Vector3 rotation_{0.0f, 0.0f, 0.0f};  // Euler angles in degrees
  Vector3 scale_{1.0f, 1.0f, 1.0f};
};

}  // namespace game_engine

#endif  // CORE_TRANSFORM_H_