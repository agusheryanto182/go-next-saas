/** Error raised by the sign-in request; `message` is safe to show to the user. */
export class SignInError extends Error {}

const GENERIC = "Something went wrong. Try again.";

/** Maps a failed login response (or a network failure) to the text the user sees. */
export function signInErrorMessage(response: Response | undefined, problem: unknown): string {
  if (!response) return GENERIC;
  if (response.status === 401) return "Invalid email or password.";
  if (response.status === 429) return tooManyAttempts(response.headers.get("Retry-After"));
  const detail =
    problem && typeof problem === "object" && "detail" in problem ? problem.detail : undefined;
  return typeof detail === "string" && detail ? detail : GENERIC;
}

function tooManyAttempts(retryAfter: string | null): string {
  const seconds = retryAfter ? Number.parseInt(retryAfter, 10) : Number.NaN;
  if (!Number.isFinite(seconds) || seconds <= 0) {
    return "Too many failed attempts. Try again later.";
  }
  if (seconds < 60) {
    return `Too many failed attempts. Try again in ${seconds} ${seconds === 1 ? "second" : "seconds"}.`;
  }
  const minutes = Math.ceil(seconds / 60);
  return `Too many failed attempts. Try again in ${minutes} ${minutes === 1 ? "minute" : "minutes"}.`;
}
