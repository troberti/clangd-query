import {
  runClangdQuery,
  assert,
  assertContains,
  assertMatches,
  countOccurrences,
  waitForDaemonReady,
} from '../test-helpers.js';

console.log('Testing search command...\n');

// Ensure daemon is ready before running tests
console.log('Waiting for daemon to be ready...');
await waitForDaemonReady();

// Test 1: Search for GameObject class
console.log('Test 1: Search for GameObject class');
const result1 = await runClangdQuery(['search', 'GameObject']);
assert(result1.exitCode === 0, 'Command should succeed');
assertContains(result1.stdout, 'class game_engine::GameObject');
assertContains(result1.stdout, 'game_engine::GameObject::GameObject');
assertContains(result1.stdout, 'game_engine::Engine::CreateGameObject');
assertContains(result1.stdout, 'game_engine::Engine::DestroyGameObject');
assertContains(result1.stdout, 'game_engine::Engine::GetGameObject');
assertContains(result1.stdout, 'game_engine::GameObject::~GameObject');

console.log('✓ Test 1 passed\n');

// Test 2: Search for Character with inheritance
console.log('Test 2: Search for Character class and related symbols');
const result2 = await runClangdQuery(['search', 'Character']);
assert(result2.exitCode === 0, 'Command should succeed');
assertContains(result2.stdout, 'class game_engine::Character');
// Should find Character class and its constructors/destructors
assertMatches(result2.stdout, /Character.*character\.h/);
console.log('✓ Test 2 passed\n');

// Test 3: Search with limit flag
console.log('Test 3: Search with --limit flag');
const result3 = await runClangdQuery(['search', 'Update', '--limit', '3']);
assert(result3.exitCode === 0, 'Command should succeed');
// Count the number of result lines (excluding header)
const lines3 = result3.stdout.split('\n').filter(line => line.includes(' at '));
assert(lines3.length <= 3, `Should return at most 3 results, got ${lines3.length}`);
console.log('✓ Test 3 passed\n');

// Test 4: Search for non-existent symbol
console.log('Test 4: Search for non-existent symbol');
const result4 = await runClangdQuery(['search', 'NonExistentSymbol']);
assert(result4.exitCode === 0, 'Command should succeed even with no results');
assertContains(result4.stdout, 'No symbols found matching "NonExistentSymbol"');
console.log('✓ Test 4 passed\n');

// Test 5: Search for factory methods
console.log('Test 5: Search for factory methods');
const result5 = await runClangdQuery(['search', 'Create']);
assert(result5.exitCode === 0, 'Command should succeed');
// Should find factory Create methods
assertContains(result5.stdout, 'game_engine::Factory::Create');
assertContains(result5.stdout, 'game_engine::EnemyFactory::CreateEnemy');
assertContains(result5.stdout, 'game_engine::Engine::CreateGameObject');
console.log('✓ Test 5 passed\n');

// Test 6: Case-insensitive fuzzy search
console.log('Test 6: Fuzzy search test');
const result6 = await runClangdQuery(['search', 'updatable']);
assert(result6.exitCode === 0, 'Command should succeed');
// Should find Updatable interface even with lowercase search
assertContains(result6.stdout, 'Updatable');
console.log('✓ Test 6 passed\n');

console.log('All search tests passed! ✓');