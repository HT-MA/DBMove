import { describe, expect, it } from 'vitest';
import { buildMapping, validateMapping } from './mapping';

describe('buildMapping', () => {
  it('maps every selected database to itself by default', () => {
    expect(buildMapping(['a', 'b'], [])).toEqual([
      { source: 'a', target: 'a' },
      { source: 'b', target: 'b' },
    ]);
  });
  it('preserves edited target names for databases that stay selected', () => {
    const prev = [
      { source: 'a', target: 'a_target' },
      { source: 'b', target: 'b' },
    ];
    expect(buildMapping(['a', 'c'], prev)).toEqual([
      { source: 'a', target: 'a_target' },
      { source: 'c', target: 'c' },
    ]);
  });
  it('returns an empty mapping for no selection', () => {
    expect(buildMapping([], [{ source: 'a', target: 'x' }])).toEqual([]);
  });
});

describe('validateMapping', () => {
  it('accepts a valid mapping', () => {
    expect(
      validateMapping([
        { source: 'a', target: 't1' },
        { source: 'b', target: 't2' },
      ])
    ).toBeNull();
  });
  it('rejects an empty mapping', () => {
    expect(validateMapping([])).toMatch(/at least one source database/i);
    expect(validateMapping([{ source: '', target: 'x' }])).toMatch(/at least one source database/i);
  });
  it('rejects duplicate target names', () => {
    expect(
      validateMapping([
        { source: 'a', target: 'same' },
        { source: 'b', target: 'same' },
      ])
    ).toMatch(/unique/i);
  });
});
