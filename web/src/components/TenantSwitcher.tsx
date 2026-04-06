import { useEffect, useState } from "react";
import { useAuth } from "@/hooks/useAuth";
import { listTenantsAdmin } from "@/api/client";
import type { Tenant } from "@/api/types";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";

export function TenantSwitcher() {
  const { user, selectTenant } = useAuth();
  const [tenants, setTenants] = useState<Tenant[]>([]);

  useEffect(() => {
    if (user?.is_root) {
      listTenantsAdmin().then((t) => setTenants(t || [])).catch(() => {});
    }
  }, [user?.is_root]);

  if (!user?.is_root || tenants.length === 0) return null;

  return (
    <Select
      value={user.tenant_id ?? undefined}
      onValueChange={(v) => selectTenant(v)}
    >
      <SelectTrigger className="h-7 text-xs border-border">
        <SelectValue placeholder="Switch tenant" />
      </SelectTrigger>
      <SelectContent>
        {tenants.map((t) => (
          <SelectItem key={t.id} value={t.id} className="text-xs">
            {t.name}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
