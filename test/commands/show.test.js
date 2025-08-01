import {
  runClangdQuery,
  assert,
  assertContains,
  assertMatches,
  countOccurrences,
  waitForDaemonReady,
} from '../test-helpers.js';

console.log('Testing show command...\n');

// Ensure daemon is ready before running tests
console.log('Waiting for daemon to be ready...');
await waitForDaemonReady();

// Test 1: Show a method with separate declaration/definition
console.log('Test 1: Show GameObject::Update method');
const result1 = await runClangdQuery(['show', 'GameObject::Update']);
assert(result1.exitCode === 0, 'Command should succeed');
// Should show both declaration and definition
assertContains(result1.stdout, 'From include/core/game_object.h');
assertContains(result1.stdout, '(declaration)');
assertContains(result1.stdout, 'void Update(float delta_time) override;');
assertContains(result1.stdout, 'From src/core/game_object.cpp');
assertContains(result1.stdout, '(definition)');
assertContains(result1.stdout, 'void GameObject::Update(float delta_time) {');
assertContains(result1.stdout, 'OnUpdate(delta_time);');
console.log('✓ Test 1 passed\n');

// Test 2: Show an inline method (only declaration)
console.log('Test 2: Show inline IsActive method');
const result2 = await runClangdQuery(['show', 'GameObject::IsActive']);
assert(result2.exitCode === 0, 'Command should succeed');
// Should show the inline definition from header
assertContains(result2.stdout, 'From include/core/game_object.h');
assertContains(result2.stdout, 'bool IsActive() const override { return active_; }');
console.log('✓ Test 2 passed\n');

// Test 3: Show a template method
console.log('Test 3: Show template GetComponent method');
const result3 = await runClangdQuery(['show', 'GetComponent']);
assert(result3.exitCode === 0, 'Command should succeed');
// Should show the template method declaration and definition
assertContains(result3.stdout, 'std::optional<std::shared_ptr<T>> GetComponent() const');
assertContains(result3.stdout, 'GameObject::GetComponent() const {');
assertContains(result3.stdout, 'std::dynamic_pointer_cast<T>');
console.log('✓ Test 3 passed\n');

// Test 4: Show non-existent symbol
console.log('Test 4: Show non-existent symbol');
const result4 = await runClangdQuery(['show', 'NonExistentMethod']);
assert(result4.exitCode === 0, 'Command should succeed even with no results');
assertContains(result4.stdout, 'No symbols found matching "NonExistentMethod"');
console.log('✓ Test 4 passed\n');

console.log('All show tests passed! ✓');