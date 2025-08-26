/**
 * Tests for compression module
 */

import { compress, decompress, isCompressed } from '../compression';

describe('LZSS Compression', () => {
  test('compresses and decompresses text correctly', async () => {
    const original = Buffer.from('Hello world! This is a test of LZSS compression.');
    const compressed = await compress(original);
    const decompressed = await decompress(compressed);

    expect(decompressed.toString()).toBe(original.toString());
    expect(compressed.length).toBeLessThan(original.length);
  });

  test('handles repetitive text efficiently', async () => {
    const original = Buffer.from('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa');
    const compressed = await compress(original);

    expect(compressed.length).toBeLessThan(original.length / 2);
  });

  test('detects compressed data', async () => {
    const text = Buffer.from('Hello world!');
    const compressed = await compress(text);

    expect(isCompressed(compressed)).toBe(true);
    expect(isCompressed(text)).toBe(false);
  });

  test('handles empty buffer', async () => {
    const empty = Buffer.from('');
    const compressed = await compress(empty);
    const decompressed = await decompress(compressed);

    expect(decompressed.toString()).toBe('');
  });

  test('handles text that looks like binary', async () => {
    // LZJS is designed for text, not arbitrary binary
    const text = Buffer.from('Hello\x00World\xFF', 'utf-8');
    const compressed = await compress(text);
    const decompressed = await decompress(compressed);

    expect(decompressed.toString('utf-8')).toBe(text.toString('utf-8'));
  });
});