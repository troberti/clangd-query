import {
  runClangdQuery,
  assert,
  assertContains,
  assertMatches,
  countOccurrences,
  waitForDaemonReady,
} from '../test-helpers.js';

console.log('Testing usages command...\n');

// Ensure daemon is ready before running tests
console.log('Waiting for daemon to be ready...');
await waitForDaemonReady();

// Test 1: Find usages of GameObject class
console.log('Test 1: Find usages of GameObject');
const result1 = await runClangdQuery(['usages', 'GameObject']);
assert(result1.exitCode === 0, 'Command should succeed');
// Should show the selected symbol and references
assertContains(result1.stdout, 'Selected symbol: game_engine::GameObject');
assertContains(result1.stdout, 'Found 33 references:');
// Check for some key references
assertContains(result1.stdout, 'include/core/game_object.h:');
assertContains(result1.stdout, 'include/game/character.h:');  // Character inherits from GameObject
assertContains(result1.stdout, 'src/core/engine.cpp:');  // Engine uses GameObject
console.log('✓ Test 1 passed\n');

// Test 2: Find usages of a method
console.log('Test 2: Find usages of Update method');
const result2 = await runClangdQuery(['usages', 'Update']);
assert(result2.exitCode === 0, 'Command should succeed');
// Should find multiple Update methods
assertContains(result2.stdout, 'references:');
// Update is called in various places
assertMatches(result2.stdout, /\d+ references:/);
console.log('✓ Test 2 passed\n');

// Test 3: Find usages of Transform (a smaller class)
console.log('Test 3: Find usages of Transform');
const result3 = await runClangdQuery(['usages', 'Transform']);
assert(result3.exitCode === 0, 'Command should succeed');
// Should find Transform usages
assertContains(result3.stdout, 'Selected symbol: game_engine::Transform');
assertContains(result3.stdout, 'references:');
// Transform is used in GameObject
assertContains(result3.stdout, 'include/core/game_object.h:');
console.log('✓ Test 3 passed\n');

// Test 4: Find usages of non-existent symbol
console.log('Test 4: Find usages of non-existent symbol');
const result4 = await runClangdQuery(['usages', 'NonExistentSymbol']);
assert(result4.exitCode === 0, 'Command should succeed even with no results');
assertContains(result4.stdout, 'No symbols found matching "NonExistentSymbol"');
console.log('✓ Test 4 passed\n');

console.log('All usages tests passed! ✓');