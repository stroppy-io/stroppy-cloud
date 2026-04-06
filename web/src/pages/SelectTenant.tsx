import { useNavigate } from "react-router-dom";
import { useAuth } from "@/hooks/useAuth";
import { Activity, Building2 } from "lucide-react";

export function SelectTenant() {
  const { user, selectTenant } = useAuth();
  const navigate = useNavigate();

  if (!user) return null;

  async function handleSelect(tenantId: string) {
    await selectTenant(tenantId);
    navigate("/", { replace: true });
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <div className="w-full max-w-md space-y-6 rounded-lg border border-border bg-card p-8">
        <div className="flex flex-col items-center gap-2">
          <Activity className="h-8 w-8 text-primary" />
          <h1 className="text-lg font-semibold tracking-tight">
            Select Tenant
          </h1>
          <p className="text-sm text-muted-foreground">
            Choose a workspace to continue
          </p>
        </div>

        {(user.tenants || []).length === 0 ? (
          <div className="text-center text-sm text-muted-foreground py-8">
            No tenants assigned. Contact admin.
          </div>
        ) : (
          <div className="space-y-2">
            {(user.tenants || []).map((t) => (
              <button
                key={t.id}
                onClick={() => handleSelect(t.id)}
                className="w-full flex items-center gap-3 rounded-md border border-border p-4 text-left transition-colors hover:bg-muted/50"
              >
                <Building2 className="h-5 w-5 text-muted-foreground shrink-0" />
                <div className="flex-1 min-w-0">
                  <div className="text-sm font-medium truncate">
                    {t.tenant_name}
                  </div>
                  <div className="text-xs text-muted-foreground">{t.role}</div>
                </div>
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
