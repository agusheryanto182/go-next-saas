import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";

import { SignInForm } from "./sign-in-form";

const router = vi.hoisted(() => ({ replace: vi.fn(), refresh: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => router }));

const LOGIN = "*/api/v1/auth/login";

function problem(status: number, detail: string, headers?: Record<string, string>) {
  return HttpResponse.json(
    { type: "about:blank", title: "Error", status, detail },
    { status, headers: { "Content-Type": "application/problem+json", ...headers } },
  );
}

async function fillAndSubmit(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText("Email"), "ada@example.com");
  await user.type(screen.getByLabelText("Password"), "secret");
  await user.click(screen.getByRole("button", { name: "Sign in" }));
}

describe("SignInForm", () => {
  beforeEach(() => {
    router.replace.mockClear();
    router.refresh.mockClear();
  });

  it("shows field errors and focuses the first invalid field", async () => {
    const user = userEvent.setup();
    renderWithQuery(<SignInForm />);

    await user.click(screen.getByRole("button", { name: "Sign in" }));

    const email = screen.getByLabelText("Email");
    expect(await screen.findByText("Enter a valid email address")).toBeInTheDocument();
    expect(screen.getByText("Enter your password")).toBeInTheDocument();
    expect(email).toHaveAttribute("aria-invalid", "true");
    expect(email).toHaveAccessibleDescription("Enter a valid email address");
    expect(email).toHaveFocus();
  });

  it("focuses the password when only it is invalid", async () => {
    const user = userEvent.setup();
    renderWithQuery(<SignInForm />);

    await user.type(screen.getByLabelText("Email"), "ada@example.com");
    await user.click(screen.getByRole("button", { name: "Sign in" }));

    expect(await screen.findByText("Enter your password")).toBeInTheDocument();
    expect(screen.getByLabelText("Password")).toHaveFocus();
  });

  it("rejects a malformed email without calling the API", async () => {
    const user = userEvent.setup();
    renderWithQuery(<SignInForm />);

    await user.type(screen.getByLabelText("Email"), "not-an-email");
    await user.type(screen.getByLabelText("Password"), "secret");
    await user.click(screen.getByRole("button", { name: "Sign in" }));

    expect(await screen.findByText("Enter a valid email address")).toBeInTheDocument();
  });

  it("shows the generic message for every 401", async () => {
    server.use(http.post(LOGIN, () => problem(401, "Invalid email or password.")));
    const user = userEvent.setup();
    renderWithQuery(<SignInForm />);

    await fillAndSubmit(user);

    expect(await screen.findByRole("alert")).toHaveTextContent("Invalid email or password.");
    expect(router.replace).not.toHaveBeenCalled();
  });

  it("says when to try again after a 429", async () => {
    server.use(http.post(LOGIN, () => problem(429, "slow down", { "Retry-After": "120" })));
    const user = userEvent.setup();
    renderWithQuery(<SignInForm />);

    await fillAndSubmit(user);

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Too many failed attempts. Try again in 2 minutes.",
    );
  });

  it("falls back to a generic message without Retry-After", async () => {
    server.use(http.post(LOGIN, () => problem(429, "slow down")));
    const user = userEvent.setup();
    renderWithQuery(<SignInForm />);

    await fillAndSubmit(user);

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Too many failed attempts. Try again later.",
    );
  });

  it("shows a generic message when the request fails", async () => {
    server.use(http.post(LOGIN, () => HttpResponse.error()));
    const user = userEvent.setup();
    renderWithQuery(<SignInForm />);

    await fillAndSubmit(user);

    expect(await screen.findByRole("alert")).toHaveTextContent("Something went wrong. Try again.");
  });

  it("disables the button while the request is pending", async () => {
    let release!: () => void;
    const gate = new Promise<void>((resolve) => (release = resolve));
    server.use(
      http.post(LOGIN, async () => {
        await gate;
        return new HttpResponse(null, { status: 204 });
      }),
    );
    const user = userEvent.setup();
    renderWithQuery(<SignInForm />);

    await fillAndSubmit(user);

    const button = screen.getByRole("button", { name: "Sign in" });
    await waitFor(() => expect(button).toBeDisabled());
    release();
    await waitFor(() => expect(router.replace).toHaveBeenCalled());
  });

  it("sends the credentials and goes to /orgs by default", async () => {
    let body: unknown;
    server.use(
      http.post(LOGIN, async ({ request }) => {
        body = await request.json();
        return new HttpResponse(null, { status: 204 });
      }),
    );
    const user = userEvent.setup();
    renderWithQuery(<SignInForm />);

    await fillAndSubmit(user);

    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/orgs"));
    expect(router.refresh).toHaveBeenCalled();
    expect(body).toEqual({ email: "ada@example.com", password: "secret" });
  });

  it("goes to a same-origin redirect", async () => {
    server.use(http.post(LOGIN, () => new HttpResponse(null, { status: 204 })));
    const user = userEvent.setup();
    renderWithQuery(<SignInForm redirect="/orgs/acme" />);

    await fillAndSubmit(user);

    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/orgs/acme"));
  });

  it("ignores an external redirect", async () => {
    server.use(http.post(LOGIN, () => new HttpResponse(null, { status: 204 })));
    const user = userEvent.setup();
    renderWithQuery(<SignInForm redirect="https://evil.example" />);

    await fillAndSubmit(user);

    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/orgs"));
  });
});
