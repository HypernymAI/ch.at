/**
 * NaCl/TweetNaCl Cryptographic utilities for DonutSentry v2
 * Uses X25519 for key exchange and Ed25519 for signatures
 */

import * as nacl from 'tweetnacl';
import * as naclUtil from 'tweetnacl-util';
import * as base32 from 'hi-base32';
import { createHash } from 'crypto';

export interface NaClKeyPair {
  encryptionKeys: nacl.BoxKeyPair;
  signingKeys: nacl.SignKeyPair;
}

/**
 * Generate NaCl keypairs (X25519 + Ed25519)
 */
export function generateNaClKeyPair(): NaClKeyPair {
  return {
    encryptionKeys: nacl.box.keyPair(),
    signingKeys: nacl.sign.keyPair()
  };
}

/**
 * Encode both public keys for DNS transmission (split into two labels)
 */
export function encodePublicKeys(encPub: Uint8Array, sigPub: Uint8Array): string {
  // Encode each 32-byte key separately
  const encPubEncoded = base32.encode(Buffer.from(encPub)).replace(/=/g, '');
  const sigPubEncoded = base32.encode(Buffer.from(sigPub)).replace(/=/g, '');
  
  // Return as two labels joined by dot
  return `${encPubEncoded}.${sigPubEncoded}`;
}

/**
 * Decode both public keys from server response (single base32 string)
 */
export function decodePublicKeys(encoded: string): { encPub: Uint8Array; sigPub: Uint8Array } {
  // Server sends both keys as single base32 string
  const bundle = base32Decode(encoded);
  
  if (bundle.length !== 64) {
    throw new Error(`Invalid public key bundle size: ${bundle.length}`);
  }
  
  return {
    encPub: bundle.slice(0, 32),
    sigPub: bundle.slice(32, 64)
  };
}

/**
 * Encrypt data using NaCl box (X25519 + XSalsa20-Poly1305)
 */
export function naclEncrypt(
  plaintext: Uint8Array, 
  senderPriv: Uint8Array, 
  recipientPub: Uint8Array
): Uint8Array {
  // Generate nonce (24 bytes for NaCl)
  const nonce = nacl.randomBytes(nacl.box.nonceLength);
  
  // Encrypt using box
  const ciphertext = nacl.box(plaintext, nonce, recipientPub, senderPriv);
  
  // Return: nonce[24] || ciphertext[...]
  const result = new Uint8Array(nonce.length + ciphertext.length);
  result.set(nonce, 0);
  result.set(ciphertext, nonce.length);
  
  return result;
}

/**
 * Decrypt data encrypted with naclEncrypt
 */
export function naclDecrypt(
  data: Uint8Array, 
  recipientPriv: Uint8Array,
  senderPub: Uint8Array
): Uint8Array {
  if (data.length < nacl.box.nonceLength) {
    throw new Error('Data too short');
  }
  
  // Extract nonce and ciphertext
  const nonce = data.slice(0, nacl.box.nonceLength);
  const ciphertext = data.slice(nacl.box.nonceLength);
  
  // Decrypt
  const plaintext = nacl.box.open(ciphertext, nonce, senderPub, recipientPriv);
  if (!plaintext) {
    throw new Error('Decryption failed');
  }
  
  return plaintext;
}

/**
 * Sign data with Ed25519
 */
export function ed25519Sign(data: Uint8Array, privKey: Uint8Array): Uint8Array {
  return nacl.sign.detached(data, privKey);
}

/**
 * Verify Ed25519 signature
 */
export function ed25519Verify(
  data: Uint8Array, 
  signature: Uint8Array, 
  pubKey: Uint8Array
): boolean {
  return nacl.sign.detached.verify(data, signature, pubKey);
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
  // Use decode.asBytes() to get raw bytes instead of UTF-8 string
  const decoded = base32.decode.asBytes(padded);
  return new Uint8Array(decoded);
}

/**
 * Base64 encode
 */
export function base64Encode(data: Uint8Array): string {
  return naclUtil.encodeBase64(data);
}

/**
 * Base64 decode
 */
export function base64Decode(encoded: string): Uint8Array {
  return naclUtil.decodeBase64(encoded);
}

/**
 * Derive shared secret via X25519 ECDH
 */
export function deriveSharedSecret(privateKey: Uint8Array, publicKey: Uint8Array): Uint8Array {
  return nacl.box.before(publicKey, privateKey);
}

/**
 * Derive XOR key from shared secret and context
 */
export function deriveXORKey(sharedSecret: Uint8Array, context: string, length: number): Uint8Array {
  const key = new Uint8Array(length);
  
  for (let i = 0; i < length; i += 32) {
    // Create input for this block
    const input = Buffer.concat([
      Buffer.from(sharedSecret),
      Buffer.from(context),
      Buffer.from([i / 32])
    ]);
    
    // Hash it
    const hash = createHash('sha256').update(input).digest();
    
    // Copy to output
    const copyLen = Math.min(32, length - i);
    key.set(hash.slice(0, copyLen), i);
  }
  
  return key;
}

/**
 * XOR encrypt/decrypt - zero overhead!
 */
export function xorEncrypt(plaintext: Uint8Array, key: Uint8Array): Uint8Array {
  if (key.length < plaintext.length) {
    throw new Error('XOR key too short');
  }
  
  const result = new Uint8Array(plaintext.length);
  for (let i = 0; i < plaintext.length; i++) {
    result[i] = plaintext[i] ^ key[i];
  }
  return result;
}

/**
 * XOR decrypt - same as encrypt
 */
export function xorDecrypt(ciphertext: Uint8Array, key: Uint8Array): Uint8Array {
  return xorEncrypt(ciphertext, key);
}