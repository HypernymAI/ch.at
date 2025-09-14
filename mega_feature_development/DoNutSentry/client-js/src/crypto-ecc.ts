/**
 * ECC Cryptographic utilities for DonutSentry v2
 * Uses X25519 for encryption and Ed25519 for signatures
 */

import * as base32 from 'hi-base32';
import { Buffer } from 'buffer';

// Polyfill for browser environment
declare const window: any;
if (typeof window !== 'undefined' && !window.Buffer) {
  (window as any).Buffer = Buffer;
}

export interface ECCKeyPair {
  encryptionKeys: {
    publicKey: Uint8Array;  // X25519 public key (32 bytes)
    privateKey: Uint8Array; // X25519 private key (32 bytes)
  };
  signingKeys: {
    publicKey: Uint8Array;  // Ed25519 public key (32 bytes)
    privateKey: Uint8Array; // Ed25519 private key (64 bytes)
  };
}

/**
 * Generate ECC keypairs (X25519 + Ed25519)
 * Note: In browser, we'll use @noble/curves library
 */
export async function generateECCKeyPair(): Promise<ECCKeyPair> {
  // Dynamic import to avoid bundling issues
  const { x25519 } = await import('@noble/curves/ed25519');
  const { ed25519 } = await import('@noble/curves/ed25519');
  
  // Generate X25519 keypair for encryption
  const encPriv = x25519.utils.randomSecretKey();
  const encPub = x25519.getPublicKey(encPriv);
  
  // Generate Ed25519 keypair for signing
  const sigPriv = ed25519.utils.randomSecretKey();
  const sigPub = ed25519.getPublicKey(sigPriv);
  
  return {
    encryptionKeys: {
      publicKey: encPub,
      privateKey: encPriv
    },
    signingKeys: {
      publicKey: sigPub,
      privateKey: sigPriv
    }
  };
}

/**
 * Encode both public keys for DNS transmission
 */
export function encodePublicKeys(encPub: Uint8Array, sigPub: Uint8Array): string {
  // Concatenate both 32-byte public keys (64 bytes total)
  const bundle = new Uint8Array(64);
  bundle.set(encPub, 0);
  bundle.set(sigPub, 32);
  
  // Base32 encode without padding
  return base32.encode(Buffer.from(bundle)).replace(/=/g, '');
}

/**
 * Decode both public keys from DNS response
 */
export function decodePublicKeys(encoded: string): { encPub: Uint8Array; sigPub: Uint8Array } {
  // Add padding back if needed
  const padded = encoded + '='.repeat((8 - encoded.length % 8) % 8);
  const decoded = base32.decode(padded);
  const bundle = new Uint8Array(Buffer.from(decoded, 'latin1'));
  
  if (bundle.length !== 64) {
    throw new Error(`Invalid public key bundle size: ${bundle.length}`);
  }
  
  return {
    encPub: bundle.slice(0, 32),
    sigPub: bundle.slice(32, 64)
  };
}

/**
 * Encrypt data using X25519 + ChaCha20-Poly1305
 */
export async function x25519Encrypt(
  plaintext: Uint8Array, 
  senderPriv: Uint8Array, 
  recipientPub: Uint8Array
): Promise<Uint8Array> {
  const { x25519 } = await import('@noble/curves/ed25519');
  const { chacha20poly1305 } = await import('@noble/ciphers/chacha');
  const { sha256 } = await import('@noble/hashes/sha2');
  
  // Generate ephemeral keypair
  const ephemeralPriv = x25519.utils.randomSecretKey();
  const ephemeralPub = x25519.getPublicKey(ephemeralPriv);
  
  // Compute shared secret
  const sharedSecret = x25519.getSharedSecret(ephemeralPriv, recipientPub);
  
  // Derive encryption key using HKDF-like construction
  const keyMaterial = new Uint8Array(96); // 32 + 32 + 32
  keyMaterial.set(sharedSecret, 0);
  keyMaterial.set(ephemeralPub, 32);
  keyMaterial.set(recipientPub, 64);
  const key = sha256(keyMaterial);
  
  // Generate nonce (12 bytes for ChaCha20-Poly1305)
  const nonce = crypto.getRandomValues(new Uint8Array(12));
  
  // Encrypt
  const cipher = chacha20poly1305(key, nonce);
  const ciphertext = cipher.encrypt(plaintext);
  
  // Return: ephemeral_pubkey[32] || nonce[12] || ciphertext[...]
  const result = new Uint8Array(32 + 12 + ciphertext.length);
  result.set(ephemeralPub, 0);
  result.set(nonce, 32);
  result.set(ciphertext, 44);
  
  return result;
}

/**
 * Decrypt data encrypted with x25519Encrypt
 */
export async function x25519Decrypt(
  ciphertext: Uint8Array, 
  recipientPriv: Uint8Array
): Promise<Uint8Array> {
  const { x25519 } = await import('@noble/curves/ed25519');
  const { chacha20poly1305 } = await import('@noble/ciphers/chacha');
  const { sha256 } = await import('@noble/hashes/sha2');
  
  if (ciphertext.length < 44) {
    throw new Error('Ciphertext too short');
  }
  
  // Extract components
  const ephemeralPub = ciphertext.slice(0, 32);
  const nonce = ciphertext.slice(32, 44);
  const encrypted = ciphertext.slice(44);
  
  // Compute shared secret
  const sharedSecret = x25519.getSharedSecret(recipientPriv, ephemeralPub);
  
  // Derive decryption key (same as encryption)
  const ourPub = x25519.getPublicKey(recipientPriv);
  const keyMaterial = new Uint8Array(96);
  keyMaterial.set(sharedSecret, 0);
  keyMaterial.set(ephemeralPub, 32);
  keyMaterial.set(ourPub, 64);
  const key = sha256(keyMaterial);
  
  // Decrypt
  const cipher = chacha20poly1305(key, nonce);
  return cipher.decrypt(encrypted);
}

/**
 * Sign data with Ed25519
 */
export async function ed25519Sign(data: Uint8Array, privKey: Uint8Array): Promise<Uint8Array> {
  const { ed25519 } = await import('@noble/curves/ed25519');
  return ed25519.sign(data, privKey);
}

/**
 * Verify Ed25519 signature
 */
export async function ed25519Verify(
  data: Uint8Array, 
  signature: Uint8Array, 
  pubKey: Uint8Array
): Promise<boolean> {
  const { ed25519 } = await import('@noble/curves/ed25519');
  return ed25519.verify(signature, data, pubKey);
}

/**
 * Pack signature and encrypted data
 */
export function packSignedEncrypted(signature: Uint8Array, encrypted: Uint8Array): Uint8Array {
  // signature[64] || encrypted[...]
  const result = new Uint8Array(64 + encrypted.length);
  result.set(signature, 0);
  result.set(encrypted, 64);
  return result;
}

/**
 * Unpack signature and encrypted data
 */
export function unpackSignedEncrypted(data: Uint8Array): { 
  signature: Uint8Array; 
  encrypted: Uint8Array;
} {
  if (data.length < 64) {
    throw new Error('Data too short for signature');
  }
  return {
    signature: data.slice(0, 64),
    encrypted: data.slice(64)
  };
}

/**
 * Base32 encode for DNS compatibility
 */
export function base32Encode(data: Uint8Array): string {
  return base32.encode(Buffer.from(data)).replace(/=/g, '');
}

/**
 * Base32 decode
 */
export function base32Decode(encoded: string): Uint8Array {
  const padded = encoded + '='.repeat((8 - encoded.length % 8) % 8);
  const decoded = base32.decode(padded);
  return new Uint8Array(Buffer.from(decoded, 'latin1'));
}

/**
 * Base64 encode
 */
export function base64Encode(data: Uint8Array): string {
  return Buffer.from(data).toString('base64');
}

/**
 * Base64 decode
 */
export function base64Decode(encoded: string): Uint8Array {
  return new Uint8Array(Buffer.from(encoded, 'base64'));
}