"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation } from "@tanstack/react-query";
import { Loader2 } from "lucide-react";
import { useRouter } from "next/navigation";
import { useForm } from "react-hook-form";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { createBrowserClient } from "@/lib/api/client";
import { afterSignInPath } from "@/lib/auth/redirect";
import { signInSchema, type SignInValues } from "@/lib/auth/schemas";
import { SignInError, signInErrorMessage } from "@/lib/auth/sign-in-error";

export function SignInForm({ redirect }: { redirect?: string | null }) {
  const router = useRouter();
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<SignInValues>({
    resolver: zodResolver(signInSchema),
    defaultValues: { email: "", password: "" },
    shouldFocusError: true,
  });

  const signIn = useMutation({
    mutationFn: async (values: SignInValues) => {
      let result;
      try {
        result = await createBrowserClient().POST("/v1/auth/login", { body: values });
      } catch {
        throw new SignInError(signInErrorMessage(undefined, undefined));
      }
      if (!result.response.ok) {
        throw new SignInError(signInErrorMessage(result.response, result.error));
      }
    },
    onSuccess: () => {
      router.replace(afterSignInPath(redirect));
      router.refresh();
    },
  });

  return (
    <form
      onSubmit={handleSubmit((values) => signIn.mutate(values))}
      noValidate
      className="flex flex-col gap-4"
    >
      {signIn.isError && (
        <Alert variant="destructive">
          <AlertDescription>{signIn.error.message}</AlertDescription>
        </Alert>
      )}
      <div className="flex flex-col gap-2">
        <Label htmlFor="email">Email</Label>
        <Input
          id="email"
          type="email"
          autoComplete="username"
          aria-invalid={errors.email ? true : undefined}
          aria-describedby={errors.email ? "email-error" : undefined}
          {...register("email")}
        />
        {errors.email && (
          <p id="email-error" className="text-sm text-destructive">
            {errors.email.message}
          </p>
        )}
      </div>
      <div className="flex flex-col gap-2">
        <Label htmlFor="password">Password</Label>
        <Input
          id="password"
          type="password"
          autoComplete="current-password"
          aria-invalid={errors.password ? true : undefined}
          aria-describedby={errors.password ? "password-error" : undefined}
          {...register("password")}
        />
        {errors.password && (
          <p id="password-error" className="text-sm text-destructive">
            {errors.password.message}
          </p>
        )}
      </div>
      <Button type="submit" disabled={signIn.isPending} className="w-full">
        {signIn.isPending && <Loader2 className="animate-spin" aria-hidden />}
        Sign in
      </Button>
    </form>
  );
}
