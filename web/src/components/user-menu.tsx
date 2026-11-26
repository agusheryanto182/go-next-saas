"use client";

import { useMutation } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { toast } from "sonner";

import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { createBrowserClient } from "@/lib/api/client";

function initials(name: string, email: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  const letters =
    parts.length > 1 ? parts[0][0] + parts[parts.length - 1][0] : (parts[0] ?? email).slice(0, 2);
  return letters.toUpperCase();
}

export function UserMenu({ fullName, email }: { fullName: string; email: string }) {
  const router = useRouter();
  const signOut = useMutation({
    mutationFn: async () => {
      const { response } = await createBrowserClient().POST("/v1/auth/logout");
      if (!response.ok) throw new Error(`logout failed: ${response.status}`);
    },
    onSuccess: () => {
      router.replace("/login");
      router.refresh();
    },
    onError: () => toast.error("Could not sign out. Try again."),
  });

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label="Account menu"
        className="rounded-full outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
      >
        <Avatar>
          <AvatarFallback>{initials(fullName, email)}</AvatarFallback>
        </Avatar>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-60 max-w-[calc(100vw-2rem)]">
        <DropdownMenuLabel className="flex flex-col font-normal">
          <span className="truncate font-medium">{fullName}</span>
          <span className="truncate text-muted-foreground">{email}</span>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuItem disabled={signOut.isPending} onSelect={() => signOut.mutate()}>
          Sign out
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
