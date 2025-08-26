const DoNutSentryClient = require('./dist/index.js').default;

async function testLocal() {
  console.log('Testing DoNutSentry against local ch.at...\n');

  // Configure for local testing
  const client = new DoNutSentryClient({
    domain: 'q.ch.at',
    dnsServers: ['127.0.0.1:5354']  // Local DNS on high port
  });

  // Test 1: Simple encoding
  console.log('Test 1: Simple encoding');
  try {
    const result = await client.query('what is dns');
    console.log('Query:', result.query);
    console.log('Domain:', result.metadata.domain);
    console.log('Encoding:', result.metadata.encoding);
    console.log('Success:', result.metadata.success);
    if (result.response) {
      console.log('Response:', result.response.substring(0, 100) + '...');
    }
  } catch (error) {
    console.log('Error:', error.message);
  }

  console.log('\n---\n');

  // Test 2: Base32 encoding
  console.log('Test 2: Base32 encoding');
  try {
    const result = await client.query('Hello, world!');
    console.log('Query:', result.query);
    console.log('Domain:', result.metadata.domain);
    console.log('Encoding:', result.metadata.encoding);
    console.log('Success:', result.metadata.success);
    if (result.response) {
      console.log('Response:', result.response.substring(0, 100) + '...');
    }
  } catch (error) {
    console.log('Error:', error.message);
  }

  console.log('\n---\n');

  // Test 3: Direct DNS test to verify server is working
  console.log('Test 3: Direct DNS test (non-DoNutSentry)');
  const dns = require('dns');
  const resolver = new dns.Resolver();
  resolver.setServers(['127.0.0.1:5354']);
  
  resolver.resolveTxt('test.ch.at', (err, records) => {
    if (err) {
      console.log('Direct DNS error:', err.message);
    } else {
      console.log('Direct DNS response:', records);
    }
  });
}

testLocal().catch(console.error);