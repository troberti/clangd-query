package test

import (
	"testing"
)

func TestViewCommand(t *testing.T) {
	tc := GetTestContext(t)
	
	t.Run("View GameObject complete class", func(t *testing.T) {
		result := tc.RunCommand("view", "GameObject")
		tc.AssertExitCode(result, 0)
		// Should show the complete class definition
		tc.AssertContains(result.Stdout, "class GameObject : public Updatable")
		tc.AssertContains(result.Stdout, "void Update(float delta_time) override")
		tc.AssertContains(result.Stdout, "Transform transform_;")
		tc.AssertContains(result.Stdout, "std::vector<std::shared_ptr<Component>> components_;")
	})
	
	t.Run("View specific method GameObject::Update", func(t *testing.T) {
		result := tc.RunCommand("view", "GameObject::Update")
		tc.AssertExitCode(result, 0)
		// Should show the method implementation
		tc.AssertContains(result.Stdout, "void GameObject::Update(float delta_time)")
		tc.AssertContains(result.Stdout, "OnUpdate(delta_time);")
	})
	
	t.Run("View Factory class", func(t *testing.T) {
		result := tc.RunCommand("view", "Factory")
		tc.AssertExitCode(result, 0)
		// Should show the Factory class
		tc.AssertContains(result.Stdout, "class Factory")
		tc.AssertContains(result.Stdout, "std::unique_ptr<Base> Create(const std::string& type_name)")
		tc.AssertContains(result.Stdout, "void Register(const std::string& type_name, Creator creator)")
	})
	
	t.Run("View non-existent symbol", func(t *testing.T) {
		result := tc.RunCommand("view", "NonExistentClass")
		tc.AssertExitCode(result, 0)
		tc.AssertContains(result.Stdout, "No symbols found matching \"NonExistentClass\"")
	})
}