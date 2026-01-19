'use client';

import { ArrowRight } from 'lucide-react';
import { FaGithub } from 'react-icons/fa6';

import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { EXTERNAL_LINKS } from '@/constants/external-links';

export function Hero() {
  return (
    <section className="relative overflow-hidden">
      <div className="container">
        <div className="bordered-div-padding relative flex flex-col items-center gap-8 border-x text-center md:gap-10 lg:gap-16 lg:!py-25">
          {/* Main Heading */}
          <div className="max-w-4xl space-y-6 md:space-y-8 lg:space-y-12">
            {/* Version Badge */}
            <a
              href={EXTERNAL_LINKS.GITHUB + '/releases/latest'}
              target="_blank"
              rel="noopener noreferrer"
              className="relative inline-flex items-center overflow-hidden rounded-sm border px-4 py-2 text-sm"
            >
              <span className="text-muted-foreground">
                {import.meta.env.PUBLIC_STOWRY_VERSION || 'dev'}
              </span>
              <ArrowRight className="ml-2 size-4" />
            </a>
            <h1 className="font-weight-display text-2xl leading-snug tracking-tighter md:text-3xl lg:text-5xl">
              Self-hosted object storage,{' '}
              <span className="block">simplified.</span>
            </h1>
            <p className="text-muted-foreground mx-auto max-w-[700px] text-sm leading-relaxed md:text-lg lg:text-xl">
              Stowry is a lightweight object storage server written in Go.
              Deploy a single binary, configure your metadata backend, and
              start storing files with secure presigned URL authentication.
            </p>
          </div>

          {/* CTA Buttons */}
          <div className="flex flex-wrap items-center justify-center gap-4 md:gap-6">
            <Button asChild>
              <a href="/getting-started">
                Get Started
                <ArrowRight className="ml-2 size-4" />
              </a>
            </Button>
            <Button asChild variant="outline">
              <a href={EXTERNAL_LINKS.GITHUB} target="_blank" rel="noopener noreferrer">
                <FaGithub className="size-5" />
                View on GitHub
              </a>
            </Button>
          </div>
          <div
            className={cn(
              'pointer-events-none absolute top-0 left-full hidden h-[calc(100%+1px)] w-screen overflow-hidden border-b text-start select-none lg:block',
            )}
            aria-hidden="true"
            role="presentation"
          >
            <p className="p-4 whitespace-pre opacity-20">{`# config.yaml
server:
  port: 5708
  mode: store

database:
  type: sqlite
  dsn: stowry.db

storage:
  path: ./data

auth:
  region: us-east-1
  service: s3
  keys:
    - access_key: YOUR_KEY
      secret_key: YOUR_SECRET`}</p>
          </div>
        </div>
      </div>
      <div className="container">
        <div className="bordered-div-padding flex flex-col items-center justify-center border gap-8">
          {/* Quick Install */}
          <div className="w-full max-w-2xl">
            <div className="rounded-lg border bg-muted/50 p-4 font-mono text-sm">
              <div className="text-muted-foreground mb-2"># Quick start</div>
              <div className="text-foreground">curl -LO https://github.com/sagarc03/stowry/releases/latest/download/stowry-linux-amd64</div>
              <div className="text-foreground">chmod +x stowry-linux-amd64 && ./stowry-linux-amd64 serve</div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
