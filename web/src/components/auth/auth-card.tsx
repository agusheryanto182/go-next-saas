import { Building2 } from "lucide-react";
import type { ReactNode } from "react";

import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

interface AuthCardProps {
  title: string;
  description?: string;
  footer?: ReactNode;
  children: ReactNode;
}

/** Centered card shared by the sign-in and password pages. */
export function AuthCard({ title, description, footer, children }: AuthCardProps) {
  return (
    <main className="flex min-h-svh flex-col items-center justify-center gap-6 bg-muted/40 px-4 py-10">
      <span className="flex size-12 items-center justify-center rounded-xl bg-primary text-primary-foreground">
        <Building2 className="size-6" aria-hidden />
      </span>
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle className="text-xl">
            <h1>{title}</h1>
          </CardTitle>
          {description && <CardDescription>{description}</CardDescription>}
        </CardHeader>
        <CardContent>{children}</CardContent>
        {footer && (
          <CardFooter className="justify-center text-sm text-muted-foreground">{footer}</CardFooter>
        )}
      </Card>
    </main>
  );
}
