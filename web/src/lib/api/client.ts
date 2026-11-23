import createClient from "openapi-fetch";

/**
 * Typed API client for the browser. Requests go to the same origin (`/api`), so the session cookie
 * is sent automatically and stays first-party (ADR 0006, ADR 0018).
 *
 * `Paths` comes from the types generated from openapi/openapi.yaml.
 */
export function createBrowserClient<Paths extends object>() {
  return createClient<Paths>({ baseUrl: "/api", credentials: "same-origin" });
}
