import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";

import { server } from "@/test/server";

import { createBrowserClient } from "./client";

describe("createBrowserClient", () => {
  it("calls same-origin /api with typed paths and query parameters", async () => {
    let url: URL | undefined;
    server.use(
      http.get("*/api/v1/orgs/:slug/members", ({ request }) => {
        url = new URL(request.url);
        return HttpResponse.json({ items: [], page: 1, size: 50, totalItems: 0, totalPages: 0 });
      }),
    );

    const { data, error } = await createBrowserClient().GET("/v1/orgs/{slug}/members", {
      params: { path: { slug: "acme-corp" }, query: { role: "admin", page: 1, size: 50 } },
    });

    expect(error).toBeUndefined();
    expect(data?.totalItems).toBe(0);
    expect(url?.pathname).toBe("/api/v1/orgs/acme-corp/members");
    expect(url?.searchParams.get("role")).toBe("admin");
  });

  it("returns problem details as a typed error", async () => {
    server.use(
      http.get("*/api/v1/me", () =>
        HttpResponse.json(
          { type: "about:blank", title: "Unauthorized", status: 401, detail: "Sign in first." },
          { status: 401, headers: { "Content-Type": "application/problem+json" } },
        ),
      ),
    );

    const { data, error } = await createBrowserClient().GET("/v1/me");

    expect(data).toBeUndefined();
    expect(error?.detail).toBe("Sign in first.");
  });
});
