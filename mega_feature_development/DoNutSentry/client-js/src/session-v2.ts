/**
 * Session management for DonutSentry v2 with encryption and paging
 */

import { CustomResolver } from './custom-resolver';
import { promises as dnsPromises } from 'dns';
import {
  NaClKeyPair,
  generateNaClKeyPair,
  encodePublicKeys,
  decodePublicKeys,
  naclEncrypt,
  naclDecrypt,
  ed25519Sign,
  ed25519Verify,
  base32Encode,
  base32Decode,
  base64Encode,
  base64Decode,
  packSignedEncrypted,
  unpackSignedEncrypted,
  deriveSharedSecret,
  deriveXORKey,
  xorEncrypt,
  xorDecrypt
} from './crypto-nacl';

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
  private keys?: NaClKeyPair;
  private serverEncPubKey?: Uint8Array;
  private serverSigPubKey?: Uint8Array;
  private sessionId?: string;
  private sharedSecret?: Uint8Array;

  constructor(options: SessionV2Options) {
    this.resolver = options.resolver;
    this.timeout = options.timeout || 5000;
  }

  /**
   * Initialize v2 session with keypair exchange
   */
  async initSession(): Promise<string> {
    // Generate client keypairs (X25519 + Ed25519)
    this.keys = generateNaClKeyPair();
    
    // Encode both public keys
    const encoded = encodePublicKeys(
      this.keys.encryptionKeys.publicKey,
      this.keys.signingKeys.publicKey
    );
    
    // Request session initialization
    const domain = `${encoded}.init.qp.ch.at`;
    console.log(`[V2 Init] Requesting session: ${domain}`);
    
    try {
      const records = await this.resolver.resolveTxt(domain);
      // DNS might split the response into multiple strings
      const response = records.flat().join('');
      
      console.log(`[V2 Init Debug] Response length: ${response.length}`);
      console.log(`[V2 Init Debug] Response start: ${response.substring(0, 20)}`);
      console.log(`[V2 Init Debug] First 3 chars: "${response.substring(0, 3)}"`);
      
      // Response format: length_prefix[3 digits] + encrypted_session_id[base64] + server_pubkeys[base32]
      if (response.length < 4) {
        throw new Error('Invalid init response format');
      }
      
      // Parse length prefix
      const encSessionLen = parseInt(response.substring(0, 3));
      if (isNaN(encSessionLen) || encSessionLen <= 0) {
        throw new Error(`Invalid length prefix: "${response.substring(0, 3)}""`);
      }
      
      // Extract parts using length
      const encryptedSessionIdB64 = response.substring(3, 3 + encSessionLen);
      const serverPubKeysB32 = response.substring(3 + encSessionLen);
      
      console.log(`[V2 Init Debug] Encrypted session ID B64: ${encryptedSessionIdB64.length} chars`);
      console.log(`[V2 Init Debug] Server keys B32: ${serverPubKeysB32.length} chars`);
      console.log(`[V2 Init Debug] Server keys B32 start: ${serverPubKeysB32.substring(0, 20)}`);
      
      const encryptedSessionId = base64Decode(encryptedSessionIdB64);
      const serverKeys = decodePublicKeys(serverPubKeysB32);
      
      // Decrypt session ID
      const sessionIdBytes = naclDecrypt(
        encryptedSessionId,
        this.keys.encryptionKeys.secretKey,
        serverKeys.encPub
      );
      this.sessionId = base32Encode(sessionIdBytes);
      
      // Store server public keys
      this.serverEncPubKey = serverKeys.encPub;
      this.serverSigPubKey = serverKeys.sigPub;
      
      // Derive shared secret for XOR encryption
      this.sharedSecret = deriveSharedSecret(
        this.keys.encryptionKeys.secretKey,
        this.serverEncPubKey
      );
      
      console.log(`[V2 Init] Session established: ${this.sessionId}`);
      console.log(`[V2 Init] Shared secret derived for XOR encryption`);
      return this.sessionId;
      
    } catch (error) {
      throw new Error(`Failed to initialize v2 session: ${error}`);
    }
  }

  /**
   * Send an encrypted query page (XOR encryption - zero overhead!)
   */
  async sendQueryPage(pageNum: number, content: string): Promise<void> {
    if (!this.keys || !this.sharedSecret || !this.sessionId) {
      throw new Error('Session not initialized');
    }

    const plaintext = new TextEncoder().encode(content);
    
    // Derive XOR key for this page
    const context = `query:page:${pageNum}`;
    const xorKey = deriveXORKey(this.sharedSecret, context, plaintext.length);
    
    // Encrypt with XOR - same size as plaintext!
    const encrypted = xorEncrypt(plaintext, xorKey);
    
    // Encode for DNS - just the encrypted data, no signature overhead!
    const encoded = base32Encode(encrypted);
    const pageNumEncoded = base32Encode(new Uint8Array([pageNum]));
    
    const domain = `${this.sessionId}.${pageNumEncoded}.${encoded}.qp.ch.at`;
    
    if (domain.length > 255) {
      throw new Error(`Query page too large: ${domain.length} bytes`);
    }
    
    console.log(`[V2 QueryPage] Sending encrypted page ${pageNum}: ${domain.substring(0, 50)}...`);
    
    // Retry with subexponential backoff starting at 10ms
    let lastError;
    for (let attempt = 0; attempt < 3; attempt++) {
      try {
        const records = await this.resolver.resolveTxt(domain);
        const response = records.flat().join('');
        
        if (response !== 'ACK') {
          console.log(`[V2 QueryPage] Got response: "${response}" (length: ${response.length})`);
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
   * Execute query and get first response page (with async processing support)
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
      
      // Check if response is "PROCESSING" (async mode)
      if (response === 'PROCESSING') {
        console.log('[V2 Exec] Query submitted for async processing, polling for results...');
        
        // Poll for status
        let pollCount = 0;
        const maxPolls = 60; // Max 60 seconds
        
        while (pollCount < maxPolls) {
          await new Promise(resolve => setTimeout(resolve, 1000)); // Wait 1 second
          pollCount++;
          
          const statusDomain = `${this.sessionId}.status.qp.ch.at`;
          console.log(`[V2 Status] Polling status (attempt ${pollCount}/${maxPolls})`);
          
          try {
            const statusRecords = await this.resolver.resolveTxt(statusDomain);
            const statusResponse = statusRecords.flat().join('');
            
            if (statusResponse === 'PROCESSING') {
              if (pollCount % 5 === 0) {
                console.log(`[V2 Status] Still processing... (${pollCount}s)`);
              }
              continue;
            }
            
            // Response is ready - parse it
            console.log(`[V2 Status] Response ready after ${pollCount}s`);
            return this.parseResponsePage(statusResponse);
            
          } catch (statusError) {
            console.error(`[V2 Status] Status check failed: ${statusError}`);
            // Continue polling on error
          }
        }
        
        throw new Error(`Query processing timed out after ${maxPolls} seconds`);
      }
      
      // Direct response (backward compatibility)
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
   * Parse response page with XOR decryption
   */
  private async parseResponsePage(encodedResponse: string): Promise<PagedResponse> {
    if (!this.sharedSecret) {
      throw new Error('Session not initialized');
    }
    
    // Decode base64
    const encrypted = base64Decode(encodedResponse);
    
    // First, decrypt to get page metadata
    // Try with page 0 context to read metadata
    const tempContext = `response:page:0`;
    const tempKey = deriveXORKey(this.sharedSecret, tempContext, Math.min(20, encrypted.length));
    const tempDecrypted = xorDecrypt(encrypted.slice(0, 20), tempKey);
    const tempText = new TextDecoder().decode(tempDecrypted);
    
    // Extract page number from metadata
    const metaMatch = tempText.match(/^\[Page (\d+)\/(\d+)\]/);
    if (!metaMatch) {
      throw new Error('Invalid page format');
    }
    
    const pageNum = parseInt(metaMatch[1]);
    const totalPagesTemp = parseInt(metaMatch[2]);
    
    // Now decrypt with correct page context
    const context = `response:page:${pageNum - 1}`; // 0-indexed on server
    const xorKey = deriveXORKey(this.sharedSecret, context, encrypted.length);
    const plaintext = xorDecrypt(encrypted, xorKey);
    const pageContent = new TextDecoder().decode(plaintext);
    
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
    // With XOR encryption - ZERO OVERHEAD!
    // Each DNS label max 63 chars
    // After base32 encoding: ~1.6x expansion
    // DNS domain: SESSION(26).PAGE(2).DATA(?).qp.ch.at(10)
    // Available for data label: 63 chars base32
    // After decoding: 63 / 1.6 = ~39 bytes
    // Since XOR has no overhead, we can send 39 chars per page!
    const maxContentPerPage = 39;
    
    const pages: string[] = [];
    for (let i = 0; i < query.length; i += maxContentPerPage) {
      pages.push(query.slice(i, i + maxContentPerPage));
    }
    
    return pages;
  }
}