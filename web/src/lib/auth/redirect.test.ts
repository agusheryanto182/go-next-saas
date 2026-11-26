import { describe, expect, it } from "vitest";

import { afterSignInPath } from "./redirect";

describe("afterSignInPath", () => {
  it("keeps a same-origin path with query", () => {
    expect(afterSignInPath("/orgs/acme?tab=1")).toBe("/orgs/acme?tab=1");
  });

  it.each([
    ["external URL", "https://evil.example"],
    ["protocol-relative", "//evil.example"],
    ["backslash", "/\\evil.example"],
    ["javascript scheme", "javascript:alert(1)"],
    ["control characters", "/\t/evil.example"],
    ["relative path", "orgs"],
    ["login itself", "/login?redirect=/orgs"],
    ["a path under login", "/login/"],
    ["empty", ""],
    ["null", null],
    ["undefined", undefined],
  ])("falls back to /orgs for %s", (_name, value) => {
    expect(afterSignInPath(value)).toBe("/orgs");
  });
});
