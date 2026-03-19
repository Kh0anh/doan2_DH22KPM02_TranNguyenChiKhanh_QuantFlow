/**
 * [3.1.1] Login Page — form + brute-force error handling.
 * Full implementation: Task 3.1.1
 */
export default function LoginPage() {
  return (
    <div className="w-full max-w-sm space-y-6 p-6">
      <div className="space-y-2 text-center">
        <h1 className="text-2xl font-semibold text-foreground">QuantFlow</h1>
        <p className="text-sm text-muted-foreground">Đăng nhập vào hệ thống</p>
      </div>
      {/* TODO [3.1.1]: Implement LoginForm component with brute-force protection */}
      <div className="rounded-md border border-dashed border-border p-8 text-center text-sm text-muted-foreground">
        Login form — Task 3.1.1
      </div>
    </div>
  );
}
