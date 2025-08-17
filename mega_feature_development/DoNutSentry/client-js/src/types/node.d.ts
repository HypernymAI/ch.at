// Additional Node.js crypto types that might be missing

declare module 'crypto' {
  interface KeyPairSyncResult<T1, T2> {
    publicKey: T1;
    privateKey: T2;
  }
}