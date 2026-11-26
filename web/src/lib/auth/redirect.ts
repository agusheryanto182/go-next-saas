const DEFAULT_PATH = "/orgs";
const BASE = "http://x.invalid";

/**
 * Where to go after signing in. Only same-origin paths are accepted, so a crafted
 * `?redirect=` link cannot send someone to another site (open redirect).
 */
export function afterSignInPath(redirect: string | null | undefined): string {
  if (!redirect || !redirect.startsWith("/") || redirect.startsWith("//")) return DEFAULT_PATH;
  // Backslashes are read as slashes by browsers; control characters can hide a scheme or host.
  if (/[\\\u0000-\u001f\u007f]/.test(redirect)) return DEFAULT_PATH;
  let url: URL;
  try {
    url = new URL(redirect, BASE);
  } catch {
    return DEFAULT_PATH;
  }
  if (url.origin !== BASE) return DEFAULT_PATH;
  if (url.pathname === "/login" || url.pathname.startsWith("/login/")) return DEFAULT_PATH;
  return redirect;
}
