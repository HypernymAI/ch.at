import React from 'react'
import { DocsThemeConfig } from 'nextra-theme-docs'

const config: DocsThemeConfig = {
  logo: <span>ch.at</span>,
  project: {
    link: 'https://github.com/yourusername/ch.at',
  },
  docsRepositoryBase: 'https://github.com/yourusername/ch.at/tree/main/website',
  editLink: {
    text: 'Edit this page on GitHub →'
  },
  footer: {
    text: 'ch.at - Universal Basic Intelligence',
  },
  darkMode: true,
  primaryHue: 200,
  navigation: true,
  sidebar: {
    toggleButton: true,
    defaultMenuCollapseLevel: 1
  },
  toc: {
    float: true,
    extraContent: null
  },
  feedback: {
    content: 'Questions? Give us feedback →',
    labels: 'documentation'
  },
  useNextSeoProps() {
    return {
      titleTemplate: '%s – ch.at'
    }
  },
  head: (
    <>
      <meta name="viewport" content="width=device-width, initial-scale=1.0" />
      <meta property="og:title" content="ch.at Documentation" />
      <meta property="og:description" content="Universal AI chat service accessible through HTTP, SSH, DNS, and API" />
    </>
  ),
  banner: {
    key: 'donutsentry-update',
    text: (
      <a href="/guides/donutsentry" target="_blank">
        🍩 DoNutSentry protocol now supports large queries through sessions. Learn more →
      </a>
    )
  }
}

export default config