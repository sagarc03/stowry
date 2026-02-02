// @ts-check
import mdx from '@astrojs/mdx';
import react from '@astrojs/react';
import sitemap from '@astrojs/sitemap';
import starlight from '@astrojs/starlight';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'astro/config';

export default defineConfig({
  site: 'https://stowry.dev',
  integrations: [
    starlight({
      title: 'Stowry',
      description: 'Documentation for Stowry - Self-hosted object storage, simplified',
      customCss: ['./src/styles/global.css'],
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
            { label: 'Server CLI', slug: 'cli-reference' },
            { label: 'Client CLI', slug: 'client-cli' },
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
