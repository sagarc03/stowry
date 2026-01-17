import { Package, Lock, Layers, Database } from 'lucide-react';

import { PlusSigns } from '@/components/icons/plus-signs';
import { cn } from '@/lib/utils';

const features = [
  {
    icon: Package,
    title: (
      <>
        Single
        <br />
        Binary
      </>
    ),
    description: 'No external dependencies required.',
    subDescription:
      'Ship one binary with SQLite embedded for development, or connect to PostgreSQL for production. Zero configuration to get started.',
  },
  {
    icon: Lock,
    title: (
      <>
        Presigned
        <br />
        URLs
      </>
    ),
    description: 'Secure file access without exposing credentials.',
    subDescription:
      'Generate time-limited URLs for upload, download, and delete operations. Works with AWS SDKs you already know.',
  },
  {
    icon: Layers,
    title: (
      <>
        Three Server
        <br />
        Modes
      </>
    ),
    description: 'Store, Static, or SPA.',
    subDescription:
      'Use Store mode for object storage API, Static mode for file serving with index.html fallback, or SPA mode for single-page applications.',
  },
  {
    icon: Database,
    title: (
      <>
        Pluggable
        <br />
        Backends
      </>
    ),
    description: 'Choose SQLite or PostgreSQL.',
    subDescription:
      'SQLite for simplicity and development, PostgreSQL for scale and production. Switch backends without changing your application code.',
  },
];

export function Features() {
  return (
    <section className="container">
      <div className="grid grid-cols-1 border border-t-0 md:grid-cols-2">
        {features.map((feature, index) => (
          <div
            key={index}
            className={cn(
              'bordered-div-padding relative space-y-8',
              index == 0 && 'border-b md:border-e',
              index == 1 && 'border-b md:border-b-0',
              index == 3 && 'border-t md:border-s',
            )}
          >
            {index === 0 && (
              // Height is 100% + 2px to account for parent border not being included in the calculation
              <PlusSigns className="absolute inset-0 -mt-0.25 hidden !h-[calc(100%+2px)] -translate-x-full border-y md:block" />
            )}
            <div className="space-y-4 md:space-y-6">
              <div className="space-y-4">
                <h2 className="text-muted-foreground flex items-center gap-2 text-sm leading-snug font-medium md:text-base">
                  <feature.icon className="size-5" />
                  {feature.title}
                </h2>
                <h3 className="text-foreground font-weight-display leading-snug md:text-xl">
                  {feature.description}
                </h3>
              </div>
              <p className="text-muted-foreground text-sm leading-relaxed md:text-base">
                {feature.subDescription}
              </p>
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}
