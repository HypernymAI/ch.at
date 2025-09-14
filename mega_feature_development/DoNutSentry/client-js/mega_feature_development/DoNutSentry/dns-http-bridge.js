const http = require('http');
const dns = require('dns').promises;
const { Resolver } = require('dns').promises;

// CORS headers for local development
const CORS_HEADERS = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Methods': 'GET, POST, OPTIONS',
  'Access-Control-Allow-Headers': 'Content-Type',
  'Content-Type': 'application/json'
};

const server = http.createServer(async (req, res) => {
  // Handle CORS preflight
  if (req.method === 'OPTIONS') {
    res.writeHead(200, CORS_HEADERS);
    res.end();
    return;
  }

  // Parse URL
  const url = new URL(req.url, `http://${req.headers.host}`);
  
  if (url.pathname === '/dns/query' && req.method === 'GET') {
    const domain = url.searchParams.get('domain');
    const type = url.searchParams.get('type') || 'TXT';
    
    if (!domain) {
      res.writeHead(400, CORS_HEADERS);
      res.end(JSON.stringify({ error: 'Missing domain parameter' }));
      return;
    }

    console.log(`[Bridge] DNS query: ${domain} (${type})`);
    
    try {
      // Create a new resolver for each request to avoid caching
      const resolver = new Resolver();
      resolver.setServers(['127.0.0.1:8053']);
      
      const startTime = Date.now();
      let records;
      
      if (type === 'TXT') {
        records = await resolver.resolveTxt(domain);
        // Flatten TXT record arrays
        records = records.map(r => Array.isArray(r) ? r.join('') : r);
      } else {
        records = await resolver.resolve(domain, type);
      }
      
      const duration = Date.now() - startTime;
      
      res.writeHead(200, CORS_HEADERS);
      res.end(JSON.stringify({
        domain,
        type,
        records,
        duration,
        timestamp: new Date().toISOString()
      }));
      
    } catch (error) {
      console.error(`[Bridge] DNS error for ${domain}:`, error.message);
      res.writeHead(500, CORS_HEADERS);
      res.end(JSON.stringify({ 
        error: error.message,
        domain,
        type 
      }));
    }
  } else {
    res.writeHead(404, CORS_HEADERS);
    res.end(JSON.stringify({ error: 'Not found' }));
  }
});

const PORT = 8081;
server.listen(PORT, () => {
  console.log(`DNS-HTTP Bridge running on http://localhost:${PORT}`);
  console.log(`Proxying DNS queries to 127.0.0.1:8053`);
  console.log(`\nExample: http://localhost:${PORT}/dns/query?domain=test.q.ch.at&type=TXT`);
});