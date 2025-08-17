/**
 * Session management for DoNutSentry multi-part queries
 */

import * as crypto from 'crypto';
import * as base32 from 'hi-base32';
import { promises as dnsPromises } from 'dns';

export interface SessionOptions {
  domain: string;
  resolver: dnsPromises.Resolver;
  timeout?: number;
}

export class SessionManager {
  private domain: string;
  private resolver: dnsPromises.Resolver;
  private timeout: number;
  private publicKey: Buffer;
  private privateKey: Buffer;

  constructor(options: SessionOptions) {
    this.domain = options.domain;
    this.resolver = options.resolver;
    this.timeout = options.timeout || 5000;
    
    // Generate ephemeral keypair for this session
    const { publicKey, privateKey } = crypto.generateKeyPairSync('rsa', {
      modulusLength: 2048,
      publicKeyEncoding: { type: 'spki', format: 'der' },
      privateKeyEncoding: { type: 'pkcs8', format: 'der' }
    });
    
    this.publicKey = publicKey as Buffer;
    this.privateKey = privateKey as Buffer;
  }

  /**
   * Initialize a new session
   */
  async initSession(): Promise<string> {
    // Send public key to server
    const pubKeyHash = crypto.createHash('sha256').update(this.publicKey).digest();
    const encoded = base32.encode(pubKeyHash).toLowerCase().replace(/=/g, '');
    
    const domain = `${encoded}.init.${this.domain}`;
    
    try {
      const records = await this.resolver.resolveTxt(domain);
      const response = records.flat().join('');
      
      // Response should be encrypted session ID
      const encryptedSessionId = Buffer.from(response, 'base64');
      
      // Decrypt with our private key
      const sessionId = crypto.privateDecrypt(
        {
          key: this.privateKey,
          padding: crypto.constants.RSA_PKCS1_OAEP_PADDING
        },
        encryptedSessionId
      );
      
      return base32.encode(sessionId).toLowerCase().replace(/=/g, '');
    } catch (error) {
      throw new Error(`Failed to initialize session: ${error}`);
    }
  }

  /**
   * Send a chunk of data
   */
  async sendChunk(sessionId: string, chunkNum: number, data: string): Promise<void> {
    const chunkNumEncoded = base32.encode(Buffer.from([chunkNum])).toLowerCase().replace(/=/g, '');
    const dataEncoded = base32.encode(data).toLowerCase().replace(/=/g, '');
    
    const domain = `${sessionId}.${chunkNumEncoded}.${dataEncoded}.${this.domain}`;
    
    if (domain.length > 255) {
      throw new Error(`Chunk too large: ${domain.length} bytes (max 255)`);
    }
    
    try {
      const records = await this.resolver.resolveTxt(domain);
      const response = records.flat().join('');
      
      if (response !== 'ACK') {
        throw new Error(`Chunk ${chunkNum} not acknowledged: ${response}`);
      }
    } catch (error) {
      throw new Error(`Failed to send chunk ${chunkNum}: ${error}`);
    }
  }

  /**
   * Execute the query after all chunks sent
   */
  async execute(sessionId: string, totalChunks: number): Promise<string> {
    const totalEncoded = base32.encode(Buffer.from([totalChunks])).toLowerCase().replace(/=/g, '');
    const domain = `${sessionId}.${totalEncoded}.exec.${this.domain}`;
    
    try {
      const records = await this.resolver.resolveTxt(domain);
      return records.flat().join('');
    } catch (error) {
      throw new Error(`Failed to execute session: ${error}`);
    }
  }

  /**
   * Calculate how many chunks are needed for a query
   */
  calculateChunks(query: string, compressed: Buffer): { chunks: string[], totalChunks: number } {
    const sessionIdLength = 52; // 32 bytes base32 encoded
    const chunkNumLength = 4;  // 1-2 bytes base32 encoded  
    const dotsAndDomain = 3 + this.domain.length; // dots + domain
    
    const maxDataPerChunk = 255 - sessionIdLength - chunkNumLength - dotsAndDomain;
    
    // Base32 encode the compressed data
    const encoded = base32.encode(compressed).toLowerCase().replace(/=/g, '');
    
    // Split into chunks
    const chunks: string[] = [];
    for (let i = 0; i < encoded.length; i += maxDataPerChunk) {
      chunks.push(encoded.slice(i, i + maxDataPerChunk));
    }
    
    return { chunks, totalChunks: chunks.length };
  }
}