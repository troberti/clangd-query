import {
  runClangdQuery,
  assert,
  assertContains,
  assertMatches,
  countOccurrences,
  waitForDaemonReady,
} from '../test-helpers.js';

console.log('Testing view command...\n');

// Ensure daemon is ready before running tests
console.log('Waiting for daemon to be ready...');
await waitForDaemonReady();

// Test 1: View a complete class
console.log('Test 1: View GameObject class');
const result1 = await runClangdQuery(['view', 'GameObject']);
assert(result1.exitCode === 0, 'Command should succeed');
// Should show the complete class definition
assertContains(result1.stdout, 'class GameObject : public Updatable');
assertContains(result1.stdout, 'void Update(float delta_time) override');
assertContains(result1.stdout, 'Transform transform_;');
assertContains(result1.stdout, 'std::vector<std::shared_ptr<Component>> components_;');
console.log('✓ Test 1 passed\n');

// Test 2: View a specific method
console.log('Test 2: View specific method GameObject::Update');
const result2 = await runClangdQuery(['view', 'GameObject::Update']);
assert(result2.exitCode === 0, 'Command should succeed');
// Should show the method implementation
assertContains(result2.stdout, 'void GameObject::Update(float delta_time)');
assertContains(result2.stdout, 'OnUpdate(delta_time);');
console.log('✓ Test 2 passed\n');

// Test 3: View a regular class (Factory)
console.log('Test 3: View Factory class');
const result3 = await runClangdQuery(['view', 'Factory']);
assert(result3.exitCode === 0, 'Command should succeed');
// Should show the Factory class
assertContains(result3.stdout, 'class Factory');
assertContains(result3.stdout, 'std::unique_ptr<Base> Create(const std::string& type_name)');
assertContains(result3.stdout, 'void Register(const std::string& type_name, Creator creator)');
console.log('✓ Test 3 passed\n');

// Test 4: View non-existent symbol
console.log('Test 4: View non-existent symbol');
const result4 = await runClangdQuery(['view', 'NonExistentClass']);
assert(result4.exitCode === 0, 'Command should succeed even with no results');
assertContains(result4.stdout, 'No symbols found matching "NonExistentClass"');
console.log('✓ Test 4 passed\n');

console.log('All view tests passed! ✓');