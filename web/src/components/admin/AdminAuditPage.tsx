import { adminQueries } from '@api/queries/admin'
import {
  type AuditSearch,
  AuditTable,
  auditSearchSchema,
  sinceToIso,
} from '@components/audit/AuditTable'
import { keepPreviousData, useInfiniteQuery } from '@tanstack/react-query'
import { useMemo } from 'react'
import { z } from 'zod'

export const adminAuditSearchSchema = auditSearchSchema.extend({
  tenant: z.string().optional().catch(undefined),
})
export type AdminAuditSearch = z.infer<typeof adminAuditSearchSchema>

export function AdminAuditPage({
  search,
  onSearchChange,
}: {
  search: AdminAuditSearch
  onSearchChange: (next: Partial<AdminAuditSearch>) => void
}) {
  const q = useMemo(
    () => ({
      action: search.action,
      actor: search.actor,
      since: sinceToIso(search.since),
      tenant: search.tenant,
    }),
    [search]
  )
  const list = useInfiniteQuery({ ...adminQueries.audit(q), placeholderData: keepPreviousData })
  const tenants = useInfiniteQuery(adminQueries.tenants())
  const rows = useMemo(() => list.data?.pages.flatMap((p) => p.data) ?? [], [list.data])
  return (
    <AuditTable
      rows={rows}
      loading={list.isPending}
      error={list.isError ? list.error : undefined}
      onRetry={() => void list.refetch()}
      hasMore={!!list.hasNextPage}
      fetchingMore={list.isFetchingNextPage}
      onLoadMore={() => void list.fetchNextPage()}
      search={search}
      onSearchChange={(next: Partial<AuditSearch>) => onSearchChange(next)}
      showTenant
      tenantOptions={
        tenants.data?.pages.flatMap((p) => p.data).map((x) => ({ label: x.name, value: x.id })) ??
        []
      }
      tenantFilter={search.tenant}
      onTenantFilter={(v) => onSearchChange({ tenant: v })}
    />
  )
}
