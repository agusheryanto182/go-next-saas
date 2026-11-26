import { Building2 } from "lucide-react";
import Link from "next/link";
import { redirect } from "next/navigation";
import { Suspense, type ReactNode } from "react";

import { UserMenu } from "@/components/user-menu";
import { createServerClient } from "@/lib/api/server";

/** Signed-in area. The API decides who is signed in; a 401 sends the visitor to the sign-in page. */
export default function AppLayout({ children }: { children: ReactNode }) {
  // Everything below reads the request cookie, so it renders behind a Suspense boundary.
  return (
    <Suspense fallback={<div className="min-h-svh" aria-busy="true" />}>
      <AppShell>{children}</AppShell>
    </Suspense>
  );
}

async function AppShell({ children }: { children: ReactNode }) {
  const api = await createServerClient();
  const { data: user, response } = await api.GET("/v1/me");
  if (response.status === 401) redirect("/login");
  if (!user) throw new Error(`GET /v1/me failed with status ${response.status}`);

  return (
    <div className="flex min-h-svh flex-col">
      <header className="flex h-14 items-center justify-between gap-4 border-b px-4">
        <Link href="/orgs" className="flex min-w-0 items-center gap-2 font-semibold">
          <span className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-primary text-primary-foreground">
            <Building2 className="size-4" aria-hidden />
          </span>
          <span className="truncate">go-next-saas</span>
        </Link>
        <UserMenu fullName={user.fullName} email={user.email} />
      </header>
      <div className="mx-auto w-full max-w-3xl flex-1 px-4 py-8">{children}</div>
    </div>
  );
}
