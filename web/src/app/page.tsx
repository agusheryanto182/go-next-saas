import { Building2 } from "lucide-react";

import { Button } from "@/components/ui/button";

/** Public landing page. A placeholder until milestone 4. */
export default function HomePage() {
  return (
    <main className="mx-auto flex min-h-svh max-w-2xl flex-col items-center justify-center gap-6 px-4 text-center">
      <span className="flex size-12 items-center justify-center rounded-xl bg-primary text-primary-foreground">
        <Building2 className="size-6" aria-hidden />
      </span>
      <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">
        One workspace for every team
      </h1>
      <p className="text-muted-foreground">
        Invite your people, see who signed in and when, and keep each organization&apos;s data to
        itself.
      </p>
      <Button asChild size="lg">
        <a href="/login">Sign in</a>
      </Button>
    </main>
  );
}
