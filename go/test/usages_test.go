package test

import (
	"regexp"
	"testing"
)

func TestUsagesCommand(t *testing.T) {
	tc := GetTestContext(t)

	t.Run("Find usages of GameObject class", func(t *testing.T) {
		result := tc.RunCommand("usages", "GameObject")
		tc.AssertExitCode(result, 0)
		// Should show the selected symbol and references
		tc.AssertContains(result.Stdout, "Selected symbol: game_engine::GameObject")
		tc.AssertContains(result.Stdout, "Found 33 references:")
		// Check for some key references
		tc.AssertContains(result.Stdout, "include/core/game_object.h:")
		tc.AssertContains(result.Stdout, "include/game/character.h:") // Character inherits from GameObject
		tc.AssertContains(result.Stdout, "src/core/engine.cpp:")      // Engine uses GameObject
	})

	t.Run("Find usages of Update method", func(t *testing.T) {
		result := tc.RunCommand("usages", "Update")
		tc.AssertExitCode(result, 0)
		// Should find multiple Update methods
		tc.AssertContains(result.Stdout, "references:")
		// Update is called in various places - verify it matches pattern like "15 references:"
		matched, err := regexp.MatchString(`\d+ references:`, result.Stdout)
		if err != nil {
			t.Fatalf("Regex error: %v", err)
		}
		if !matched {
			t.Errorf("Expected output to match pattern '\\d+ references:', got:\n%s", result.Stdout)
		}
	})

	t.Run("Find usages of Transform class", func(t *testing.T) {
		result := tc.RunCommand("usages", "Transform")
		tc.AssertExitCode(result, 0)
		// Should find Transform usages
		tc.AssertContains(result.Stdout, "Selected symbol: game_engine::Transform")
		tc.AssertContains(result.Stdout, "references:")
		// Transform is used in GameObject
		tc.AssertContains(result.Stdout, "include/core/game_object.h:")
	})

	t.Run("Find usages of non-existent symbol", func(t *testing.T) {
		result := tc.RunCommand("usages", "NonExistentSymbol")
		tc.AssertExitCode(result, 0)
		tc.AssertContains(result.Stdout, "No symbols found matching \"NonExistentSymbol\"")
	})
}
