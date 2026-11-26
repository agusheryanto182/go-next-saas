import { NextResponse, type NextRequest } from "next/server";

export const SESSION_COOKIE = "__Host-session";

/**
 * Sends visitors without a session cookie to the sign-in page. It only checks that the cookie
 * exists; whether the session is valid and what the user may do is decided by the API.
 */
export function proxy(request: NextRequest) {
  if (request.cookies.has(SESSION_COOKIE)) return NextResponse.next();
  const { pathname, search } = request.nextUrl;
  return NextResponse.redirect(
    new URL(`/login?redirect=${encodeURIComponent(pathname + search)}`, request.url),
  );
}

export const config = {
  matcher: ["/orgs", "/orgs/:path*"],
};
