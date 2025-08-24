import nextra from 'nextra'

const withNextra = nextra({
  theme: 'nextra-theme-docs',
  themeConfig: './theme.config.tsx',
  defaultShowCopyCode: true
})

export default withNextra({
  reactStrictMode: true,
  basePath: process.env.NODE_ENV === 'production' ? '/docs' : '',
  images: {
    unoptimized: true
  }
})