import { cookies, headers } from "next/headers";
import createClient from "openapi-fetch";

import type { paths } from "./schema";

interface IncomingRequest {
  cookie: string;
  forwardedFor: string | null;
  realIp: string | null;
  userAgent: string | null;
}

/**
 * Headers the API needs when Next.js calls it on behalf of a browser: the session cookie for
 * authentication, and the client's IP and user agent so login history records the real user,
 * not this server.
 */
export function forwardedHeaders(req: IncomingRequest): Record<string, string> {
  const out: Record<string, string> = {};
  if (req.cookie) out.cookie = req.cookie;
  const ip = req.forwardedFor ?? req.realIp;
  if (ip) out["x-forwarded-for"] = ip;
  if (req.userAgent) out["user-agent"] = req.userAgent;
  return out;
}

/**
 * Typed API client for server components. It calls the API directly (API_INTERNAL_URL), forwarding
 * the incoming request's cookie and client details. Authorization still happens in the API.
 */
export async function createServerClient() {
  const [cookieStore, headerStore] = await Promise.all([cookies(), headers()]);
  return createClient<paths>({
    baseUrl: `${apiInternalUrl()}/api`,
    headers: forwardedHeaders({
      cookie: cookieStore.toString(),
      forwardedFor: headerStore.get("x-forwarded-for"),
      realIp: headerStore.get("x-real-ip"),
      userAgent: headerStore.get("user-agent"),
    }),
    cache: "no-store",
  });
}

function apiInternalUrl(): string {
  const url = process.env.API_INTERNAL_URL;
  if (!url) throw new Error("API_INTERNAL_URL is not set");
  return url.replace(/\/+$/, "");
}
