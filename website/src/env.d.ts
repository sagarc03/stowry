/// <reference types="astro/client" />

interface ImportMetaEnv {
  readonly PUBLIC_STOWRY_VERSION: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
