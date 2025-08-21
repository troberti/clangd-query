package test

import (
	"testing"
)

func TestShowCommand(t *testing.T) {
	tc := GetTestContext(t)
	
	t.Run("Show GameObject::Update method with separate declaration/definition", func(t *testing.T) {
		result := tc.RunCommand("show", "GameObject::Update")
		tc.AssertExitCode(result, 0)
		// Should show both declaration and definition
		tc.AssertContains(result.Stdout, "From include/core/game_object.h")
		tc.AssertContains(result.Stdout, "(declaration)")
		tc.AssertContains(result.Stdout, "void Update(float delta_time) override;")
		tc.AssertContains(result.Stdout, "From src/core/game_object.cpp")
		tc.AssertContains(result.Stdout, "(definition)")
		tc.AssertContains(result.Stdout, "void GameObject::Update(float delta_time) {")
		tc.AssertContains(result.Stdout, "OnUpdate(delta_time);")
	})
	
	t.Run("Show inline IsActive method", func(t *testing.T) {
		result := tc.RunCommand("show", "GameObject::IsActive")
		tc.AssertExitCode(result, 0)
		// Should show the inline definition from header
		tc.AssertContains(result.Stdout, "From include/core/game_object.h")
		tc.AssertContains(result.Stdout, "bool IsActive() const override { return active_; }")
	})
	
	t.Run("Show template GetComponent method", func(t *testing.T) {
		result := tc.RunCommand("show", "GetComponent")
		tc.AssertExitCode(result, 0)
		// Should show the template method declaration and definition
		tc.AssertContains(result.Stdout, "std::optional<std::shared_ptr<T>> GetComponent() const")
		tc.AssertContains(result.Stdout, "GameObject::GetComponent() const {")
		tc.AssertContains(result.Stdout, "std::dynamic_pointer_cast<T>")
	})
	
	t.Run("Show Engine class complete implementation", func(t *testing.T) {
		result := tc.RunCommand("show", "Engine")
		tc.AssertExitCode(result, 0)
		// Should show the complete class
		tc.AssertContains(result.Stdout, "class Engine {")
		tc.AssertContains(result.Stdout, "static Engine& GetInstance();")
		tc.AssertContains(result.Stdout, "bool Initialize(")
		tc.AssertContains(result.Stdout, "void Shutdown();")
		tc.AssertContains(result.Stdout, "private:")
		tc.AssertContains(result.Stdout, "std::unique_ptr<RenderSystem> render_system_;")
		tc.AssertContains(result.Stdout, "static constexpr float kFixedTimeStep")
		// Verify it shows the closing brace
		tc.AssertContains(result.Stdout, "};")
	})
	
	t.Run("Show non-existent symbol", func(t *testing.T) {
		result := tc.RunCommand("show", "NonExistentMethod")
		tc.AssertExitCode(result, 0)
		tc.AssertContains(result.Stdout, "No symbols found matching \"NonExistentMethod\"")
	})
	
	// Additional test: Show Transform class
	t.Run("Show Transform class", func(t *testing.T) {
		result := tc.RunCommand("show", "Transform")
		tc.AssertExitCode(result, 0)
		// Should show the Transform class (it's a class, not a struct in transform.h)
		tc.AssertContains(result.Stdout, "class Transform")
		// Check for private member variables with underscore suffix
		tc.AssertContains(result.Stdout, "position_")
		tc.AssertContains(result.Stdout, "rotation_")
		tc.AssertContains(result.Stdout, "scale_")
	})
	
	// Additional test: Show actual struct (Vector3)
	t.Run("Show Vector3 struct", func(t *testing.T) {
		result := tc.RunCommand("show", "Vector3")
		tc.AssertExitCode(result, 0)
		// Should show the Vector3 struct
		tc.AssertContains(result.Stdout, "struct Vector3")
		tc.AssertContains(result.Stdout, "float x")
		tc.AssertContains(result.Stdout, "float y")
		tc.AssertContains(result.Stdout, "float z")
	})
}