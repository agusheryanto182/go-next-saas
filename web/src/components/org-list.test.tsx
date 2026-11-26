import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { OrgList } from "./org-list";

describe("OrgList", () => {
  it("lists organizations with role and a deactivated badge", () => {
    render(
      <OrgList
        items={[
          { id: "1", name: "Acme", slug: "acme", role: "owner", status: "active" },
          { id: "2", name: "Globex", slug: "globex", role: "member", status: "deactivated" },
        ]}
      />,
    );

    expect(screen.getByRole("heading", { name: "Organizations" })).toBeInTheDocument();
    const items = screen.getAllByRole("listitem");
    expect(items).toHaveLength(2);
    expect(within(items[0]).getByRole("link", { name: "Acme" })).toHaveAttribute(
      "href",
      "/orgs/acme",
    );
    expect(items[0]).toHaveTextContent("owner");
    expect(within(items[0]).queryByText("Deactivated")).not.toBeInTheDocument();
    expect(within(items[1]).getByText("Deactivated")).toBeInTheDocument();
  });

  it("shows the empty state", () => {
    render(<OrgList items={[]} />);

    expect(screen.getByText("You do not belong to any organization yet")).toBeInTheDocument();
    expect(screen.queryByRole("list")).not.toBeInTheDocument();
  });
});
