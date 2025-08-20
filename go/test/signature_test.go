package test

import (
	"testing"
)

func TestSignatureCommand(t *testing.T) {
	tc := NewTestContext(t)
	
	// Ensure daemon is ready before running tests
	t.Log("Waiting for daemon to be ready...")
	tc.WaitForDaemonReady()
	
	t.Run("Get signature of GameObject constructor", func(t *testing.T) {
		result := tc.RunCommand("signature", "GameObject")
		tc.AssertExitCode(result, 0)
		// Should show the constructor signature
		tc.AssertContains(result.Stdout, "GameObject::GameObject(const std::string &name)")
		tc.AssertContains(result.Stdout, "Parameters:")
		tc.AssertContains(result.Stdout, "const std::string & name")
		tc.AssertContains(result.Stdout, "Modifiers: explicit")
	})
	
	t.Run("Get signature of Update method", func(t *testing.T) {
		result := tc.RunCommand("signature", "Update")
		tc.AssertExitCode(result, 0)
		// Should show Update method signatures
		tc.AssertContains(result.Stdout, "Update")
		tc.AssertContains(result.Stdout, "Parameters:")
		tc.AssertContains(result.Stdout, "float delta_time")
	})
	
	t.Run("Get signature of GetComponent template method", func(t *testing.T) {
		result := tc.RunCommand("signature", "GetComponent")
		tc.AssertExitCode(result, 0)
		// Should show the template method signature
		tc.AssertContains(result.Stdout, "GetComponent")
		tc.AssertContains(result.Stdout, "Template Parameters: <typename T>")
		tc.AssertContains(result.Stdout, "std::optional<std::shared_ptr<T>>")
	})
	
	t.Run("Get signature of non-existent function", func(t *testing.T) {
		result := tc.RunCommand("signature", "NonExistentFunction")
		tc.AssertExitCode(result, 0)
		tc.AssertContains(result.Stdout, "No function or method named 'NonExistentFunction' found")
	})
	
	// Additional test: overloaded methods
	t.Run("Get signature of overloaded method", func(t *testing.T) {
		result := tc.RunCommand("signature", "Initialize")
		tc.AssertExitCode(result, 0)
		// Should show Initialize method signature
		tc.AssertContains(result.Stdout, "Initialize")
		tc.AssertContains(result.Stdout, "Parameters:")
	})
}