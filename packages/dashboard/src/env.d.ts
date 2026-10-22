interface ImportMetaEnv {
  /**
   * The API's origin when the dashboard runs on a host of its own, like
   * https://api.cerebellumlabs.dev. Empty when both share an origin, as in
   * development, where Vite proxies /api to the server.
   */
  readonly VITE_API_URL?: string
  /** "true" runs the dashboard on the in-memory mock with a simulated robot. */
  readonly VITE_API_MOCK?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
