declare module 'lzjs' {
  export function compress(data: string): string;
  export function decompress(data: string): string;
  export function compressToBase64(data: string): string;
  export function decompressFromBase64(data: string): string;
}