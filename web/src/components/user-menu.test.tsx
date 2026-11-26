import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithQuery } from "@/test/render";
import { server } from "@/test/server";

import { UserMenu } from "./user-menu";

const router = vi.hoisted(() => ({ replace: vi.fn(), refresh: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => router }));
const toast = vi.hoisted(() => ({ error: vi.fn() }));
vi.mock("sonner", () => ({ toast }));

async function openMenu(user: ReturnType<typeof userEvent.setup>) {
  renderWithQuery(<UserMenu fullName="Ada Lovelace" email="ada@example.com" />);
  await user.click(screen.getByRole("button", { name: "Account menu" }));
}

describe("UserMenu", () => {
  beforeEach(() => {
    router.replace.mockClear();
    router.refresh.mockClear();
    toast.error.mockClear();
  });

  it("shows the user and signs out to /login", async () => {
    let called = false;
    server.use(
      http.post("*/api/v1/auth/logout", () => {
        called = true;
        return new HttpResponse(null, { status: 204 });
      }),
    );
    const user = userEvent.setup();
    await openMenu(user);

    expect(screen.getByText("Ada Lovelace")).toBeInTheDocument();
    expect(screen.getByText("ada@example.com")).toBeInTheDocument();
    await user.click(screen.getByRole("menuitem", { name: "Sign out" }));

    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/login"));
    expect(called).toBe(true);
    expect(router.refresh).toHaveBeenCalled();
  });

  it("stays and shows a toast when sign-out fails", async () => {
    server.use(http.post("*/api/v1/auth/logout", () => new HttpResponse(null, { status: 500 })));
    const user = userEvent.setup();
    await openMenu(user);

    await user.click(screen.getByRole("menuitem", { name: "Sign out" }));

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith("Could not sign out. Try again."));
    expect(router.replace).not.toHaveBeenCalled();
  });
});
