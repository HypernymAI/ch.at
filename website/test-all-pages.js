const puppeteer = require('puppeteer');
const fs = require('fs');

async function testAllPages() {
  console.log('🔍 Starting comprehensive UI tests...\n');
  const browser = await puppeteer.launch({ 
    headless: 'new',
    args: ['--no-sandbox', '--disable-setuid-sandbox']
  });
  
  const results = {
    passed: [],
    failed: [],
    errors: {}
  };

  try {
    const page = await browser.newPage();
    await page.setViewport({ width: 1280, height: 800 });
    
    // Collect console errors
    const pageErrors = [];
    page.on('console', msg => {
      if (msg.type() === 'error') {
        pageErrors.push(msg.text());
      }
    });
    
    page.on('pageerror', error => {
      pageErrors.push(error.toString());
    });

    // Pages to test
    const pagesToTest = [
      { url: '/', name: 'Homepage' },
      { url: '/tutorials/getting-started', name: 'Getting Started Tutorial' },
      { url: '/tutorials/first-query', name: 'First Query Tutorial' },
      { url: '/tutorials/web-interface', name: 'Web Interface Tutorial' },
      { url: '/tutorials/terminal-usage', name: 'Terminal Usage Tutorial' },
      { url: '/tutorials/ssh-chat', name: 'SSH Chat Tutorial' },
      { url: '/tutorials/dns-queries', name: 'DNS Queries Tutorial' },
      { url: '/tutorials/api-basics', name: 'API Basics Tutorial' },
      { url: '/guides/self-hosting', name: 'Self-hosting Guide' },
      { url: '/guides/docker-deployment', name: 'Docker Guide' },
      { url: '/guides/systemd-setup', name: 'Systemd Guide' },
      { url: '/guides/ssl-certificates', name: 'SSL Guide' },
      { url: '/guides/rate-limiting', name: 'Rate Limiting Guide' },
      { url: '/guides/custom-models', name: 'Custom Models Guide' },
      { url: '/guides/api-integration', name: 'API Integration Guide' },
      { url: '/guides/contributing', name: 'Contributing Guide' },
      { url: '/guides/donutsentry', name: 'DoNutSentry Guide' },
      { url: '/explanations/architecture', name: 'Architecture' },
      { url: '/explanations/protocols', name: 'Protocols' },
      { url: '/explanations/privacy-model', name: 'Privacy Model' },
      { url: '/explanations/design-philosophy', name: 'Design Philosophy' },
      { url: '/explanations/donutsentry-theory', name: 'DoNutSentry Theory' },
      { url: '/reference/api', name: 'API Reference' },
      { url: '/reference/cli-tools', name: 'CLI Tools' },
      { url: '/reference/configuration', name: 'Configuration' },
      { url: '/reference/environment-variables', name: 'Environment Variables' },
      { url: '/reference/error-codes', name: 'Error Codes' },
      { url: '/reference/ports', name: 'Ports Reference' }
    ];

    // Test each page
    for (const pageInfo of pagesToTest) {
      pageErrors.length = 0; // Clear errors for this page
      
      try {
        console.log(`Testing: ${pageInfo.name} (${pageInfo.url})`);
        
        const response = await page.goto(`http://localhost:3002${pageInfo.url}`, { 
          waitUntil: 'networkidle0',
          timeout: 10000 
        });
        
        if (!response.ok() && response.status() !== 304) {
          throw new Error(`HTTP ${response.status()}`);
        }
        
        // Wait a bit for any delayed errors
        await new Promise(resolve => setTimeout(resolve, 1000));
        
        // Check for React error boundary
        const hasError = await page.evaluate(() => {
          const errorElement = document.querySelector('.error-boundary, #__next-error__, [data-nextjs-error]');
          return errorElement !== null;
        });
        
        if (hasError) {
          throw new Error('React error boundary triggered');
        }
        
        // Check if main content exists
        const hasContent = await page.evaluate(() => {
          const main = document.querySelector('main');
          return main && main.textContent.trim().length > 0;
        });
        
        if (!hasContent) {
          throw new Error('No main content found');
        }
        
        if (pageErrors.length > 0) {
          throw new Error(`Console errors: ${pageErrors.join(', ')}`);
        }
        
        results.passed.push(pageInfo.name);
        console.log(`  ✅ Passed\n`);
        
      } catch (error) {
        results.failed.push(pageInfo.name);
        results.errors[pageInfo.name] = error.message;
        console.log(`  ❌ Failed: ${error.message}\n`);
        
        // Take screenshot of error
        await page.screenshot({ 
          path: `error-${pageInfo.url.replace(/\//g, '-')}.png`,
          fullPage: true 
        });
      }
    }
    
    // Summary
    console.log('\n📊 Test Summary:');
    console.log(`✅ Passed: ${results.passed.length}/${pagesToTest.length}`);
    console.log(`❌ Failed: ${results.failed.length}/${pagesToTest.length}`);
    
    if (results.failed.length > 0) {
      console.log('\n🔴 Failed pages:');
      results.failed.forEach(name => {
        console.log(`  - ${name}: ${results.errors[name]}`);
      });
    }
    
    // Save results to file
    fs.writeFileSync('test-results.json', JSON.stringify(results, null, 2));
    console.log('\n📁 Detailed results saved to test-results.json');
    
  } catch (error) {
    console.error('\n🚨 Test suite failed:', error.message);
    process.exit(1);
  } finally {
    await browser.close();
  }
}

// Run tests
testAllPages();