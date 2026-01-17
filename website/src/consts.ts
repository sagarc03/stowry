// Place any global data in this file.
// You can import this data from anywhere in your site by using the `import` keyword.

export const SITE_TITLE = 'Stowry';
export const SITE_DESCRIPTION =
  'Lightweight, self-hosted object storage server with pluggable backends and presigned URL authentication.';

export const SITE_METADATA = {
  title: {
    default: SITE_TITLE,
    template: '%s | Stowry',
  },
  description: SITE_DESCRIPTION,
  keywords: [
    'object storage',
    'self-hosted',
    'presigned urls',
    'golang',
    's3 alternative',
    'file storage',
    'sqlite',
    'postgresql',
  ],
  authors: [{ name: 'Stowry Team' }],
  creator: 'Stowry Team',
  publisher: 'Stowry',
  robots: {
    index: true,
    follow: true,
  },
  icons: {
    icon: [
      { url: '/favicon/favicon.ico', sizes: '48x48' },
      { url: '/favicon/favicon.svg', type: 'image/svg+xml' },
      { url: '/favicon/favicon-96x96.png', sizes: '96x96', type: 'image/png' },
      { url: '/favicon/favicon.svg', type: 'image/svg+xml' },
      { url: '/favicon/favicon.ico' },
    ],
    apple: [{ url: '/favicon/apple-touch-icon.png', sizes: '180x180' }],
    shortcut: [{ url: '/favicon/favicon.ico' }],
  },
  openGraph: {
    title: SITE_TITLE,
    description: SITE_DESCRIPTION,
    siteName: 'Stowry',
    images: [
      {
        url: '/og-image.jpg',
        width: 1200,
        height: 630,
        alt: 'Stowry - Self-hosted object storage, simplified',
      },
    ],
  },
  twitter: {
    card: 'summary_large_image',
    title: SITE_TITLE,
    description: SITE_DESCRIPTION,
    images: ['/og-image.jpg'],
  },
};
