import { redirect } from "next/navigation";

/**
 * Root route — immediately redirects to /login.
 * Auth protection is handled in (dashboard)/layout.tsx.
 */
export default function Home() {
  redirect("/login");
}
