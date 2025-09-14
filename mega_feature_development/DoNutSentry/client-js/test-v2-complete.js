const { DoNutSentryClient } = require('./dist/index.js');

// Hook to capture timing metrics
function instrumentClient(client) {
    const metrics = {
        sessionInit: 0,
        queryPages: [],
        execute: 0,
        responsePages: [],
        retries: []
    };
    
    const resolver = client.resolver;
    const originalResolveTxt = resolver.resolveTxt.bind(resolver);
    
    resolver.resolveTxt = async function(domain) {
        const startTime = Date.now();
        let attempt = 0;
        
        const tryResolve = async () => {
            try {
                const result = await originalResolveTxt(domain);
                const duration = Date.now() - startTime;
                
                // Log metrics
                if (domain.includes('.init.')) {
                    metrics.sessionInit = duration;
                    console.log(`[TIMING] Session init: ${duration}ms`);
                } else if (domain.includes('.exec.')) {
                    metrics.execute = duration;
                    console.log(`[TIMING] Execute: ${duration}ms`);
                } else if (domain.includes('.page.')) {
                    const pageMatch = domain.match(/\.page\.(\d+)\./);
                    if (pageMatch) {
                        metrics.responsePages.push({ page: parseInt(pageMatch[1]), duration });
                        console.log(`[TIMING] Response page ${pageMatch[1]}: ${duration}ms`);
                    }
                } else if (domain.match(/\.[A-Z0-9]{2}\./)) {
                    metrics.queryPages.push(duration);
                    console.log(`[TIMING] Query page ${metrics.queryPages.length}: ${duration}ms`);
                }
                
                if (attempt > 0) {
                    metrics.retries.push({ domain: domain.substring(0, 50), attempts: attempt + 1, duration });
                }
                
                return result;
            } catch (error) {
                attempt++;
                throw error;
            }
        };
        
        return tryResolve();
    };
    
    return metrics;
}

async function testV2WithDetails() {
    console.log('=== DonutSentry v2 Protocol - Complete Flow with Timing Metrics ===\n');
    
    const client = new DoNutSentryClient({
        dnsServers: ['127.0.0.1:8053']
    });
    const metrics = instrumentClient(client);

    // Test 1: Simple test that shows all conversions
    console.log('TEST 1: SIMPLE QUERY\n' + '='.repeat(50));
    const query = 'what is DNS';
    console.log('ORIGINAL QUERY:', query);
    console.log('Query length:', query.length, 'bytes');
    console.log('Hex:', Buffer.from(query).toString('hex'));
    console.log('Base64:', Buffer.from(query).toString('base64'));
    console.log('Base64 (no padding):', Buffer.from(query).toString('base64').replace(/=/g, ''));
    
    console.log('\n>>> Executing v2 query...\n');
    
    const startTime1 = Date.now();
    try {
        const result = await client.query(query, { version: 'v2' });
        const totalTime1 = Date.now() - startTime1;
        
        console.log('\nRESULTS:');
        console.log('Protocol:', result.metadata.version);
        console.log('Session ID:', result.metadata.sessionId);
        console.log('Query pages used:', result.metadata.totalQueryPages || 1);
        console.log('Response pages:', result.metadata.totalResponsePages || 1);
        console.log('Total time:', result.metadata.duration, 'ms');
        console.log('Response preview:', result.response.substring(0, 100) + '...');
        
        console.log('\nTIMING METRICS:');
        console.log(`Session init: ${metrics.sessionInit}ms`);
        console.log(`Query pages: ${metrics.queryPages.length} sent, avg ${Math.round(metrics.queryPages.reduce((a,b) => a+b, 0) / metrics.queryPages.length || 0)}ms`);
        console.log(`Execute: ${metrics.execute}ms`);
        console.log(`Response pages: ${metrics.responsePages.length} fetched, avg ${Math.round(metrics.responsePages.map(p => p.duration).reduce((a,b) => a+b, 0) / metrics.responsePages.length || 0)}ms`);
        console.log(`Retries: ${metrics.retries.length}`);
        
    } catch (error) {
        console.error('Error:', error.message);
    }
    
    // Test 1.5: Find timeout threshold
    console.log('\n\nTEST 1.5: FINDING TIMEOUT THRESHOLD\n' + '='.repeat(50));
    let timeoutChunks = 0;
    for (let chunks = 10; chunks <= 25; chunks += 5) {
        const testQuery = 'x'.repeat(chunks * 37);
        console.log(`\nTesting ${chunks} chunks...`);
        const start = Date.now();
        try {
            await client.query(testQuery, { version: 'v2' });
            console.log(`✓ Success in ${Date.now() - start}ms`);
        } catch (error) {
            console.log(`✗ Failed after ${Date.now() - start}ms: ${error.message}`);
            timeoutChunks = chunks;
            break;
        }
    }
    
    // Test 2: Long query that requires input paging
    console.log('\n\nTEST 2: LONG QUERY REQUIRING INPUT PAGING\n' + '='.repeat(50));
    const longQuery = 'Please provide an extremely detailed and comprehensive analysis of quantum computing including: ' +
                      'The fundamental quantum mechanical principles like superposition and entanglement, ' +
                      'how qubits function differently from classical bits, the concept of quantum gates and circuits, ' +
                      'major quantum algorithms including Shor\'s algorithm for factoring and Grover\'s search algorithm, ' +
                      'the challenges of quantum decoherence and error correction, current approaches to building quantum computers ' +
                      'including superconducting qubits and trapped ions, potential applications in cryptography and drug discovery, ' +
                      'and the current state of quantum supremacy claims and what they really mean for practical computing.';
    
    console.log('ORIGINAL QUERY:', longQuery);
    console.log('Query length:', longQuery.length, 'bytes');
    const base64Long = Buffer.from(longQuery).toString('base64').replace(/=/g, '');
    console.log('Base64 encoded length:', base64Long.length, 'chars');
    console.log('DNS label limit: 63 chars per label');
    console.log('Usable chars per label: ~50 (after session ID and metadata)');
    console.log('Estimated query pages needed:', Math.ceil(base64Long.length / 50));
    
    // Show first few chunks
    console.log('\nQUERY CHUNKING:');
    for (let i = 0; i < Math.min(3, Math.ceil(base64Long.length / 50)); i++) {
        const start = i * 50;
        const end = Math.min((i + 1) * 50, base64Long.length);
        const chunk = base64Long.substring(start, end);
        console.log(`Chunk ${i}: "${chunk}" (${chunk.length} chars)`);
    }
    console.log('... plus', Math.ceil(base64Long.length / 50) - 3, 'more chunks\n');
    
    console.log('>>> Executing long v2 query...\n');
    
    // Reset metrics for test 2
    const metrics2 = {
        sessionInit: metrics.sessionInit,
        queryPages: [],
        execute: 0,
        responsePages: [],
        retries: []
    };
    Object.assign(metrics, metrics2);
    
    const startTime2 = Date.now();
    try {
        const result2 = await client.query(longQuery, { version: 'v2' });
        const totalTime2 = Date.now() - startTime2;
        
        console.log('\nRESULTS:');
        console.log('Success:', result2.metadata.success);
        console.log('Error:', result2.metadata.error);
        console.log('Protocol:', result2.metadata.version);
        console.log('Session ID:', result2.metadata.sessionId);
        console.log('Query pages used:', result2.metadata.totalQueryPages || 1);
        console.log('Response pages:', result2.metadata.totalResponsePages || 1);
        console.log('Total time:', result2.metadata.duration, 'ms');
        
        console.log('\nDETAILED TIMING BREAKDOWN:');
        console.log(`Session init: ${metrics.sessionInit}ms`);
        console.log(`Query pages (${metrics.queryPages.length} total):`);
        const queryPageAvg = Math.round(metrics.queryPages.reduce((a,b) => a+b, 0) / metrics.queryPages.length);
        console.log(`  Min: ${Math.min(...metrics.queryPages)}ms`);
        console.log(`  Max: ${Math.max(...metrics.queryPages)}ms`);
        console.log(`  Avg: ${queryPageAvg}ms`);
        console.log(`  Total: ${metrics.queryPages.reduce((a,b) => a+b, 0)}ms`);
        console.log(`Execute: ${metrics.execute}ms`);
        console.log(`Response pages (${metrics.responsePages.length} total): ${metrics.responsePages.map(p => p.duration).reduce((a,b) => a+b, 0)}ms`);
        console.log(`Retries needed: ${metrics.retries.length}`);
        if (metrics.retries.length > 0) {
            console.log('Retry details:', metrics.retries);
        }
        console.log(`\nTOTAL ACTUAL TIME: ${totalTime2}ms`);
        console.log(`Time per query page: ${queryPageAvg}ms`);
        
        console.log('\nRESPONSE PAGES:');
        // Show each page
        for (let i = 0; i < result2.metadata.totalResponsePages; i++) {
            const start = i * 400;
            const end = Math.min((i + 1) * 400, result2.response.length);
            const pageContent = result2.response.substring(start, end);
            console.log(`\nPage ${i + 1}/${result2.metadata.totalResponsePages} (${pageContent.length} chars):`);
            console.log('---');
            console.log(pageContent);
            console.log('---');
        }
        
        console.log('\nFINAL ASSEMBLED RESPONSE:');
        console.log('Total length:', result2.response.length, 'chars');
        console.log('Full text:', result2.response);
        
    } catch (error) {
        console.error('\nError:', error.message);
        console.error('Note: Long queries often timeout due to multiple DNS round trips');
    }
    
    // Test 3: Query for very long response
    console.log('\n\nTEST 3: QUERY FOR VERY LONG RESPONSE\n' + '='.repeat(50));
    const detailQuery = 'explain in exhaustive detail how large language models work';
    console.log('ORIGINAL QUERY:', detailQuery);
    console.log('Query length:', detailQuery.length, 'bytes');
    console.log('Expected: Large response requiring many pages\n');
    
    console.log('>>> Executing detail query...\n');
    
    try {
        const result3 = await client.query(detailQuery, { version: 'v2' });
        
        console.log('\nRESULTS:');
        console.log('Protocol:', result3.metadata.version);
        console.log('Session ID:', result3.metadata.sessionId);
        console.log('Query pages used:', result3.metadata.totalQueryPages || 1);
        console.log('Response pages:', result3.metadata.totalResponsePages || 1);
        console.log('Total time:', result3.metadata.duration, 'ms');
        
        console.log('\nRESPONSE PAGES:');
        // Show each page
        for (let i = 0; i < result3.metadata.totalResponsePages; i++) {
            const start = i * 400;
            const end = Math.min((i + 1) * 400, result3.response.length);
            const pageContent = result3.response.substring(start, end);
            console.log(`\nPage ${i + 1}/${result3.metadata.totalResponsePages} (${pageContent.length} chars):`);
            console.log('---');
            console.log(pageContent);
            console.log('---');
        }
        
        console.log('\nFINAL ASSEMBLED RESPONSE:');
        console.log('Total length:', result3.response.length, 'chars');
        console.log('Full text:', result3.response);
        
    } catch (error) {
        console.error('Error:', error.message);
    }
}

testV2WithDetails().catch(console.error);