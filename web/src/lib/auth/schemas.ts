import { z } from "zod";

/** Sign-in rules from the contract (LoginRequest in openapi/openapi.yaml). Passwords are not checked against password rules. */
export const signInSchema = z.object({
  email: z
    .string()
    .trim()
    .max(255, "Email must be 255 characters or fewer")
    .pipe(z.email("Enter a valid email address")),
  password: z
    .string()
    .min(1, "Enter your password")
    .max(128, "Password must be 128 characters or fewer"),
});

export type SignInValues = z.infer<typeof signInSchema>;
