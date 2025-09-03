/**
 * Session management for DonutSentry v2 with encryption and paging
 */

import { CustomResolver } from './custom-resolver';
import { promises as dnsPromises } from 'dns';
import {
  KeyPair,
  generateKeyPair,
  exportPublicKey,
  importPublicKey,
  hashPublicKey,
  rsaEncrypt,
  rsaDecrypt,
  rsaSign,
  rsaVerify,
  base32Encode,
  base32Decode,
  base64Encode,
  base64Decode,
  packSignedData,
  unpackSignedData
} from './crypto';

export interface SessionV2Options {
  resolver: dnsPromises.Resolver | CustomResolver;
  timeout?: number;
}

export interface PagedResponse {
  content: string;
  currentPage: number;
  totalPages: number;
  hasMore: boolean;
}

export class SessionV2Manager {
  private resolver: dnsPromises.Resolver | CustomResolver;
  private timeout: number;
  private keys?: KeyPair;
  private serverPublicKey?: any; // CryptoKey
  private sessionId?: string;

  constructor(options: SessionV2Options) {
    this.resolver = options.resolver;
    this.timeout = options.timeout || 5000;
  }

  /**
   * Initialize v2 session with keypair exchange
   */
  async initSession(): Promise<string> {
    // Generate client keypair
    this.keys = await generateKeyPair();
    
    // Export and hash public key
    const pubKeyDER = await exportPublicKey(this.keys.publicKey);
    const pubKeyHash = await hashPublicKey(pubKeyDER);
    const encoded = base32Encode(pubKeyHash);
    
    // Request session initialization
    const domain = `${encoded}.init.qp.ch.at`;
    console.log(`[V2 Init] Requesting session: ${domain}`);
    
    try {
      const records = await this.resolver.resolveTxt(domain);
      const response = records.flat().join('');
      
      // Decode response: session_id + server_pubkey
      const responseData = base64Decode(response);
      
      // First 16 bytes are session ID
      const sessionIdBytes = responseData.slice(0, 16);
      this.sessionId = base32Encode(sessionIdBytes);
      
      // Rest is server public key DER
      const serverPubKeyDER = responseData.slice(16);
      this.serverPublicKey = await importPublicKey(serverPubKeyDER);
      
      console.log(`[V2 Init] Session established: ${this.sessionId}`);
      return this.sessionId;
      
    } catch (error) {
      throw new Error(`Failed to initialize v2 session: ${error}`);
    }
  }

  /**
   * Send an encrypted, signed query page
   */
  async sendQueryPage(pageNum: number, content: string): Promise<void> {
    if (!this.keys || !this.serverPublicKey || !this.sessionId) {
      throw new Error('Session not initialized');
    }

    const data = new TextEncoder().encode(content);
    
    // For MVP, just send plaintext (TODO: encrypt and sign)
    const encoded = base32Encode(data);
    const pageNumEncoded = base32Encode(new Uint8Array([pageNum]));
    
    const domain = `${this.sessionId}.${pageNumEncoded}.${encoded}.qp.ch.at`;
    
    if (domain.length > 255) {
      throw new Error(`Query page too large: ${domain.length} bytes`);
    }
    
    console.log(`[V2 QueryPage] Sending page ${pageNum}: ${domain.substring(0, 50)}...`);
    
    // Retry with subexponential backoff starting at 10ms
    let lastError;
    for (let attempt = 0; attempt < 3; attempt++) {
      try {
        const records = await this.resolver.resolveTxt(domain);
        const response = records.flat().join('');
        
        if (response !== 'ACK') {
          throw new Error(`Page ${pageNum} not acknowledged: ${response}`);
        }
        return; // Success
      } catch (error) {
        lastError = error;
        if (attempt < 2) {
          // Subexponential backoff: 10ms, 25ms, 40ms
          await new Promise(resolve => setTimeout(resolve, 10 + (attempt * 15)));
        }
      }
    }
    throw new Error(`Failed to send query page ${pageNum}: ${lastError}`);
  }

  /**
   * Execute query and get first response page
   */
  async execute(totalQueryPages: number): Promise<PagedResponse> {
    if (!this.sessionId) {
      throw new Error('Session not initialized');
    }

    const totalEncoded = base32Encode(new Uint8Array([totalQueryPages]));
    const domain = `${this.sessionId}.${totalEncoded}.exec.qp.ch.at`;
    
    console.log(`[V2 Exec] Executing with ${totalQueryPages} query pages`);
    
    try {
      const records = await this.resolver.resolveTxt(domain);
      const response = records.flat().join('');
      
      return this.parseResponsePage(response);
    } catch (error) {
      throw new Error(`Failed to execute session: ${error}`);
    }
  }

  /**
   * Get a specific response page
   */
  async getPage(pageNum: number): Promise<PagedResponse> {
    if (!this.sessionId) {
      throw new Error('Session not initialized');
    }

    const domain = `${this.sessionId}.page.${pageNum}.qp.ch.at`;
    
    console.log(`[V2 Page] Fetching page ${pageNum}`);
    
    try {
      const records = await this.resolver.resolveTxt(domain);
      const response = records.flat().join('');
      
      return this.parseResponsePage(response);
    } catch (error) {
      throw new Error(`Failed to get page ${pageNum}: ${error}`);
    }
  }

  /**
   * Parse response page with metadata
   */
  private parseResponsePage(encodedResponse: string): PagedResponse {
    // For MVP, just decode base64 (TODO: decrypt and verify signature)
    const pageData = base64Decode(encodedResponse);
    const pageContent = new TextDecoder().decode(pageData);
    
    // Parse metadata: [Page N/M]content
    const match = pageContent.match(/^\[Page (\d+)\/(\d+)\]/);
    if (!match) {
      throw new Error('Invalid page format');
    }
    
    const currentPage = parseInt(match[1]);
    const totalPages = parseInt(match[2]);
    const content = pageContent.slice(match[0].length);
    
    return {
      content,
      currentPage,
      totalPages,
      hasMore: currentPage < totalPages
    };
  }

  /**
   * Calculate how to split query into pages
   */
  calculateQueryPages(query: string): string[] {
    // Each DNS label max 63 chars
    // After base32 encoding: ~1.6x expansion
    // Leave room for session ID (26 chars) + page num (2 chars) + dots
    const maxContentPerPage = Math.floor((63 - 2) / 1.6); // ~38 chars
    
    const pages: string[] = [];
    for (let i = 0; i < query.length; i += maxContentPerPage) {
      pages.push(query.slice(i, i + maxContentPerPage));
    }
    
    return pages;
  }
}