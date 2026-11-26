import type { Metadata } from "next";
import { Suspense } from "react";

import { AuthCard } from "@/components/auth/auth-card";
import { SignInForm } from "@/components/auth/sign-in-form";

export const metadata: Metadata = { title: "Sign in" };

export default function LoginPage({ searchParams }: PageProps<"/login">) {
  return (
    <AuthCard title="Sign in" description="Sign in to your workspace.">
      {/* searchParams is request data; reading it behind Suspense keeps the card itself static. */}
      <Suspense fallback={<SignInForm />}>
        <SignInFormWithRedirect searchParams={searchParams} />
      </Suspense>
    </AuthCard>
  );
}

async function SignInFormWithRedirect({
  searchParams,
}: {
  searchParams: PageProps<"/login">["searchParams"];
}) {
  const { redirect } = await searchParams;
  return <SignInForm redirect={typeof redirect === "string" ? redirect : null} />;
}
