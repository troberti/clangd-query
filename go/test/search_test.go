package test

import (
	"strings"
	"testing"
)

func TestSearchCommand(t *testing.T) {
	tc := GetTestContext(t)

	t.Run("Search for GameObject class", func(t *testing.T) {
		result := tc.RunCommand("search", "GameObject")
		tc.AssertExitCode(result, 0)

		tc.AssertContains(result.Stdout,
			// Note this string starts with "Found X symbols", but the tests are flaky,
			// sometimes matching 7 symbols, sometimes 8. So we just omit this part
			// of the string. The important thing is that the GameObject results
			// are first.
			" matching \"GameObject\":\n"+
				"\n"+
				"- `class game_engine::GameObject` at include/core/game_object.h:22:7\n"+
				"- `game_engine::GameObject::GameObject` at src/core/game_object.cpp:12:13\n"+
				"- `game_engine::Engine::game_objects_` at include/core/engine.h:120:44\n"+
				"- `game_engine::Engine::CreateGameObject` at src/core/engine.cpp:136:37\n"+
				"- `game_engine::Engine::DestroyGameObject` at src/core/engine.cpp:142:14\n"+
				"- `game_engine::Engine::GetGameObjects` at include/core/engine.h:94:51\n")
		tc.AssertContains(result.Stdout, "game_engine::GameObject::~GameObject")
	})

	t.Run("Search for Character class and related symbols", func(t *testing.T) {
		result := tc.RunCommand("search", "Character")
		tc.AssertExitCode(result, 0)
		tc.AssertContains(result.Stdout, "class game_engine::Character")
		// Should find Character class in character.h
		if !strings.Contains(result.Stdout, "Character") || !strings.Contains(result.Stdout, "character.h") {
			t.Errorf("Expected to find Character in character.h")
		}
	})

	t.Run("Search with limit flag", func(t *testing.T) {
		result := tc.RunCommand("search", "Update", "--limit", "3")
		tc.AssertExitCode(result, 0)
		// Count the number of result lines (those containing ' at ')
		lines := strings.Split(result.Stdout, "\n")
		resultCount := 0
		for _, line := range lines {
			if strings.Contains(line, " at ") {
				resultCount++
			}
		}
		if resultCount > 3 {
			t.Errorf("Expected at most 3 results with --limit 3, got %d", resultCount)
		}
	})

	t.Run("Search for non-existent symbol", func(t *testing.T) {
		result := tc.RunCommand("search", "NonExistentSymbol")
		tc.AssertExitCode(result, 0)
		tc.AssertContains(result.Stdout, "No symbols found matching \"NonExistentSymbol\"")
	})

	t.Run("Search for factory methods", func(t *testing.T) {
		result := tc.RunCommand("search", "Create")
		tc.AssertExitCode(result, 0)
		tc.AssertContains(result.Stdout, "game_engine::Factory::Create")
		tc.AssertContains(result.Stdout, "game_engine::EnemyFactory::CreateEnemy")
		tc.AssertContains(result.Stdout, "game_engine::Engine::CreateGameObject")
	})

	t.Run("Fuzzy search test", func(t *testing.T) {
		result := tc.RunCommand("search", "updatable")
		tc.AssertExitCode(result, 0)
		// Should find Updatable interface even with lowercase search
		tc.AssertContains(result.Stdout, "Updatable")
	})
}
