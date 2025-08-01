import {
  runClangdQuery,
  assert,
  assertContains,
  assertMatches,
  countOccurrences,
  waitForDaemonReady,
} from '../test-helpers.js';

console.log('Testing hierarchy command...\n');

// Ensure daemon is ready before running tests
console.log('Waiting for daemon to be ready...');
await waitForDaemonReady();

// Test 1: View hierarchy of a derived class
console.log('Test 1: View hierarchy of Character class');
const result1 = await runClangdQuery(['hierarchy', 'Character']);
assert(result1.exitCode === 0, 'Command should succeed');
// Should show inheritance hierarchy
assertContains(result1.stdout, 'Inherits from:');
assertContains(result1.stdout, '└── GameObject');
assertContains(result1.stdout, 'Character - include/game/character.h');
assertContains(result1.stdout, '├── Enemy');
assertContains(result1.stdout, '└── Player');
console.log('✓ Test 1 passed\n');

console.log('All hierarchy tests passed! ✓');