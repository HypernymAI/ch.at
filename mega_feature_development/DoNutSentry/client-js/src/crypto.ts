/**
 * Cryptographic utilities for DonutSentry v2
 * Uses Web Crypto API for RSA operations
 */

import * as base32 from 'hi-base32';

// For Node.js environment
declare global {
  var crypto: any;
}

export interface KeyPair {
  publicKey: any; // CryptoKey
  privateKey: any; // CryptoKey
}

/**
 * Generate RSA-2048 keypair for session
 */
export async function generateKeyPair(): Promise<KeyPair> {
  const keyPair = await crypto.subtle.generateKey(
    {
      name: "RSA-OAEP",
      modulusLength: 2048,
      publicExponent: new Uint8Array([1, 0, 1]),
      hash: "SHA-256",
    },
    true,
    ["encrypt", "decrypt"]
  );

  // Also generate signing keys
  const signingKeyPair = await crypto.subtle.generateKey(
    {
      name: "RSA-PSS",
      modulusLength: 2048,
      publicExponent: new Uint8Array([1, 0, 1]),
      hash: "SHA-256",
    },
    true,
    ["sign", "verify"]
  );

  // For simplicity, we'll use same key for both (in production, use separate keys)
  return {
    publicKey: keyPair.publicKey,
    privateKey: keyPair.privateKey
  };
}

/**
 * Export public key to DER format
 */
export async function exportPublicKey(key: any): Promise<Uint8Array> {
  const exported = await crypto.subtle.exportKey("spki", key);
  return new Uint8Array(exported);
}

/**
 * Import public key from DER format
 */
export async function importPublicKey(der: Uint8Array): Promise<any> {
  return await crypto.subtle.importKey(
    "spki",
    der,
    {
      name: "RSA-OAEP",
      hash: "SHA-256"
    },
    true,
    ["encrypt"]
  );
}

/**
 * Hash public key for session init
 */
export async function hashPublicKey(pubKeyDER: Uint8Array): Promise<Uint8Array> {
  const hash = await crypto.subtle.digest("SHA-256", pubKeyDER);
  // Return first 20 bytes
  return new Uint8Array(hash).slice(0, 20);
}

/**
 * Encrypt data with RSA-OAEP
 */
export async function rsaEncrypt(data: Uint8Array, publicKey: any): Promise<Uint8Array> {
  const encrypted = await crypto.subtle.encrypt(
    {
      name: "RSA-OAEP"
    },
    publicKey,
    data
  );
  return new Uint8Array(encrypted);
}

/**
 * Decrypt data with RSA-OAEP
 */
export async function rsaDecrypt(data: Uint8Array, privateKey: any): Promise<Uint8Array> {
  const decrypted = await crypto.subtle.decrypt(
    {
      name: "RSA-OAEP"
    },
    privateKey,
    data
  );
  return new Uint8Array(decrypted);
}

/**
 * Sign data with RSA-PSS
 */
export async function rsaSign(data: Uint8Array, privateKey: any): Promise<Uint8Array> {
  // Need to reimport key for signing
  const exported = await crypto.subtle.exportKey("pkcs8", privateKey);
  const signingKey = await crypto.subtle.importKey(
    "pkcs8",
    exported,
    {
      name: "RSA-PSS",
      hash: "SHA-256"
    },
    true,
    ["sign"]
  );

  const signature = await crypto.subtle.sign(
    {
      name: "RSA-PSS",
      saltLength: 32
    },
    signingKey,
    data
  );
  return new Uint8Array(signature);
}

/**
 * Verify signature with RSA-PSS
 */
export async function rsaVerify(
  data: Uint8Array, 
  signature: Uint8Array, 
  publicKey: any
): Promise<boolean> {
  // Need to reimport key for verification
  const exported = await crypto.subtle.exportKey("spki", publicKey);
  const verifyKey = await crypto.subtle.importKey(
    "spki",
    exported,
    {
      name: "RSA-PSS",
      hash: "SHA-256"
    },
    true,
    ["verify"]
  );

  return await crypto.subtle.verify(
    {
      name: "RSA-PSS",
      saltLength: 32
    },
    verifyKey,
    signature,
    data
  );
}

/**
 * Base32 encode for DNS compatibility
 */
export function base32Encode(data: Uint8Array): string {
  // Convert to Buffer for hi-base32
  const buffer = Buffer.from(data);
  return base32.encode(buffer).replace(/=/g, '');
}

/**
 * Base32 decode
 */
export function base32Decode(encoded: string): Uint8Array {
  // Add padding back
  const padded = encoded + '='.repeat((8 - encoded.length % 8) % 8);
  const decoded = base32.decode(padded);
  return new Uint8Array(Buffer.from(decoded, 'latin1'));
}

/**
 * Base64 encode
 */
export function base64Encode(data: Uint8Array): string {
  return btoa(String.fromCharCode.apply(null, Array.from(data)));
}

/**
 * Base64 decode
 */
export function base64Decode(encoded: string): Uint8Array {
  const binary = atob(encoded);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes;
}

/**
 * Combine signature and data for transmission
 */
export function packSignedData(signature: Uint8Array, data: Uint8Array): Uint8Array {
  const combined = new Uint8Array(signature.length + data.length);
  combined.set(signature, 0);
  combined.set(data, signature.length);
  return combined;
}

/**
 * Split signature and data after transmission
 */
export function unpackSignedData(combined: Uint8Array): { signature: Uint8Array, data: Uint8Array } {
  // RSA-2048 signature is 256 bytes
  const signature = combined.slice(0, 256);
  const data = combined.slice(256);
  return { signature, data };
}