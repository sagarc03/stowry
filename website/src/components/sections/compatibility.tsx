'use client';

import { useEffect, useRef, useState } from 'react';

import { Check, Code, Copy } from 'lucide-react';
import { AnimatePresence, motion } from 'motion/react';

import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { useMediaQuery } from '@/hooks/use-media-query';
import { cn } from '@/lib/utils';

const frameworks = [
  {
    name: 'Go',
    lang: 'go',
    code: `package main

import (
    "fmt"
    "net/http"
    stowry "github.com/sagarc03/stowry-go"
)

func main() {
    client := stowry.NewClient(
        "http://localhost:5708",
        "YOUR_ACCESS_KEY",
        "YOUR_SECRET_KEY",
    )

    // Generate presigned URLs
    uploadURL := client.PresignPut("/photos/vacation.jpg", 900)
    downloadURL := client.PresignGet("/photos/vacation.jpg", 900)

    fmt.Println("Upload URL:", uploadURL)
    fmt.Println("Download URL:", downloadURL)
}`,
  },
  {
    name: 'Python',
    lang: 'python',
    code: `import requests
from stowrypy import StowryClient

client = StowryClient(
    endpoint="http://localhost:5708",
    access_key="YOUR_ACCESS_KEY",
    secret_key="YOUR_SECRET_KEY",
)

# Upload a file
put_url = client.presign_put("/photos/vacation.jpg", expires=900)
with open("vacation.jpg", "rb") as f:
    requests.put(put_url, data=f, headers={"Content-Type": "image/jpeg"})

# Download a file
get_url = client.presign_get("/photos/vacation.jpg", expires=900)
response = requests.get(get_url)
print(f"Downloaded {len(response.content)} bytes")`,
  },
  {
    name: 'JavaScript',
    lang: 'javascript',
    code: `import { StowryClient } from 'stowryjs';

const client = new StowryClient({
  endpoint: 'http://localhost:5708',
  accessKey: 'YOUR_ACCESS_KEY',
  secretKey: 'YOUR_SECRET_KEY',
});

// Upload a file
const putUrl = await client.presignPut('/photos/vacation.jpg', 900);
await fetch(putUrl, {
  method: 'PUT',
  body: file,
  headers: { 'Content-Type': 'image/jpeg' },
});

// Download a file
const getUrl = await client.presignGet('/photos/vacation.jpg', 900);
const response = await fetch(getUrl);
console.log('Downloaded', response.headers.get('content-length'), 'bytes');`,
  },
  {
    name: 'AWS SDK',
    lang: 'typescript',
    code: `import { S3Client, GetObjectCommand, PutObjectCommand } from '@aws-sdk/client-s3';
import { getSignedUrl } from '@aws-sdk/s3-request-presigner';

const client = new S3Client({
  endpoint: 'http://localhost:5708',
  region: 'us-east-1',
  credentials: {
    accessKeyId: 'YOUR_ACCESS_KEY',
    secretAccessKey: 'YOUR_SECRET_KEY',
  },
  forcePathStyle: true,
});

// Generate presigned upload URL
const putUrl = await getSignedUrl(
  client,
  new PutObjectCommand({ Bucket: 'bucket', Key: 'photos/vacation.jpg' }),
  { expiresIn: 900 }
);

// Generate presigned download URL
const getUrl = await getSignedUrl(
  client,
  new GetObjectCommand({ Bucket: 'bucket', Key: 'photos/vacation.jpg' }),
  { expiresIn: 900 }
);`,
  },
];

export function Compatibility() {
  const [activeTab, setActiveTab] = useState(frameworks[0].name);
  const [highlightedCode, setHighlightedCode] = useState<{
    light: Record<string, string>;
    dark: Record<string, string>;
  }>({
    light: {},
    dark: {},
  });
  const [isLoading, setIsLoading] = useState(true);
  const [theme, setTheme] = useState<'light' | 'dark'>('light');
  const { isAtLeast } = useMediaQuery();
  const isMobile = !isAtLeast('md');

  useEffect(() => {
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

  useEffect(() => {
    async function highlightCode() {
      try {
        const shiki = await import('shiki');
        const highlighter = await shiki.createHighlighter({
          themes: ['github-dark', 'github-light'],
          langs: [
            'typescript',
            'javascript',
            'python',
            'go',
          ],
        });

        const lightHighlighted: Record<string, string> = {};
        const darkHighlighted: Record<string, string> = {};

        for (const framework of frameworks) {
          lightHighlighted[framework.name] = highlighter.codeToHtml(
            framework.code,
            {
              lang: framework.lang,
              theme: 'github-light',
            },
          );

          darkHighlighted[framework.name] = highlighter.codeToHtml(
            framework.code,
            {
              lang: framework.lang,
              theme: 'github-dark',
            },
          );
        }

        setHighlightedCode({
          light: lightHighlighted,
          dark: darkHighlighted,
        });
      } catch (error) {
        console.error('Failed to highlight code:', error);
      } finally {
        setIsLoading(false);
      }
    }

    highlightCode();
  }, []);

  // Get the current highlighted code based on theme
  const currentHighlightedCode =
    theme === 'dark' ? highlightedCode.dark : highlightedCode.light;

  return (
    <section className="container">
      <div className="bordered-div-padding border border-t-0">
        <div className="space-y-4">
          <h3 className="text-muted-foreground flex items-center gap-2 text-sm leading-snug font-medium md:text-base">
            <Code className="size-5" />
            SDK Examples
          </h3>
          <h2 className="text-foreground font-weight-display leading-snug md:text-xl">
            Generate presigned URLs with:
          </h2>
        </div>

        <div className="mt-6 gap-6">
          {isMobile ? (
            <Select value={activeTab} onValueChange={setActiveTab}>
              <SelectTrigger className="w-full">
                <SelectValue>{activeTab}</SelectValue>
              </SelectTrigger>
              <SelectContent>
                {frameworks.map((framework) => (
                  <SelectItem key={framework.name} value={framework.name}>
                    {framework.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          ) : (
            <Tabs value={activeTab} onValueChange={setActiveTab}>
              <TabsList className="flex gap-3">
                {frameworks.map((framework) => (
                  <TabsTrigger key={framework.name} value={framework.name}>
                    {framework.name}
                  </TabsTrigger>
                ))}
              </TabsList>
            </Tabs>
          )}

          <div className="mt-4">
            {isLoading ? (
              <Card className="relative overflow-hidden !p-0">
                <CardContent className="!p-0">
                  <div className="flex items-center justify-center py-8">
                    <div className="text-muted-foreground">Loading...</div>
                  </div>
                </CardContent>
              </Card>
            ) : (
              frameworks.map(
                (framework) =>
                  framework.name === activeTab && (
                    <Card
                      key={framework.name}
                      className="relative overflow-hidden !p-0"
                    >
                      <CardContent className="!p-0">
                        <div
                          dangerouslySetInnerHTML={{
                            __html:
                              currentHighlightedCode[framework.name] || '',
                          }}
                          className="overflow-x-auto text-sm [&_pre]:m-0 [&_pre]:bg-transparent [&_pre]:p-4"
                        />
                        <CopyButton
                          text={framework.code}
                          className="absolute top-4 right-4"
                        />
                      </CardContent>
                    </Card>
                  ),
              )
            )}
          </div>
        </div>
      </div>
    </section>
  );
}

interface CopyButtonProps {
  text: string;
  className?: string;
}

function CopyButton({ text, className }: CopyButtonProps) {
  const [copied, setCopied] = useState(false);
  const timeoutRef = useRef<NodeJS.Timeout>(undefined);

  const handleCopy = async () => {
    if (copied) return;

    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);

      clearTimeout(timeoutRef.current);
      timeoutRef.current = setTimeout(() => {
        setCopied(false);
      }, 2000);
    } catch (err) {
      console.error('Failed to copy text: ', err);
    }
  };

  return (
    <Button
      aria-label="Copy code"
      onClick={handleCopy}
      variant="ghost"
      size="icon"
      className={cn(className)}
    >
      <AnimatePresence mode="wait" initial={false}>
        <motion.div
          key={copied ? 'check' : 'copy'}
          initial={{ opacity: 0, y: 2 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: -2 }}
          transition={{ duration: 0.15 }}
        >
          {copied ? <Check className="size-4" /> : <Copy className="size-4" />}
        </motion.div>
      </AnimatePresence>
    </Button>
  );
}
