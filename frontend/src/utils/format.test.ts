import { describe, expect, it } from 'vitest';
import { formatBytes, formatDuration, formatSpeed, formatTime } from './format';

describe('formatBytes', () => {
  it('formats zero and small values', () => {
    expect(formatBytes(0)).toBe('0 B');
    expect(formatBytes(512)).toBe('512 B');
  });
  it('formats binary units', () => {
    expect(formatBytes(1024)).toBe('1.0 KB');
    expect(formatBytes(1536)).toBe('1.5 KB');
    expect(formatBytes(1048576)).toBe('1.0 MB');
    expect(formatBytes(19327352832)).toBe('18.0 GB');
  });
});

describe('formatSpeed', () => {
  it('returns a dash for missing speed', () => {
    expect(formatSpeed(0)).toBe('-');
  });
  it('formats bytes per second', () => {
    expect(formatSpeed(1048576)).toBe('1.0 MB/s');
  });
});

describe('formatDuration', () => {
  it('returns a dash without start time', () => {
    expect(formatDuration(null, null)).toBe('-');
    expect(formatDuration(undefined, null)).toBe('-');
  });
  it('computes elapsed seconds', () => {
    const start = new Date('2026-08-24T00:00:00Z');
    const end = new Date('2026-08-24T00:00:45Z');
    expect(formatDuration(start.toISOString(), end.toISOString())).toBe('45s');
  });
  it('computes minutes and hours', () => {
    const start = new Date('2026-08-24T00:00:00Z');
    const end = new Date('2026-08-24T00:05:03Z');
    expect(formatDuration(start.toISOString(), end.toISOString())).toBe('5m 3s');
    const end2 = new Date('2026-08-24T01:02:03Z');
    expect(formatDuration(start.toISOString(), end2.toISOString())).toBe('1h 2m');
  });
});

describe('formatTime', () => {
  it('returns a dash for missing input', () => {
    expect(formatTime(null)).toBe('-');
    expect(formatTime(undefined)).toBe('-');
  });
  it('formats an ISO timestamp', () => {
    expect(formatTime('2026-08-24T08:00:00Z')).toMatch(/2026/);
  });
});
