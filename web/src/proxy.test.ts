import { NextRequest } from "next/server";
import { describe, expect, it } from "vitest";

import { config, proxy } from "./proxy";

describe("proxy", () => {
  it("redirects to /login with the encoded path when there is no session cookie", () => {
    const res = proxy(new NextRequest("http://localhost:3000/orgs/acme?tab=1"));

    expect(res.status).toBe(307);
    expect(res.headers.get("location")).toBe(
      "http://localhost:3000/login?redirect=%2Forgs%2Facme%3Ftab%3D1",
    );
  });

  it("passes through when the session cookie exists", () => {
    const res = proxy(
      new NextRequest("http://localhost:3000/orgs", {
        headers: { cookie: "__Host-session=abc" },
      }),
    );

    expect(res.headers.get("location")).toBeNull();
    expect(res.headers.get("x-middleware-next")).toBe("1");
  });

  it("covers the protected routes only", () => {
    expect(config.matcher).toEqual(["/orgs", "/orgs/:path*"]);
  });
});
