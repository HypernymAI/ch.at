const DoNutSentryClient = require('./dist/index.js').default;

async function testDNS() {
  console.log('Testing DoNutSentry DNS client...\n');

  // Test against real ch.at
  const client = new DoNutSentryClient({
    domain: 'q.ch.at',
    dnsServers: ['8.8.8.8']  // Use Google DNS
  });

  // Test 1: Simple encoding
  console.log('Test 1: Simple encoding');
  try {
    const result = await client.query('what is dns');
    console.log('Query:', result.query);
    console.log('Domain:', result.metadata.domain);
    console.log('Encoding:', result.metadata.encoding);
    console.log('Mode:', result.metadata.mode);
    console.log('Success:', result.metadata.success);
    if (result.metadata.success) {
      console.log('Response preview:', result.response.substring(0, 50) + '...');
    } else {
      console.log('Error:', result.metadata.error);
    }
  } catch (error) {
    console.log('Error:', error.message);
  }

  console.log('\n---\n');

  // Test 2: Base32 encoding
  console.log('Test 2: Base32 encoding');
  try {
    const result = await client.query('What is AI?');
    console.log('Query:', result.query);
    console.log('Domain:', result.metadata.domain);
    console.log('Encoding:', result.metadata.encoding);
    console.log('Success:', result.metadata.success);
  } catch (error) {
    console.log('Error:', error.message);
  }

  console.log('\n---\n');

  // Test 3: Encoding stats
  console.log('Test 3: Encoding statistics');
  const testQuery = 'Explain quantum computing';
  const stats = await client.getEncodingStats(testQuery);
  console.log('Query:', testQuery);
  console.log('Simple:', stats.simple.length, 'chars,', stats.simple.valid ? 'valid' : 'invalid');
  console.log('Base32:', stats.base32.length, 'chars,', stats.base32.valid ? 'valid' : 'invalid');
}

testDNS().catch(console.error);