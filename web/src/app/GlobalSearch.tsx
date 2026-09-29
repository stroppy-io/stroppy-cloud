import { api } from '@api/client'
import { css } from '@emotion/css'
import type { GrafanaTheme2, IconName } from '@grafana/data'
import { Icon, Input, Menu, Text, useStyles2 } from '@grafana/ui'
import { useTenant } from '@hooks/useTenant'
import { useDebouncedCallback } from '@tanstack/react-pacer'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { forwardRef, useEffect, useImperativeHandle, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

interface Hit {
  group: 'runs' | 'tests' | 'databases' | 'workloads' | 'suites'
  id: string
  title: string
  subtitle?: string
  status?: string
  to: string
  params: Record<string, string>
}

const GROUP_ICON: Record<Hit['group'], IconName> = {
  runs: 'play',
  tests: 'vial',
  databases: 'database',
  workloads: 'bolt',
  suites: 'layer-group',
}

async function searchAll(slug: string, q: string): Promise<Hit[]> {
  const limit = 5
  const [runs, tests, dbs, wls, suites] = await Promise.all([
    api.GET('/api/v1/t/{slug}/runs', { params: { path: { slug }, query: { search: q, limit } } }),
    api.GET('/api/v1/t/{slug}/tests', { params: { path: { slug }, query: { search: q, limit } } }),
    api.GET('/api/v1/t/{slug}/databases', {
      params: { path: { slug }, query: { search: q, limit } },
    }),
    api.GET('/api/v1/t/{slug}/workloads', {
      params: { path: { slug }, query: { search: q, limit } },
    }),
    api.GET('/api/v1/t/{slug}/suites', { params: { path: { slug }, query: { search: q, limit } } }),
  ])
  const hits: Hit[] = []
  for (const r of runs.data?.data ?? [])
    hits.push({
      group: 'runs',
      id: r.id,
      title: r.name,
      subtitle: [r.summary?.db_kind, r.summary?.db_version].filter(Boolean).join(' '),
      status: r.status,
      to: '/t/$slug/runs/$id',
      params: { slug, id: r.id },
    })
  for (const t of tests.data?.data ?? [])
    hits.push({
      group: 'tests',
      id: t.id,
      title: t.name,
      subtitle: t.description,
      status: t.status,
      to: '/t/$slug/library/tests/$id',
      params: { slug, id: t.id },
    })
  for (const d of dbs.data?.data ?? [])
    hits.push({
      group: 'databases',
      id: d.id,
      title: d.name,
      subtitle: `${d.kind} ${d.version}`,
      to: '/t/$slug/library/databases/$id',
      params: { slug, id: d.id },
    })
  for (const w of wls.data?.data ?? [])
    hits.push({
      group: 'workloads',
      id: w.id,
      title: w.name,
      subtitle: `${w.protocol} · stroppy ${w.stroppy_version}`,
      to: '/t/$slug/library/workloads/$id',
      params: { slug, id: w.id },
    })
  for (const s of suites.data?.data ?? [])
    hits.push({
      group: 'suites',
      id: s.id,
      title: s.name,
      subtitle: s.description,
      to: '/t/$slug/suites/$id',
      params: { slug, id: s.id },
    })
  return hits
}

const getStyles = (theme: GrafanaTheme2) => ({
  wrap: css({ position: 'relative', flex: 1, maxWidth: 520, minWidth: 200 }),
  // Positioned container for a grafana Menu; the menu itself brings its own surface styling.
  pop: css({
    position: 'absolute',
    top: `calc(100% + ${theme.spacing(0.5)})`,
    left: 0,
    right: 0,
    zIndex: theme.zIndex.dropdown,
    maxHeight: 480,
    overflowY: 'auto',
    borderRadius: theme.shape.radius.default,
    boxShadow: theme.shadows.z3,
  }),
  foot: css({
    padding: theme.spacing(0.75, 1.5),
    borderTop: `1px solid ${theme.colors.border.weak}`,
    background: theme.colors.background.elevated,
  }),
})

export interface GlobalSearchHandle {
  focus: () => void
}

const GROUPS = ['runs', 'tests', 'databases', 'workloads', 'suites'] as const

// Header search: debounced lookup across runs/tests/databases/workloads/suites of the current tenant,
// grouped Menu, arrow keys + Enter, Escape closes. Enter with no selection opens the runs list filtered.
export const GlobalSearch = forwardRef<GlobalSearchHandle>(function GlobalSearch(_props, ref) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const { slug } = useTenant()
  const navigate = useNavigate()
  const inputRef = useRef<HTMLInputElement>(null)
  const popRef = useRef<HTMLDivElement>(null)
  const [value, setValue] = useState('')
  const [q, setQ] = useState('')
  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(0)
  useImperativeHandle(ref, () => ({ focus: () => inputRef.current?.focus() }))
  const debounced = useDebouncedCallback((v: string) => setQ(v.trim()), { wait: 250 })
  const hits = useQuery({
    queryKey: ['t', slug, 'search', q],
    queryFn: () => searchAll(slug, q),
    enabled: q.length >= 2,
    staleTime: 15_000,
  })
  const items = hits.data ?? []
  // biome-ignore lint/correctness/useExhaustiveDependencies: reset selection when the result set changes
  useEffect(() => setActive(0), [items.length])

  const go = (h: Hit) => {
    setOpen(false)
    setValue('')
    setQ('')
    inputRef.current?.blur()
    void navigate({ to: h.to as never, params: h.params as never })
  }
  const onKey = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setActive((a) => Math.min(items.length - 1, a + 1))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setActive((a) => Math.max(0, a - 1))
    } else if (e.key === 'Enter') {
      if (items[active]) go(items[active])
      else if (value.trim()) {
        setOpen(false)
        void navigate({
          to: '/t/$slug/runs' as never,
          params: { slug } as never,
          search: { search: value.trim() } as never,
        })
        inputRef.current?.blur()
      }
    } else if (e.key === 'Escape') {
      setOpen(false)
      inputRef.current?.blur()
    }
  }

  const grouped = GROUPS.map((g) => ({ g, list: items.filter((h) => h.group === g) })).filter(
    (x) => x.list.length
  )
  let idx = -1
  return (
    <div className={styles.wrap}>
      <Input
        ref={inputRef}
        value={value}
        placeholder={t('nav.search')}
        aria-label={t('common.actions.search')}
        prefix={<Icon name="search" />}
        suffix={
          <Text variant="bodySmall" color="secondary">
            /
          </Text>
        }
        loading={hits.isFetching}
        onChange={(e) => {
          setValue(e.currentTarget.value)
          debounced(e.currentTarget.value)
          setOpen(true)
        }}
        onFocus={() => setOpen(true)}
        onBlur={(e) => {
          // Focus moving onto a result (click on a Menu.Item) keeps the list open until it navigates.
          if (popRef.current?.contains(e.relatedTarget as Node | null)) return
          window.setTimeout(() => setOpen(false), 150)
        }}
        onKeyDown={onKey}
      />
      {open && q.length >= 2 && (
        <div className={styles.pop} ref={popRef}>
          <Menu ariaLabel={t('common.actions.search')}>
            {grouped.map(({ g, list }) => (
              <Menu.Group key={g} label={t(`nav.searchGroups.${g}`)}>
                {list.map((h) => {
                  idx += 1
                  const i = idx
                  const status = h.status ? t(`common.status.${h.status}`) : undefined
                  return (
                    <Menu.Item
                      key={`${h.group}-${h.id}`}
                      label={h.title}
                      description={[h.subtitle, status].filter(Boolean).join(' · ') || undefined}
                      icon={GROUP_ICON[h.group]}
                      active={i === active}
                      onClick={() => go(h)}
                    />
                  )
                })}
              </Menu.Group>
            ))}
            {!items.length && !hits.isFetching && (
              <Menu.Item label={t('nav.searchEmpty')} disabled />
            )}
          </Menu>
          {items.length > 0 && (
            <div className={styles.foot}>
              <Text variant="bodySmall" color="secondary">
                {t('nav.searchHintEnter')}
              </Text>
            </div>
          )}
        </div>
      )}
    </div>
  )
})
