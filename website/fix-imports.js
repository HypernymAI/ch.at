const fs = require('fs');
const path = require('path');

// Files that have broken imports
const filesToFix = [
  'pages/tutorials/getting-started.mdx',
  'pages/tutorials/first-query.mdx',
  'pages/tutorials/terminal-usage.mdx',
  'pages/tutorials/dns-queries.mdx',
  'pages/tutorials/api-basics.mdx'
];

function fixImports(filePath) {
  const fullPath = path.join(process.cwd(), filePath);
  
  if (!fs.existsSync(fullPath)) {
    console.log(`⚠️  File not found: ${filePath}`);
    return;
  }
  
  let content = fs.readFileSync(fullPath, 'utf8');
  
  // Check if it has Tabs import
  if (content.includes('import') && content.includes('Tabs')) {
    // Replace the Tabs/Tab implementation with a simple version
    content = content.replace(
      /<Tabs items=\{([^\}]+)\}>([\s\S]*?)<\/Tabs>/g,
      (match, items, tabContent) => {
        // Extract tab contents
        const tabs = tabContent.match(/<Tab>([\s\S]*?)<\/Tab>/g) || [];
        
        // Just show all tab contents sequentially for now
        return tabs.map(tab => {
          const content = tab.replace(/<\/?Tab>/g, '');
          return content;
        }).join('\n\n---\n\n');
      }
    );
    
    // Remove the import line if no other components are used
    if (!content.includes('<Callout') && !content.includes('<Steps')) {
      content = content.replace(/import \{[^}]+\} from 'nextra\/components'\n+/g, '');
    } else {
      // Remove Tabs and Tab from import
      content = content.replace(
        /import \{ ([^}]+) \} from 'nextra\/components'/g,
        (match, imports) => {
          const importList = imports.split(',').map(i => i.trim());
          const filteredImports = importList.filter(i => !['Tabs', 'Tab'].includes(i));
          if (filteredImports.length === 0) {
            return '';
          }
          return `import { ${filteredImports.join(', ')} } from 'nextra/components'`;
        }
      );
    }
    
    fs.writeFileSync(fullPath, content);
    console.log(`✅ Fixed: ${filePath}`);
  } else {
    console.log(`ℹ️  No Tabs found in: ${filePath}`);
  }
}

console.log('🔧 Fixing broken imports...\n');

filesToFix.forEach(fixImports);

console.log('\n✨ Done!');