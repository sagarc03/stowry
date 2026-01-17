'use client';

import { useState } from 'react';

import { ChevronDown } from 'lucide-react';

import { Meteors } from '@/components/magicui/meteors';
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';

type Category =
  | 'General'
  | 'Authentication'
  | 'Development';

const categories: Category[] = [
  'General',
  'Authentication',
  'Development',
];

type FAQ = {
  question: string;
  answer: React.ReactNode;
};

const faqs: Record<Category, FAQ[]> = {
  General: [
    {
      question: 'What is Stowry?',
      answer:
        'Stowry is a lightweight, self-hosted object storage server written in Go. It provides a simple HTTP REST API for storing and retrieving files with secure presigned URL authentication. Think of it as a minimal S3-like storage server you can run anywhere.',
    },
    {
      question: 'Is Stowry S3-compatible?',
      answer: (
        <>
          Stowry is not fully S3-compatible, but it supports AWS Signature V4 for
          presigned URLs. This means you can use AWS SDKs (Go, Python, JavaScript)
          to generate presigned URLs for upload and download operations. See the{' '}
          <a href="/sdks" className="text-primary underline">
            Client SDKs
          </a>{' '}
          documentation for examples.
        </>
      ),
    },
    {
      question: 'Which database should I use - SQLite or PostgreSQL?',
      answer:
        'Use SQLite for development, testing, and small deployments. It requires no external dependencies and is embedded in the Stowry binary. Use PostgreSQL for production workloads that require higher concurrency, replication, or integration with existing database infrastructure.',
    },
    {
      question: 'What are the three server modes?',
      answer: (
        <>
          <strong>Store mode</strong> (default): Standard object storage API. Returns exact paths or 404.
          <br /><br />
          <strong>Static mode</strong>: Static file server with automatic index.html fallback for directories.
          <br /><br />
          <strong>SPA mode</strong>: Single-page application hosting. All unknown paths return /index.html for client-side routing.
        </>
      ),
    },
  ],
  Authentication: [
    {
      question: 'How do presigned URLs work?',
      answer: (
        <>
          Presigned URLs are temporary, signed URLs that grant access to specific objects without sharing your credentials. Your server generates them using your access key and secret key, then shares the URL with clients. The signature expires after a configurable duration (default 15 minutes, max 7 days). See the{' '}
          <a href="/authentication" className="text-primary underline">
            Authentication guide
          </a>{' '}
          for details.
        </>
      ),
    },
    {
      question: 'Can I make files publicly accessible?',
      answer:
        'Yes, you can enable public_read in your configuration to allow unauthenticated GET requests. This is useful for public assets like images or static files. For production, we recommend keeping public_write disabled to require authentication for uploads and deletes.',
    },
    {
      question: 'How do I generate presigned URLs?',
      answer: (
        <>
          Use one of our official SDKs (stowry-go, stowrypy, stowryjs) or use AWS SDKs with your Stowry endpoint. The SDKs provide simple methods like <code>client.PresignPut("/path", 900)</code> to generate URLs valid for 15 minutes. See the{' '}
          <a href="/sdks" className="text-primary underline">
            SDK documentation
          </a>{' '}
          for examples in multiple languages.
        </>
      ),
    },
  ],
  Development: [
    {
      question: 'How do I run Stowry locally?',
      answer: (
        <>
          Download the binary for your platform, create a config.yaml file, and run:
          <br /><br />
          <code>./stowry serve</code>
          <br /><br />
          Stowry will start on port 5708 with SQLite as the default database. See the{' '}
          <a href="/getting-started" className="text-primary underline">
            Getting Started guide
          </a>{' '}
          for a 5-minute quickstart.
        </>
      ),
    },
    {
      question: 'Can I contribute to Stowry?',
      answer: (
        <>
          Yes! Stowry is open source. You can contribute code, report bugs, suggest features, or help improve documentation. Visit our{' '}
          <a href="https://github.com/sagarc03/stowry" className="text-primary underline" target="_blank" rel="noopener noreferrer">
            GitHub repository
          </a>{' '}
          to get started.
        </>
      ),
    },
    {
      question: 'What programming languages can I use with Stowry?',
      answer:
        'Stowry provides official SDKs for Go, Python, and JavaScript. Since it uses standard HTTP with presigned URLs, you can integrate with any language that supports HTTP requests. AWS SDKs also work for generating presigned URLs thanks to Signature V4 support.',
    },
  ],
};

export function FAQSection() {
  const [activeTab, setActiveTab] = useState<Category>(categories[0]);

  return (
    <section className="overflow-hidden">
      <div className="container divide-y">
        <div className="hidden border-x border-b-0 p-7.5 md:block" />

        <div className="bordered-div-padding border-x">
          <h1 className="font-weight-display text-2xl leading-snug tracking-tighter md:text-3xl lg:text-5xl">
            Frequently Asked Questions
          </h1>
          <div className="mt-6 block md:hidden">
            <Select
              value={activeTab}
              onValueChange={(value) => setActiveTab(value as Category)}
            >
              <SelectTrigger className="w-full">
                <SelectValue>{activeTab}</SelectValue>
              </SelectTrigger>
              <SelectContent>
                {categories.map((category) => (
                  <SelectItem key={category} value={category}>
                    {category}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>

        <div className="bordered-div-padding relative hidden border-x md:block">
          <div className="absolute left-full h-[150%] w-[50vw] -translate-y-[90%] overflow-hidden border-y">
            <Meteors
              number={1000}
              angle={65}
              maxDuration={20}
              minDuration={5}
              className="opacity-10 [&>div]:opacity-10"
            />
          </div>
          <Tabs
            value={activeTab}
            onValueChange={(value) => setActiveTab(value as Category)}
            className=""
          >
            <TabsList className="flex gap-3">
              {categories.map((category) => (
                <TabsTrigger key={category} value={category}>
                  {category}
                </TabsTrigger>
              ))}
            </TabsList>
          </Tabs>
        </div>

        <div className="border-x">
          <Accordion type="single" collapsible>
            {faqs[activeTab].map((faq, index) => (
              <AccordionItem key={index} value={`item-${index}`}>
                <AccordionTrigger className="bordered-div-padding font-weight-display flex w-full items-center justify-between !pb-4 text-base hover:no-underline md:!pb-6 md:text-xl [&>svg]:hidden [&[data-state=open]_svg]:rotate-180">
                  <span>{faq.question}</span>
                  <div className="bg-card flex size-8 items-center justify-center rounded-sm border">
                    <ChevronDown className="size-5 shrink-0 tracking-tight transition-transform duration-200" />
                  </div>
                </AccordionTrigger>
                <AccordionContent className="text-muted-foreground bordered-div-padding max-w-2xl !pt-0 leading-relaxed tracking-tight">
                  {faq.answer}
                </AccordionContent>
              </AccordionItem>
            ))}
          </Accordion>
        </div>
        <div className="hidden border-x p-20 md:block" />
      </div>
    </section>
  );
}
