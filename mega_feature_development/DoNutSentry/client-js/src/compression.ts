/**
 * LZSS Compression wrapper for DoNutSentry using lzjs
 */

import * as lzjs from 'lzjs';

export async function compress(data: Buffer): Promise<Buffer> {
  // Convert buffer to UTF-8 string for lzjs
  const str = data.toString('utf-8');
  const compressed = lzjs.compress(str);
  // LZJS returns a binary string, use latin1 encoding
  return Buffer.from(compressed, 'latin1');
}

export async function decompress(data: Buffer): Promise<Buffer> {
  // Convert buffer to latin1 string for lzjs
  const str = data.toString('latin1');
  const decompressed = lzjs.decompress(str);
  return Buffer.from(decompressed, 'utf-8');
}

export function isCompressed(data: Buffer): boolean {
  // LZJS compressed data has specific patterns
  try {
    const str = data.toString('latin1');
    const decompressed = lzjs.decompress(str);
    // If decompression works and result differs, it's compressed
    return decompressed !== str && decompressed.length > 0;
  } catch {
    return false;
  }
}