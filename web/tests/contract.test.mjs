import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { parseSchemaJSON, stringifyRequest } from '../src/api/json.ts';

test('editing an unrelated field preserves the shared test patch', () => {
  const raw = readFileSync(new URL('../../pipelines/live/tests/platform/contracts/test-patch.json', import.meta.url), 'utf8');
  const original = JSON.parse(raw);
  const edited = parseSchemaJSON(raw);
  edited.name = 'edited title';
  const saved = JSON.parse(stringifyRequest(edited));
  delete saved.name;
  assert.deepEqual(saved, original);
  assert.equal(saved.execution.workload.segments[0].seed, '18446744073709551615');
  assert.equal(saved.execution.machines['db-1'].preemptible, false);
  assert.equal(saved.execution.workload.segments[0].thresholds.error_rate, 0);
  assert.equal(saved.execution.containers[0].set.files[1].content, '');
  assert.equal(saved.execution.containers[0].set.healthcheck, null);
});

test('empty overrides and exact integer strings remain distinct from absence', () => {
  const value = {env: {}, files: [], flag: false, zero: 0, seed: 18446744073709551615n};
  assert.deepEqual(JSON.parse(stringifyRequest(value)), {...value, seed: '18446744073709551615'});
  assert.equal(stringifyRequest({}), '{}');
});

test('rounded numbers cannot silently cross the request boundary', () => {
  assert.throws(() => parseSchemaJSON('{"seed":18446744073709551615}'), RangeError);
  assert.throws(() => stringifyRequest({nested: [Number.MAX_SAFE_INTEGER + 1]}), RangeError);
  assert.throws(() => stringifyRequest({value: NaN}), RangeError);
});
