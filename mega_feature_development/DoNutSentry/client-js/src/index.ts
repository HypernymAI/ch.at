/**
 * DoNutSentry Client - Query ch.at through DNS
 */

import { Resolver, promises as dnsPromises } from 'dns';
import { promisify } from 'util';
import * as base32 from 'hi-base32';
import { compress } from './compression';
import { SessionManager } from './session';
import { 
  DoNutSentryOptions, 
  QueryOptions, 
  QueryResult,
  EncodingStrategy 
} from './types';

export class DoNutSentryClient {
  private domain: string;
  private resolver: dnsPromises.Resolver;
  private options: DoNutSentryOptions;

  constructor(options: DoNutSentryOptions = {}) {
    this.domain = options.domain || 'q.ch.at';
    this.options = {
      timeout: 5000,
      retries: 3,
      ...options
    };
    
    this.resolver = new dnsPromises.Resolver();
    if (options.dnsServers) {
      this.resolver.setServers(options.dnsServers);
    }
  }

  /**
   * Query ch.at with automatic encoding selection
   */
  async query(text: string, options: QueryOptions = {}): Promise<QueryResult> {
    const startTime = Date.now();
    const encoding = options.encoding || this.selectEncodingStrategy(text);
    
    let encoded: string;
    let metadata: any = { encoding };

    try {
      // First, try to encode the query
      switch (encoding) {
        case 'simple':
          encoded = this.encodeSimple(text);
          break;
        case 'base32':
          encoded = this.encodeBase32(text);
          break;
        default:
          throw new Error(`Unknown encoding strategy: ${encoding}`);
      }

      const domain = `${encoded}.${this.domain}`;
      metadata.domain = domain;
      metadata.domainLength = domain.length;

      // Check if we need session mode (domain too long)
      if (domain.length > 255) {
        return await this.queryWithSession(text, metadata, startTime);
      }

      // Simple mode - single DNS query
      const response = await this.queryWithRetries(domain);
      
      return {
        query: text,
        response,
        metadata: {
          ...metadata,
          duration: Date.now() - startTime,
          success: true,
          mode: 'simple'
        }
      };

    } catch (error) {
      return {
        query: text,
        response: '',
        metadata: {
          ...metadata,
          duration: Date.now() - startTime,
          success: false,
          error: error instanceof Error ? error.message : 'Unknown error'
        }
      };
    }
  }

  /**
   * Select optimal encoding strategy based on query characteristics
   */
  private selectEncodingStrategy(text: string): EncodingStrategy {
    // Simple queries: alphanumeric + spaces only, under 50 chars
    if (text.length < 50 && /^[a-zA-Z0-9 ]+$/.test(text)) {
      return 'simple';
    }

    // Everything else uses base32
    return 'base32';
  }

  /**
   * Simple encoding: replace spaces with hyphens
   */
  private encodeSimple(text: string): string {
    return text
      .toLowerCase()
      .replace(/[^a-z0-9 ]/g, '')
      .replace(/\s+/g, '-')
      .replace(/^-+|-+$/g, ''); // Trim hyphens
  }

  /**
   * Base32 encode text for DNS compatibility
   */
  private encodeBase32(text: string): string {
    // hi-base32 handles UTF-8 automatically
    return base32.encode(text).toLowerCase().replace(/=/g, '');
  }


  /**
   * Query DNS with automatic retries
   */
  private async queryWithRetries(domain: string): Promise<string> {
    let lastError: Error | null = null;

    for (let i = 0; i < this.options.retries!; i++) {
      try {
        const records = await Promise.race([
          this.resolver.resolveTxt(domain),
          this.timeout(this.options.timeout!)
        ]) as string[][];

        // Concatenate all TXT record strings
        return records.flat().join('');

      } catch (error) {
        lastError = error as Error;
        
        // Don't retry on NXDOMAIN
        if (error && (error as any).code === 'ENOTFOUND') {
          break;
        }

        // Exponential backoff
        if (i < this.options.retries! - 1) {
          await this.sleep(Math.pow(2, i) * 100);
        }
      }
    }

    throw lastError || new Error('DNS query failed');
  }


  /**
   * Timeout helper
   */
  private timeout(ms: number): Promise<never> {
    return new Promise((_, reject) => {
      setTimeout(() => reject(new Error('DNS query timeout')), ms);
    });
  }

  /**
   * Sleep helper
   */
  private sleep(ms: number): Promise<void> {
    return new Promise(resolve => setTimeout(resolve, ms));
  }

  /**
   * Query using session mode for large queries
   */
  private async queryWithSession(text: string, metadata: any, startTime: number): Promise<QueryResult> {
    const session = new SessionManager({
      domain: this.domain,
      resolver: this.resolver,
      timeout: this.options.timeout
    });

    try {
      // Initialize session
      const sessionId = await session.initSession();
      metadata.sessionId = sessionId;
      metadata.mode = 'session';

      // Compress the query
      const compressed = await compress(Buffer.from(text, 'utf-8'));
      const { chunks, totalChunks } = session.calculateChunks(text, compressed);
      metadata.totalChunks = totalChunks;

      // Send chunks
      for (let i = 0; i < chunks.length; i++) {
        await session.sendChunk(sessionId, i, chunks[i]);
      }

      // Execute and get response
      const response = await session.execute(sessionId, totalChunks);

      return {
        query: text,
        response,
        metadata: {
          ...metadata,
          duration: Date.now() - startTime,
          success: true
        }
      };

    } catch (error) {
      return {
        query: text,
        response: '',
        metadata: {
          ...metadata,
          duration: Date.now() - startTime,
          success: false,
          error: error instanceof Error ? error.message : 'Unknown error',
          mode: 'session'
        }
      };
    }
  }

  /**
   * Get encoding statistics for a query
   */
  async getEncodingStats(text: string): Promise<{
    simple: { encoded: string, length: number, valid: boolean },
    base32: { encoded: string, length: number, valid: boolean }
  }> {
    const simple = this.encodeSimple(text);
    const base32 = this.encodeBase32(text);

    return {
      simple: {
        encoded: simple,
        length: simple.length,
        valid: simple.length <= 63
      },
      base32: {
        encoded: base32,
        length: base32.length,
        valid: base32.length <= 63
      }
    };
  }
}

// Export types and client
export * from './types';
export { compress, decompress } from './compression';
export { SessionManager } from './session';

// Default export
export default DoNutSentryClient;