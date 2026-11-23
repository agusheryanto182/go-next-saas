import createClient from "openapi-fetch";

import type { paths } from "./schema";

/**
 * Typed API client for the browser. Requests go to the same origin (`/api`), so the session cookie
 * is sent automatically and stays first-party (ADR 0006, ADR 0018). Paths, parameters and responses
 * are typed from openapi/openapi.yaml (`npm run gen:api`).
 */
export function createBrowserClient() {
  // Absolute URL on the current origin: `fetch` outside a real browser (tests) rejects relative URLs.
  const baseUrl = new URL("/api", window.location.origin).toString();
  return createClient<paths>({ baseUrl, credentials: "same-origin" });
}
