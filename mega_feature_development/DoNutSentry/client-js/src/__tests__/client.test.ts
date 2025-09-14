/**
 * Tests for DoNutSentry client
 */

import { DoNutSentryClient } from '../index';
import { promises as dnsPromises } from 'dns';

// Mock DNS resolver
jest.mock('dns', () => ({
  promises: {
    Resolver: jest.fn().mockImplementation(() => ({
      resolveTxt: jest.fn(),
      setServers: jest.fn()
    }))
  }
}));

describe('DoNutSentryClient', () => {
  let client: DoNutSentryClient;
  let mockResolver: any;

  beforeEach(() => {
    jest.clearAllMocks();
    client = new DoNutSentryClient();
    mockResolver = (dnsPromises.Resolver as any).mock.results[0].value;
  });

  describe('encoding strategies', () => {
    test('simple encoding for basic queries', async () => {
      const query = 'what is dns';
      mockResolver.resolveTxt.mockResolvedValue([['DNS stands for Domain Name System']]);

      const result = await client.query(query);

      expect(mockResolver.resolveTxt).toHaveBeenCalledWith('what-is-dns.q.ch.at');
      expect(result.metadata.encoding).toBe('simple');
      expect(result.response).toBe('DNS stands for Domain Name System');
    });

    test('base32 encoding for special characters', async () => {
      const query = 'What is AI?';
      mockResolver.resolveTxt.mockResolvedValue([['AI is Artificial Intelligence']]);

      const result = await client.query(query);

      expect(mockResolver.resolveTxt).toHaveBeenCalledWith(
        expect.stringMatching(/^[a-z0-9]+\.q\.ch\.at$/)
      );
      expect(result.metadata.encoding).toBe('base32');
    });

    test('uses session mode for very long queries', async () => {
      // Create a query that's too long for a single DNS label
      const query = 'a'.repeat(300);
      
      // For session mode test, we need to mock properly:
      // 1. The server would encrypt a session ID with our public key
      // 2. We decrypt it with our private key
      // Since we can't predict the RSA keypair, we'll just check the mode
      mockResolver.resolveTxt.mockImplementation(() => {
        throw new Error('Session init failed - server not implemented');
      });

      const result = await client.query(query);

      // Session mode will fail without a real server
      expect(result.metadata.mode).toBe('session');
      expect(result.metadata.success).toBe(false);
      expect(result.metadata.error).toContain('Session init failed');
    });
  });

  describe('DNS query handling', () => {
    test('concatenates multiple TXT record strings', async () => {
      mockResolver.resolveTxt.mockResolvedValue([
        ['Part 1 of the response. '],
        ['Part 2 of the response.']
      ]);

      const result = await client.query('test');

      expect(result.response).toBe('Part 1 of the response. Part 2 of the response.');
    });

    test('retries on timeout', async () => {
      mockResolver.resolveTxt
        .mockRejectedValueOnce(new Error('Timeout'))
        .mockResolvedValueOnce([['Success']]);

      const result = await client.query('test', { timeout: 100 });

      expect(mockResolver.resolveTxt).toHaveBeenCalledTimes(2);
      expect(result.response).toBe('Success');
      expect(result.metadata.success).toBe(true);
    });

    test('does not retry on NXDOMAIN', async () => {
      const error = new Error('Domain not found');
      (error as any).code = 'ENOTFOUND';
      mockResolver.resolveTxt.mockRejectedValue(error);

      const result = await client.query('test');

      expect(mockResolver.resolveTxt).toHaveBeenCalledTimes(1);
      expect(result.metadata.success).toBe(false);
      expect(result.metadata.error).toContain('Domain not found');
    });
  });

  describe('encoding statistics', () => {
    test('provides encoding stats for analysis', async () => {
      const query = 'What is machine learning?';
      const stats = await client.getEncodingStats(query);

      expect(stats.simple.length).toBeLessThan(63);
      expect(stats.simple.valid).toBe(true);
      expect(stats.base32.valid).toBe(true);
    });

    test('identifies invalid encodings', async () => {
      const longQuery = 'a'.repeat(200);
      const stats = await client.getEncodingStats(longQuery);

      expect(stats.simple.valid).toBe(false);
      expect(stats.base32.valid).toBe(false);
    });
  });

  describe('custom DNS servers', () => {
    test('uses custom DNS servers when provided', () => {
      const customClient = new DoNutSentryClient({
        dnsServers: ['8.8.8.8', '8.8.4.4']
      });

      const customResolver = (dnsPromises.Resolver as any).mock.results[1].value;
      expect(customResolver.setServers).toHaveBeenCalledWith(['8.8.8.8', '8.8.4.4']);
    });
  });

  describe('edge cases', () => {
    test('handles empty query', async () => {
      const result = await client.query('');
      expect(result.metadata.success).toBe(false);
    });

    test('handles very long domain names', async () => {
      const longQuery = 'a'.repeat(300);
      
      // Mock DNS to fail for session test
      mockResolver.resolveTxt.mockImplementation(() => {
        throw new Error('Session init failed - server not implemented');
      });
      
      const result = await client.query(longQuery);
      
      // Should use session mode for very long queries
      expect(result.metadata.mode).toBe('session');
      expect(result.metadata.success).toBe(false);
    });

    test('handles unicode in queries', async () => {
      const query = 'What is 你好?';
      mockResolver.resolveTxt.mockResolvedValue([['Hello in Chinese']]);

      const result = await client.query(query);
      
      expect(result.metadata.encoding).toBe('base32');
      expect(result.metadata.success).toBe(true);
    });
  });
});