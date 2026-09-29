import { tenantQueries } from '@api/queries/tenants'
import { type AuditSearch, AuditTable, sinceToIso } from '@components/audit/AuditTable'
import { useTenant } from '@hooks/useTenant'
import { keepPreviousData, useInfiniteQuery } from '@tanstack/react-query'
import { useMemo } from 'react'

export function AuditPage({
  search,
  onSearchChange,
}: {
  search: AuditSearch
  onSearchChange: (next: Partial<AuditSearch>) => void
}) {
  const { slug } = useTenant()
  const q = useMemo(
    () => ({ action: search.action, actor: search.actor, since: sinceToIso(search.since) }),
    [search]
  )
  const list = useInfiniteQuery({
    ...tenantQueries.auditPages(slug, q),
    placeholderData: keepPreviousData,
  })
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
      onSearchChange={onSearchChange}
      tenantSlug={slug}
    />
  )
}
