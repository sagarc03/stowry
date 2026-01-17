// @ts-check
import mdx from '@astrojs/mdx';
import react from '@astrojs/react';
import sitemap from '@astrojs/sitemap';
import starlight from '@astrojs/starlight';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'astro/config';

// https://astro.build/config
export default defineConfig({
  site: 'https://stowry.dev',
  integrations: [
    starlight({
      title: 'Stowry Docs',
      description: 'Documentation for Stowry - Self-hosted object storage, simplified',
      favicon: '/favicon/favicon.svg',
      logo: {
        light: './public/layout/logo-light.svg',
        dark: './public/layout/logo-dark.svg',
        replacesTitle: true,
      },
      sidebar: [
        {
          label: 'Getting Started',
          items: [
            { label: 'Overview', slug: 'index' },
            { label: 'Quick Start', slug: 'getting-started' },
            { label: 'Installation', slug: 'installation' },
            { label: 'Configuration', slug: 'configuration' },
          ],
        },
        {
          label: 'API Reference',
          items: [
            { label: 'HTTP API', slug: 'api-reference' },
            { label: 'CLI Commands', slug: 'cli-reference' },
            { label: 'Authentication', slug: 'authentication' },
          ],
        },
        {
          label: 'Guides',
          items: [
            { label: 'Server Modes', slug: 'server-modes' },
            { label: 'Client SDKs', slug: 'sdks' },
            { label: 'Examples', slug: 'examples' },
          ],
        },
        {
          label: 'Reference',
          items: [
            { label: 'Architecture', slug: 'architecture' },
            { label: 'Deployment', slug: 'deployment' },
          ],
        },
      ],
      expressiveCode: {
        themes: ['github-light', 'github-dark'],
      },
    }),
    mdx(),
    sitemap(),
    react(),
  ],
  vite: {
    plugins: [tailwindcss()],
  },
});
