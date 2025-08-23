/**
 * Type definitions for DoNutSentry client
 */

export type EncodingStrategy = 'simple' | 'base32' | 'session';

export interface DoNutSentryOptions {
  /**
   * Domain to query (default: 'q.ch.at')
   */
  domain?: string;

  /**
   * DNS servers to use (default: system DNS)
   */
  dnsServers?: string[];

  /**
   * Query timeout in milliseconds (default: 5000)
   */
  timeout?: number;

  /**
   * Number of retries for failed queries (default: 3)
   */
  retries?: number;

  /**
   * Default encoding strategy (default: auto-select)
   */
  defaultEncoding?: EncodingStrategy;
}

export interface QueryOptions {
  /**
   * Force specific encoding strategy
   */
  encoding?: EncodingStrategy;

  /**
   * Add cache-busting suffix
   */
  cacheBust?: boolean;

  /**
   * Custom timeout for this query
   */
  timeout?: number;
}

export interface QueryResult {
  /**
   * Original query text
   */
  query: string;

  /**
   * Response from ch.at
   */
  response: string;

  /**
   * Query metadata
   */
  metadata: {
    encoding: EncodingStrategy;
    duration: number;
    success: boolean;
    error?: string;
    domain?: string;
    domainLength?: number;
    compressionRatio?: number;
    mode?: 'simple' | 'session';
    sessionId?: string;
    totalChunks?: number;
  };
}

export interface EncodingStats {
  encoded: string;
  length: number;
  valid: boolean;
  ratio?: number;
}