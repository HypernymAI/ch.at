# ch.at Documentation Website Setup Guide

## Overview

This document provides a comprehensive guide to the Nextra-based documentation website we've created for ch.at. The site uses Nextra 3 with Next.js 14 to create a modern, searchable, GitHub-integrated documentation experience.

## Project Status

### Current State
- **Framework**: Nextra 3.3.1 (documentation theme)
- **React**: 18.3.1
- **Next.js**: 14.2.32
- **Status**: Initial setup complete, running on port 3003 (with errors to fix)
- **Structure**: Diátaxis framework implemented

### What We've Built
1. Basic Nextra documentation site with proper configuration
2. Diátaxis-based content structure (tutorials, how-to guides, explanations, reference)
3. GitHub integration with edit buttons
4. Dark mode support
5. Search functionality (built-in)
6. Mobile responsive design

## Technology Stack

### Core Dependencies
```json
{
  "dependencies": {
    "next": "^14.2.32",        // Next.js framework
    "nextra": "^3.3.1",         // Documentation framework
    "nextra-theme-docs": "^3.3.1", // Docs theme
    "react": "^18.3.1",         // React library
    "react-dom": "^18.3.1"      // React DOM
  },
  "devDependencies": {
    "@types/node": "24.3.0",    // Node.js TypeScript types
    "@types/react": "^19.1.11"  // React TypeScript types
  }
}
```

### Why Nextra?
- **Stripe-like documentation design** out of the box
- **MDX support** - React components in Markdown
- **Built-in search** using FlexSearch
- **GitHub integration** - Edit on GitHub buttons
- **Performance** - Static generation with Next.js
- **Developer friendly** - Hot reload, TypeScript support

## Directory Structure

```
website/
├── pages/                    # All documentation pages
│   ├── _app.jsx             # Next.js app wrapper (required by Nextra 3)
│   ├── _meta.js             # Root navigation structure
│   ├── index.mdx            # Homepage
│   ├── tutorials/           # Learning-oriented guides
│   │   ├── _meta.js         # Section navigation
│   │   ├── getting-started.mdx
│   │   ├── first-query.mdx
│   │   ├── web-interface.mdx
│   │   ├── terminal-usage.mdx
│   │   ├── ssh-chat.mdx
│   │   ├── dns-queries.mdx
│   │   └── api-basics.mdx
│   ├── guides/              # Task-oriented how-tos
│   │   ├── _meta.js
│   │   ├── self-hosting.mdx
│   │   └── [other guides - stubs]
│   ├── explanations/        # Understanding-oriented
│   │   ├── _meta.js
│   │   └── [explanation docs - stubs]
│   └── reference/           # Information-oriented
│       ├── _meta.js
│       └── [reference docs - stubs]
├── next.config.mjs          # Next.js + Nextra configuration
├── theme.config.tsx         # Nextra theme configuration
├── tsconfig.json            # TypeScript configuration
├── package.json             # Project dependencies
├── .gitignore              # Git ignore rules
├── README.md               # Project documentation
└── nextra-dev.log          # Development server logs
```

## Configuration Files

### 1. next.config.mjs
```javascript
import nextra from 'nextra'

const withNextra = nextra({
  theme: 'nextra-theme-docs',
  themeConfig: './theme.config.tsx',
  defaultShowCopyCode: true  // Copy button on code blocks
})

export default withNextra({
  reactStrictMode: true,
  basePath: process.env.NODE_ENV === 'production' ? '/docs' : '',
  images: {
    unoptimized: true
  }
})
```

### 2. theme.config.tsx
Key configurations:
- **Logo**: "ch.at"
- **Project link**: GitHub repository
- **Edit link**: "Edit this page on GitHub →"
- **Dark mode**: Enabled by default
- **Primary hue**: 200 (blue)
- **Sidebar**: Collapsible with toggle button
- **TOC**: Floating table of contents
- **Banner**: DoNutSentry announcement

### 3. _meta.js Files
Navigation structure using ES modules:
```javascript
export default {
  'page-slug': 'Page Title',
  'another-page': 'Another Title'
}
```

## The Diátaxis Framework

We've implemented the Diátaxis documentation framework which divides docs into four types:

### 1. Tutorials (/tutorials)
- **Purpose**: Learning-oriented, hand-holding
- **Examples**: Getting Started, Your First Query
- **Style**: Step-by-step instructions with clear outcomes

### 2. How-to Guides (/guides)
- **Purpose**: Goal-oriented, problem-solving
- **Examples**: Self-hosting, Docker deployment
- **Style**: Recipes for specific tasks

### 3. Explanations (/explanations)
- **Purpose**: Understanding-oriented, theoretical
- **Examples**: Architecture overview, Privacy model
- **Style**: Discussions and background information

### 4. Reference (/reference)
- **Purpose**: Information-oriented, technical descriptions
- **Examples**: API reference, Configuration options
- **Style**: Dry, factual, comprehensive

## Development Workflow

### Starting the Dev Server
```bash
cd website
npm run dev
# Server starts on port 3000+ (increments if ports are busy)
```

### Running with Screen (for persistent sessions)
```bash
screen -dmS nextra-docs bash -c 'npm run dev 2>&1 | tee nextra-dev.log'
# Attach: screen -r nextra-docs
# Detach: Ctrl+A, D
```

### Building for Production
```bash
npm run build  # Creates optimized production build
npm start      # Serves production build
```

## MDX Components Available

Nextra provides rich components for documentation:

### Callout
```mdx
import { Callout } from 'nextra/components'

<Callout type="info">
  This is an informational callout
</Callout>
```
Types: info, warning, error, default

### Tabs
```mdx
import { Tabs, Tab } from 'nextra/components'

<Tabs items={['npm', 'yarn', 'pnpm']}>
  <Tab>npm install</Tab>
  <Tab>yarn add</Tab>
  <Tab>pnpm add</Tab>
</Tabs>
```

### Steps
```mdx
import { Steps } from 'nextra/components'

<Steps>
### Step 1
First do this

### Step 2
Then do that
</Steps>
```

### Cards
```mdx
import { Cards, Card } from 'nextra/components'

<Cards>
  <Card title="Feature 1" href="/link">
    Description
  </Card>
</Cards>
```

### FileTree
```mdx
import { FileTree } from 'nextra/components'

<FileTree>
  <FileTree.Folder name="src" defaultOpen>
    <FileTree.File name="index.js" />
  </FileTree.Folder>
</FileTree>
```

## Current Issues & Solutions

### Issue 1: React Component Errors
**Error**: "React.jsx: type is invalid -- expected a string..."
**Cause**: The `Cards` and `Card` components in index.mdx aren't properly imported
**Solution**: Either remove these components or ensure proper imports

### Issue 2: Build Cache
**Problem**: Nextra aggressively caches, making it hard to see changes
**Solution**: 
- Delete `.next` directory
- Restart the server
- Touch files to trigger rebuild

### Issue 3: Multiple Server Instances
**Problem**: Multiple dev servers started on different ports
**Solution**: Kill extra processes, use single screen session

## Next Steps

### 1. Fix Component Errors
- Fix or remove the Cards components in index.mdx
- Ensure all Nextra components are properly imported

### 2. Complete Content
Replace stub files with actual content for:
- How-to guides (Docker, systemd, SSL, etc.)
- Explanations (architecture, protocols, etc.)
- Reference documentation (API, configuration, etc.)

### 3. Enhance Features
- Configure search properly
- Add custom CSS if needed
- Set up proper GitHub repository links
- Configure production deployment

### 4. Deploy
Options for deployment:
- **Vercel** (recommended): `vercel deploy`
- **Netlify**: Connect GitHub repo
- **GitHub Pages**: Use `next export`
- **Self-hosted**: `npm run build && npm start`

## Best Practices

### Content Writing
1. **Use MDX features**: Take advantage of React components
2. **Code blocks**: Always specify language for syntax highlighting
3. **Front matter**: Add metadata if needed
4. **Images**: Store in `public/` directory
5. **Links**: Use relative links for internal navigation

### File Organization
1. **Naming**: Use kebab-case for file names
2. **_meta.js**: Keep navigation titles concise
3. **Nesting**: Don't go deeper than 3 levels
4. **Index files**: Each directory should have an index

### Contributing Workflow
1. Fork the repository
2. Create/edit `.mdx` files in appropriate directory
3. Update `_meta.js` if adding new pages
4. Test locally with `npm run dev`
5. Submit pull request

## CLI Commands Reference

```bash
# Development
npm run dev          # Start dev server with hot reload
npm run build        # Build for production
npm start           # Start production server
npm run lint        # Run linting

# Git workflow
git add website/    # Stage changes
git commit -m "docs: update X"
git push           # Push to remote

# Process management
screen -ls         # List screen sessions
screen -r NAME     # Attach to session
lsof -i :3000      # Check what's on port
kill -9 PID        # Force kill process

# File operations
touch file.mdx     # Create new file
rm -rf .next       # Clear build cache
```

## Environment Variables

```bash
# Development
NODE_ENV=development

# Production
NODE_ENV=production
# Base path for subdirectory hosting
NEXT_PUBLIC_BASE_PATH=/docs
```

## Resources

- **Nextra Documentation**: https://nextra.site
- **Next.js Documentation**: https://nextjs.org/docs
- **MDX**: https://mdxjs.com
- **Diátaxis**: https://diataxis.fr

## Summary

We've successfully set up a modern documentation website using Nextra 3 that provides:
- Clean, Stripe-like design
- GitHub integration for contributions
- Search functionality
- Mobile responsiveness
- Dark mode support
- Structured content organization (Diátaxis)

The site is ready for content population and minor bug fixes before deployment. The architecture supports easy contributions through GitHub pull requests, making it ideal for open-source documentation.