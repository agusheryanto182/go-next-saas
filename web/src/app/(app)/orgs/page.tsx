import type { Metadata } from "next";
import { redirect } from "next/navigation";

import { OrgList } from "@/components/org-list";
import { createServerClient } from "@/lib/api/server";

export const metadata: Metadata = { title: "Organizations" };

export default async function OrgsPage() {
  const api = await createServerClient();
  const { data, response } = await api.GET("/v1/me/organizations");
  if (response.status === 401) redirect("/login?redirect=/orgs");
  if (!data) throw new Error(`GET /v1/me/organizations failed with status ${response.status}`);
  return <OrgList items={data.items} />;
}
