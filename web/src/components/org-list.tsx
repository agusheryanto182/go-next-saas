import Link from "next/link";

import { Badge } from "@/components/ui/badge";
import type { components } from "@/lib/api/schema";

type UserOrganization = components["schemas"]["UserOrganization"];

export function OrgList({ items }: { items: UserOrganization[] }) {
  return (
    <section aria-labelledby="orgs-heading" className="flex flex-col gap-4">
      <h1 id="orgs-heading" className="text-2xl font-semibold tracking-tight">
        Organizations
      </h1>
      {items.length === 0 ? (
        <div className="rounded-lg border border-dashed p-8 text-center">
          <p className="font-medium">You do not belong to any organization yet</p>
          <p className="mt-1 text-sm text-muted-foreground">Ask an administrator to invite you.</p>
        </div>
      ) : (
        <ul className="divide-y rounded-lg border">
          {items.map((org) => (
            <li key={org.id} className="flex items-center justify-between gap-3 p-4">
              <div className="min-w-0">
                <Link
                  href={`/orgs/${org.slug}`}
                  className="block truncate font-medium hover:underline"
                >
                  {org.name}
                </Link>
                <p className="truncate text-sm text-muted-foreground">
                  {org.slug} · {org.role}
                </p>
              </div>
              {org.status === "deactivated" && <Badge variant="outline">Deactivated</Badge>}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
