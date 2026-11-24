import { expect, test } from "@playwright/test";

// Smoke test: the stack starts and its parts are wired together.
// Replaced by user journeys from milestone 1 on.

const apiURL = process.env.E2E_API_URL ?? "http://localhost:8080";

test("the landing page renders", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "One workspace for every team" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Sign in" })).toBeVisible();
});

test("the API is ready: migrations ran and the database is reachable", async ({ request }) => {
  const res = await request.get(`${apiURL}/readyz`);
  expect(res.status()).toBe(200);
  expect(await res.json()).toEqual({ status: "ready" });
});
