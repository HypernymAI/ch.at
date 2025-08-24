const puppeteer = require('puppeteer');

async function debug404() {
  console.log('🔍 Debugging 404 errors...\n');
  const browser = await puppeteer.launch({ 
    headless: 'new',
    args: ['--no-sandbox', '--disable-setuid-sandbox']
  });
  
  try {
    const page = await browser.newPage();
    
    // Track all network requests
    const failedRequests = [];
    
    page.on('requestfailed', request => {
      failedRequests.push({
        url: request.url(),
        method: request.method(),
        errorText: request.failure().errorText
      });
    });
    
    page.on('response', response => {
      if (response.status() >= 400) {
        console.log(`❌ ${response.status()} - ${response.url()}`);
      }
    });
    
    console.log('Loading homepage...');
    await page.goto('http://localhost:3002/', { 
      waitUntil: 'networkidle0',
      timeout: 30000 
    });
    
    console.log('\n🔴 Failed requests:');
    failedRequests.forEach(req => {
      console.log(`  ${req.method} ${req.url}`);
      console.log(`  Error: ${req.errorText}\n`);
    });
    
    // Check what resources the page is trying to load
    const resources = await page.evaluate(() => {
      const results = {
        scripts: Array.from(document.querySelectorAll('script[src]')).map(s => s.src),
        links: Array.from(document.querySelectorAll('link[href]')).map(l => ({ href: l.href, rel: l.rel })),
        images: Array.from(document.querySelectorAll('img[src]')).map(i => i.src)
      };
      return results;
    });
    
    console.log('\n📦 Resources on page:');
    console.log('Scripts:', resources.scripts);
    console.log('Links:', resources.links);
    console.log('Images:', resources.images);
    
  } catch (error) {
    console.error('Debug failed:', error.message);
  } finally {
    await browser.close();
  }
}

debug404();