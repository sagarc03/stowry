'use client';
import { useEffect, useState } from 'react';

import { FaGithub } from 'react-icons/fa6';

import Logo from '@/components/layout/logo';
import { EXTERNAL_LINKS } from '@/constants/external-links';
import { cn } from '@/lib/utils';

const Footer = () => {
  const [theme, setTheme] = useState<'light' | 'dark'>('light');
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);

    // Get initial theme from localStorage, default to 'light' if none exists
    const savedTheme = localStorage.getItem('theme') as 'light' | 'dark' | null;
    setTheme(savedTheme || 'light');

    // Listen for theme changes
    const handleStorageChange = () => {
      const newTheme = localStorage.getItem('theme') as 'light' | 'dark' | null;
      if (newTheme) {
        setTheme(newTheme);
      }
    };

    window.addEventListener('storage', handleStorageChange);

    // Listen for direct DOM class changes (for immediate updates)
    const observer = new MutationObserver(() => {
      const isDark = document.documentElement.classList.contains('dark');
      setTheme(isDark ? 'dark' : 'light');
    });

    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['class'],
    });

    return () => {
      window.removeEventListener('storage', handleStorageChange);
      observer.disconnect();
    };
  }, []);

  // Prevent hydration mismatch by using a consistent theme class until mounted
  // Footer is in light mode when dark theme is applied (inverted behavior)
  const themeClass =
    mounted && theme === 'dark'
      ? 'light bg-foreground text-background [&_*]:border-border/30'
      : 'dark bg-background text-foreground';

  // Logo should be inverted when footer has light background (dark theme)
  // and not inverted when footer has dark background (light theme)
  const logoWordmarkClass = cn(
    'w-[min(100%,400px)] translate-y-1/4 md:translate-y-1/3 md:h-32 md:w-full lg:h-73 opacity-10',
    mounted && theme === 'dark' ? 'invert-0' : 'invert',
  );

  return (
    <footer className={cn('overflow-hidden', themeClass)}>
      <div className="container">
        {/* Navigation Links Section */}
        <div className="bordered-div-padding border-x">
          <div className="grid grid-cols-2 gap-8 md:grid-cols-4">
            <div>
              <h3 className="font-weight-display text-lg mb-4">Documentation</h3>
              <ul className="space-y-2 text-muted-foreground">
                <li><a href="/getting-started" className="hover:text-foreground transition-colors">Getting Started</a></li>
                <li><a href="/installation" className="hover:text-foreground transition-colors">Installation</a></li>
                <li><a href="/configuration" className="hover:text-foreground transition-colors">Configuration</a></li>
                <li><a href="/api-reference" className="hover:text-foreground transition-colors">API Reference</a></li>
              </ul>
            </div>
            <div>
              <h3 className="font-weight-display text-lg mb-4">Guides</h3>
              <ul className="space-y-2 text-muted-foreground">
                <li><a href="/server-modes" className="hover:text-foreground transition-colors">Server Modes</a></li>
                <li><a href="/authentication" className="hover:text-foreground transition-colors">Authentication</a></li>
                <li><a href="/sdks" className="hover:text-foreground transition-colors">Client SDKs</a></li>
                <li><a href="/examples" className="hover:text-foreground transition-colors">Examples</a></li>
              </ul>
            </div>
            <div>
              <h3 className="font-weight-display text-lg mb-4">Reference</h3>
              <ul className="space-y-2 text-muted-foreground">
                <li><a href="/cli-reference" className="hover:text-foreground transition-colors">CLI Commands</a></li>
                <li><a href="/architecture" className="hover:text-foreground transition-colors">Architecture</a></li>
                <li><a href="/deployment" className="hover:text-foreground transition-colors">Deployment</a></li>
              </ul>
            </div>
            <div>
              <h3 className="font-weight-display text-lg mb-4">Community</h3>
              <ul className="space-y-2 text-muted-foreground">
                <li>
                  <a
                    href={EXTERNAL_LINKS.GITHUB}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="hover:text-foreground transition-colors flex items-center gap-2"
                  >
                    <FaGithub className="size-4" />
                    GitHub
                  </a>
                </li>
                <li><a href="/faq" className="hover:text-foreground transition-colors">FAQ</a></li>
              </ul>
            </div>
          </div>
        </div>

        {/* Social and Status Section */}
        <div className="flex flex-col justify-between border-x border-t md:flex-row">
          <div className="bordered-div-padding flex items-center space-x-3">
            <a
              href={EXTERNAL_LINKS.GITHUB}
              className="px-3 py-2.5 transition-opacity hover:opacity-80"
              target="_blank"
              rel="noopener noreferrer"
              aria-label="GitHub"
            >
              <FaGithub className="size-5" />
            </a>
          </div>
          <div className="bordered-div-padding flex items-center border-t text-muted-foreground md:border-t-0">
            <span className="text-sm">
              Built with Go. Open source.
            </span>
          </div>
        </div>

        {/* Large Logo */}
        <Logo
          className="justify-center border-x border-t"
          iconClassName="hidden"
          wordmarkClassName={logoWordmarkClass}
        />
      </div>
    </footer>
  );
};

export default Footer;
