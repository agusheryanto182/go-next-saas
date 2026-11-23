import { http, HttpResponse } from "msw";
import createClient from "openapi-fetch";
import { describe, expect, it } from "vitest";

import { server } from "@/test/server";

import { forwardedHeaders } from "./server";

describe("forwardedHeaders", () => {
  it("forwards the cookie, client IP and user agent", () => {
    expect(
      forwardedHeaders({
        cookie: "__Host-session=abc",
        forwardedFor: "203.0.113.7",
        realIp: "10.0.0.1",
        userAgent: "Firefox",
      }),
    ).toEqual({
      cookie: "__Host-session=abc",
      "x-forwarded-for": "203.0.113.7",
      "user-agent": "Firefox",
    });
  });

  it("falls back to X-Real-IP and omits what is missing", () => {
    expect(
      forwardedHeaders({
        cookie: "",
        forwardedFor: null,
        realIp: "198.51.100.2",
        userAgent: null,
      }),
    ).toEqual({
      "x-forwarded-for": "198.51.100.2",
    });
  });

  it("sends the forwarded headers on API calls", async () => {
    let received: Headers | undefined;
    server.use(
      http.get("http://api.test/api/v1/me", ({ request }) => {
        received = request.headers;
        return HttpResponse.json({ ok: true });
      }),
    );

    const client = createClient<Record<string, never>>({
      baseUrl: "http://api.test/api",
      headers: forwardedHeaders({
        cookie: "__Host-session=abc",
        forwardedFor: "203.0.113.7",
        realIp: null,
        userAgent: null,
      }),
    });
    // The path is not in the empty test schema; the cast keeps the test about headers only.
    await (client as unknown as { GET: (path: string) => Promise<unknown> }).GET("/v1/me");

    expect(received?.get("cookie")).toBe("__Host-session=abc");
    expect(received?.get("x-forwarded-for")).toBe("203.0.113.7");
  });
});
