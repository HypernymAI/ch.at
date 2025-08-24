const puppeteer = require('puppeteer');

async function testUI() {
  console.log('Starting automated UI tests...');
  const browser = await puppeteer.launch({ 
    headless: 'new',
    args: ['--no-sandbox', '--disable-setuid-sandbox']
  });
  
  try {
    const page = await browser.newPage();
    
    // Set viewport
    await page.setViewport({ width: 1280, height: 800 });
    
    // Navigate to the site
    console.log('Navigating to http://localhost:3002...');
    const response = await page.goto('http://localhost:3002', { 
      waitUntil: 'networkidle0',
      timeout: 30000 
    });
    
    if (!response.ok()) {
      throw new Error(`Page returned status ${response.status()}`);
    }
    
    // Check for console errors
    const errors = [];
    page.on('console', msg => {
      if (msg.type() === 'error') {
        errors.push(msg.text());
      }
    });
    
    // Wait a bit to catch any React errors
    await new Promise(resolve => setTimeout(resolve, 2000));
    
    // Check if main content loaded
    const mainContent = await page.$('main');
    if (!mainContent) {
      throw new Error('Main content area not found');
    }
    
    // Check for React error boundary
    const errorBoundary = await page.$('.error-boundary, #__next-error__');
    if (errorBoundary) {
      throw new Error('React error boundary triggered');
    }
    
    // Take screenshot of current state
    await page.screenshot({ 
      path: 'homepage-screenshot.png',
      fullPage: true 
    });
    
    // Report console errors
    if (errors.length > 0) {
      console.error('\n❌ Console errors found:');
      errors.forEach(err => console.error(`  - ${err}`));
    }
    
    // Test navigation links
    console.log('\nTesting navigation...');
    const navLinks = await page.$$('nav a');
    console.log(`Found ${navLinks.length} navigation links`);
    
    // Click first tutorial link
    const tutorialLink = await page.$('a[href="/tutorials/getting-started"]');
    if (tutorialLink) {
      console.log('Clicking tutorial link...');
      await tutorialLink.click();
      await page.waitForNavigation({ waitUntil: 'networkidle0' });
      await page.screenshot({ path: 'tutorial-screenshot.png' });
    }
    
    console.log('\n✅ Basic UI tests completed');
    console.log('Screenshots saved: homepage-screenshot.png, tutorial-screenshot.png');
    
  } catch (error) {
    console.error('\n❌ UI Test Failed:', error.message);
    
    // Take error screenshot
    const page = (await browser.pages())[0];
    if (page) {
      await page.screenshot({ path: 'error-screenshot.png' });
      console.log('Error screenshot saved: error-screenshot.png');
    }
    
    process.exit(1);
  } finally {
    await browser.close();
  }
}

// Run the tests
testUI();