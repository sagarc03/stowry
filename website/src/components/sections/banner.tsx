'use client';

import { useEffect, useState } from 'react';

import { X } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

const Banner = ({ url = 'https://shadcnblocks.com' }: { url?: string }) => {
  const [isVisible, setIsVisible] = useState(true);
  const [isClient, setIsClient] = useState(false);

  // Check localStorage to see if banner was previously dismissed
  useEffect(() => {
    setIsClient(true);
    const bannerDismissed = localStorage.getItem('banner-dismissed');
    if (bannerDismissed === 'true') {
      setIsVisible(false);
    }
  }, []);

  const handleDismiss = () => {
    setIsVisible(false);
    localStorage.setItem('banner-dismissed', 'true');
  };

  // Don't render anything until client-side hydration is complete
  if (!isClient || !isVisible) {
    return null;
  }

  return (
    <div className="bg-primary relative">
      <div className="container flex items-center justify-between gap-4 py-3 pr-12">
        <div className="flex flex-1 items-center justify-center gap-3 sm:gap-4">
          <span className="text-primary-foreground text-center text-sm font-medium">
            Purchase this theme on{' '}
            <span className="font-semibold">shadcnblocks.com</span>
          </span>
          <Button size="sm" variant="secondary" asChild>
            <a href={url} target="_blank">
              Get Template
            </a>
          </Button>
        </div>
        <button
          onClick={handleDismiss}
          className={cn(
            'absolute top-1/2 right-4 -translate-y-1/2 rounded-sm p-1.5',
            'text-primary-foreground/70 hover:text-primary-foreground',
            'transition-all duration-200 hover:scale-110 hover:bg-white/10',
            'focus:ring-2 focus:ring-white/30 focus:outline-none',
          )}
          aria-label="Close banner"
        >
          <X className="size-3.5" />
        </button>
      </div>
    </div>
  );
};

export default Banner;
