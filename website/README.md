# ch.at Documentation Website

This is the documentation website for ch.at, built with [Nextra](https://nextra.site/) - a Next.js based documentation framework.

## Development

```bash
# Install dependencies
npm install

# Start development server
npm run dev

# Build for production
npm run build

# Start production server
npm start
```

The site will be available at http://localhost:3000 (or 3001 if 3000 is in use).

## Structure

The documentation follows the [Diátaxis framework](https://diataxis.fr/):

```
pages/
├── index.mdx           # Home page
├── tutorials/          # Learning-oriented guides
├── guides/             # Task-oriented how-tos  
├── explanations/       # Understanding-oriented discussions
└── reference/          # Information-oriented technical descriptions
```

## Contributing

1. Fork the repository
2. Create a new branch for your changes
3. Edit the relevant `.mdx` files in the `pages/` directory
4. Test locally with `npm run dev`
5. Submit a pull request

### Adding New Pages

1. Create a new `.mdx` file in the appropriate directory
2. Update the `_meta.json` file in that directory to include your page
3. Use MDX components from Nextra for rich content:
   - `<Callout>` for notes and warnings
   - `<Tabs>` for tabbed content
   - `<Steps>` for step-by-step guides
   - `<FileTree>` for file structure diagrams

### Configuration

- `theme.config.tsx` - Theme configuration (logo, footer, etc.)
- `next.config.mjs` - Next.js and Nextra configuration

## Key Features

- **GitHub Edit Button**: Every page has an "Edit this page on GitHub" link
- **Dark Mode**: Automatic dark/light theme switching
- **Search**: Built-in search functionality
- **Mobile Responsive**: Works on all devices
- **Fast**: Static generation with Next.js

## Deployment

The site can be deployed to any static hosting service:

- Vercel (recommended)
- Netlify
- GitHub Pages
- Self-hosted with `npm run build && npm start`