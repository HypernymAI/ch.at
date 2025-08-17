/**
 * Basic usage example for DoNutSentry client
 */

import { DoNutSentryClient } from '../src';

async function main() {
  // Create client with default settings
  const client = new DoNutSentryClient();

  console.log('DoNutSentry Client Example\n');

  // Example 1: Simple query
  console.log('1. Simple query:');
  const result1 = await client.query('what is dns');
  console.log(`Query: ${result1.query}`);
  console.log(`Response: ${result1.response}`);
  console.log(`Encoding: ${result1.metadata.encoding}`);
  console.log(`Domain: ${result1.metadata.domain}`);
  console.log('');

  // Example 2: Query with special characters
  console.log('2. Query with special characters:');
  const result2 = await client.query('What is AI? Explain briefly.');
  console.log(`Query: ${result2.query}`);
  console.log(`Response: ${result2.response}`);
  console.log(`Encoding: ${result2.metadata.encoding}`);
  console.log('');

  // Example 3: Long query that needs compression
  console.log('3. Long query requiring compression:');
  const longQuery = 'Explain how large language models like GPT work, including the transformer architecture, attention mechanisms, and training process. What makes them different from traditional neural networks?';
  const result3 = await client.query(longQuery);
  console.log(`Query: ${result3.query.substring(0, 50)}...`);
  console.log(`Response: ${result3.response}`);
  console.log(`Encoding: ${result3.metadata.encoding}`);
  console.log(`Compression ratio: ${result3.metadata.compressionRatio?.toFixed(2) || 'N/A'}`);
  console.log('');

  // Example 4: Get encoding statistics
  console.log('4. Encoding statistics for different queries:');
  const queries = [
    'hello world',
    'What is the meaning of life?',
    'Explain quantum computing in detail with examples and use cases for cryptography and optimization problems'
  ];

  for (const query of queries) {
    const stats = await client.getEncodingStats(query);
    console.log(`\nQuery: "${query.substring(0, 30)}..."`);
    console.log(`  Simple: ${stats.simple.length} chars (${stats.simple.valid ? 'valid' : 'invalid'})`);
    console.log(`  Base32: ${stats.base32.length} chars (${stats.base32.valid ? 'valid' : 'invalid'})`);
    console.log(`  Compressed: ${stats.compressed.length} chars, ratio: ${stats.compressed.ratio.toFixed(2)} (${stats.compressed.valid ? 'valid' : 'invalid'})`);
  }

  // Example 5: Custom DNS servers
  console.log('\n5. Using custom DNS servers:');
  const customClient = new DoNutSentryClient({
    dnsServers: ['8.8.8.8', '8.8.4.4'],
    timeout: 3000,
    retries: 2
  });

  const result5 = await customClient.query('hello from custom DNS');
  console.log(`Response: ${result5.response}`);
  console.log(`Duration: ${result5.metadata.duration}ms`);

  // Example 6: Force specific encoding
  console.log('\n6. Force specific encoding:');
  const result6 = await client.query('test query', { encoding: 'base32' });
  console.log(`Forced encoding: ${result6.metadata.encoding}`);
  console.log(`Domain: ${result6.metadata.domain}`);
}

// Run the example
main().catch(console.error);