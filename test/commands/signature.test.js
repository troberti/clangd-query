import {
  runClangdQuery,
  assert,
  assertContains,
  assertMatches,
  countOccurrences,
  waitForDaemonReady,
} from '../test-helpers.js';

console.log('Testing signature command...\n');

// Ensure daemon is ready before running tests
console.log('Waiting for daemon to be ready...');
await waitForDaemonReady();

// Test 1: Get signature of a constructor
console.log('Test 1: Get signature of GameObject constructor');
const result1 = await runClangdQuery(['signature', 'GameObject']);
assert(result1.exitCode === 0, 'Command should succeed');
// Should show the constructor signature
assertContains(result1.stdout, 'GameObject::GameObject(const std::string &name)');
assertContains(result1.stdout, 'Parameters:');
assertContains(result1.stdout, 'const std::string & name');
assertContains(result1.stdout, 'Modifiers: explicit');
console.log('✓ Test 1 passed\n');

// Test 2: Get signature of a method
console.log('Test 2: Get signature of Update method');
const result2 = await runClangdQuery(['signature', 'Update']);
assert(result2.exitCode === 0, 'Command should succeed');
// Should show Update method signatures
assertContains(result2.stdout, 'Update');
assertContains(result2.stdout, 'Parameters:');
assertContains(result2.stdout, 'float delta_time');
console.log('✓ Test 2 passed\n');

// Test 3: Get signature of a template function
console.log('Test 3: Get signature of GetComponent template method');
const result3 = await runClangdQuery(['signature', 'GetComponent']);
assert(result3.exitCode === 0, 'Command should succeed');
// Should show the template method signature
assertContains(result3.stdout, 'GetComponent');
assertContains(result3.stdout, 'Template Parameters: <typename T>');
assertContains(result3.stdout, 'std::optional<std::shared_ptr<T>>');
console.log('✓ Test 3 passed\n');

// Test 4: Get signature of non-existent function
console.log('Test 4: Get signature of non-existent function');
const result4 = await runClangdQuery(['signature', 'NonExistentFunction']);
assert(result4.exitCode === 0, 'Command should succeed even with no results');
assertContains(result4.stdout, "No function or method named 'NonExistentFunction' found");
console.log('✓ Test 4 passed\n');

console.log('All signature tests passed! ✓');