import {
  runClangdQuery,
  assert,
  assertContains,
  assertMatches,
  countOccurrences,
  waitForDaemonReady,
} from '../test-helpers.js';

console.log('Testing interface command...\n');

// Ensure daemon is ready before running tests
console.log('Waiting for daemon to be ready...');
await waitForDaemonReady();

// Test 1: Get interface of GameObject class
console.log('Test 1: Get interface of GameObject class');
const result1 = await runClangdQuery(['interface', 'GameObject']);
assert(result1.exitCode === 0, 'Command should succeed');
// Should show public interface
assertContains(result1.stdout, 'class game_engine::GameObject');
assertContains(result1.stdout, 'Public Interface:');
// Should show public methods
assertContains(result1.stdout, 'explicit GameObject(const std::string &name)');
assertContains(result1.stdout, 'void Update(float delta_time) override');
assertContains(result1.stdout, 'uint64_t GetId() const');
assertContains(result1.stdout, 'void AddComponent(std::shared_ptr<Component> component)');
// Should show template method
assertContains(result1.stdout, 'template <typename T>');
assertContains(result1.stdout, 'std::optional<std::shared_ptr<T>> GetComponent() const');
console.log('✓ Test 1 passed\n');

// Test 2: Get interface of Engine class
console.log('Test 2: Get interface of Engine class');
const result2 = await runClangdQuery(['interface', 'Engine']);
assert(result2.exitCode === 0, 'Command should succeed');
// Should show Engine's public interface
assertContains(result2.stdout, 'class game_engine::Engine');
assertContains(result2.stdout, 'bool Initialize(std::optional<std::string> config_file');
assertContains(result2.stdout, 'void Run()');
assertContains(result2.stdout, 'void Stop()');
assertContains(result2.stdout, 'std::shared_ptr<GameObject> CreateGameObject(const std::string &name)');
console.log('✓ Test 2 passed\n');

// Test 3: Get interface of abstract class (Updatable)
console.log('Test 3: Get interface of abstract Updatable class');
const result3 = await runClangdQuery(['interface', 'Updatable']);
assert(result3.exitCode === 0, 'Command should succeed');
// Should show the interface
assertContains(result3.stdout, 'class game_engine::Updatable');
assertContains(result3.stdout, 'virtual void Update(float delta_time) = 0');
assertContains(result3.stdout, 'virtual bool IsActive() const = 0');
console.log('✓ Test 3 passed\n');

// Test 4: Get interface of non-existent class
console.log('Test 4: Get interface of non-existent class');
const result4 = await runClangdQuery(['interface', 'NonExistentClass']);
assert(result4.exitCode === 0, 'Command should succeed even with no results');
assertContains(result4.stdout, "No class or struct named 'NonExistentClass' found");
console.log('✓ Test 4 passed\n');

console.log('All interface tests passed! ✓');