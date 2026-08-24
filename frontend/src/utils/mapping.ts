import type { DatabasePair } from '../types';

/**
 * Builds the source->target mapping after a selection change, preserving
 * target names the user already edited for databases that stay selected.
 */
export function buildMapping(selected: string[], prev: DatabasePair[]): DatabasePair[] {
  const prevMap = new Map(prev.map((p) => [p.source, p.target]));
  return selected.map((db) => ({ source: db, target: prevMap.get(db) || db }));
}

/**
 * Validates a mapping list. Returns an error message or null when valid.
 */
export function validateMapping(pairs: DatabasePair[]): string | null {
  const valid = pairs.filter((p) => p.source && p.target);
  if (valid.length === 0) {
    return 'Select at least one source database and set its target name';
  }
  const targets = valid.map((p) => p.target);
  if (new Set(targets).size !== targets.length) {
    return 'Target database names must be unique';
  }
  return null;
}
