/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** "1" enables the throwaway mock backend (see src/mock/). */
  readonly VITE_MOCK?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
