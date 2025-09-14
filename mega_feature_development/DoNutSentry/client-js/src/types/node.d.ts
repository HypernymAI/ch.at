// Additional Node.js crypto types that might be missing

declare module 'crypto' {
  interface KeyPairSyncResult<T1, T2> {
    publicKey: T1;
    privateKey: T2;
  }
}

// Add Web Crypto types for Node.js
declare global {
  type CryptoKey = any;
}