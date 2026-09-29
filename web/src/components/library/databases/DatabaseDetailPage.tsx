import { catalogQueries } from '@api/queries/catalog'
import { keys } from '@api/queries/keys'
import { databaseQueries, libraryMutations } from '@api/queries/library'
import type { CatalogDatabase, DatabaseWrite, Schemas } from '@api/types'
import { Page } from '@app/Page'
import { PageHeader } from '@app/PageHeader'
import { toast } from '@app/Toaster'
import { FavoriteButton } from '@components/FavoriteButton'
import { applyDefaults, defaults, type Schema, type Values } from '@components/schema/engine'
import { SchemaForm } from '@components/schema/SchemaForm'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import {
  Badge,
  Button,
  Combobox,
  Field,
  IconButton,
  InlineSwitch,
  LoadingPlaceholder,
  Stack,
  Tab,
  TabsBar,
  Text,
  useStyles2,
} from '@grafana/ui'
import { useSchemaValidate } from '@hooks/useSchemaValidate'
import { useTenant } from '@hooks/useTenant'
import { useDebouncedCallback } from '@tanstack/react-pacer'
import { useMutation, useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { DiffPanel } from '../shared/DiffPanel'
import { EntityAbout, RenameModal } from '../shared/EntityAbout'
import { EntityActionsMenu } from '../shared/EntityActionsMenu'
import { Panel, WithSidebar } from '../shared/Panel'
import { RequirementsTable } from '../shared/RequirementsTable'
import { TopologyPreview } from '../shared/TopologyPreview'
import { UsagesList } from '../shared/UsagesList'

type Database = Schemas['Database']
type Preview = Schemas['DatabasePreview']

export const databaseDetailSearchSchema = z.object({
  tab: z
    .enum(['overview', 'params', 'configs', 'usages', 'diff'])
    .default('overview')
    .catch('overview'),
  diff: z.string().optional().catch(undefined),
  role: z.string().optional().catch(undefined),
  schema: z.string().optional().catch(undefined),
})
export type DatabaseDetailSearch = z.infer<typeof databaseDetailSearchSchema>

const getStyles = (theme: GrafanaTheme2) => ({
  two: css({
    display: 'grid',
    gridTemplateColumns: 'minmax(0, 1fr) 360px',
    gap: theme.spacing(2),
    alignItems: 'start',
    [theme.breakpoints.down('lg')]: { gridTemplateColumns: 'minmax(0, 1fr)' },
  }),
  side: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(2) }),
  mono: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
  }),
  cfgList: css({
    display: 'grid',
    gap: theme.spacing(0.5),
    fontSize: theme.typography.bodySmall.fontSize,
    'div > span:first-child': { color: theme.colors.text.secondary, marginRight: theme.spacing(1) },
  }),
  tabs: css({ marginBottom: theme.spacing(2) }),
})

export function configSchemaId(base: string, version: string): string {
  return `${base}@${version.split('.')[0]}`
}

export function DatabaseDetailPage({
  id,
  search,
  onSearchChange,
}: {
  id: string
  search: DatabaseDetailSearch
  onSearchChange: (next: Partial<DatabaseDetailSearch>) => void
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const { slug, can } = useTenant()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const { data: db } = useSuspenseQuery(databaseQueries.detail(slug, id))
  const catalog = useQuery(catalogQueries.databases())
  const cat = catalog.data?.data.find((c) => c.kind === db.kind)
  const canEdit = can('edit-library')
  const [renaming, setRenaming] = useState(false)

  const patch = useMutation({
    mutationFn: (body: Schemas['DatabasePatch']) => libraryMutations.patchDatabase(slug, id, body),
    onSuccess: (next) => {
      qc.setQueryData(databaseQueries.detail(slug, id).queryKey, next)
      void qc.invalidateQueries({ queryKey: [...keys.t(slug), 'databases'] })
      void qc.invalidateQueries({ queryKey: [...keys.t(slug), 'tests'] })
    },
  })

  const tabs: { id: DatabaseDetailSearch['tab']; label: string; counter?: number }[] = [
    { id: 'overview', label: t('library.tabs.overview') },
    { id: 'params', label: t('library.tabs.params') },
    { id: 'configs', label: t('library.tabs.configs'), counter: countOverrides(db.configs) },
    { id: 'usages', label: t('library.tabs.usages'), counter: db.usages?.length ?? 0 },
    { id: 'diff', label: t('library.tabs.diff') },
  ]

  return (
    <Page width="wide">
      <PageHeader
        title={db.name}
        icon="database"
        breadcrumbs={[
          { label: t('nav.library') },
          { label: t('nav.databases'), to: '/t/$slug/library/databases', params: { slug } },
          { label: db.name },
        ]}
        badge={
          <Stack gap={0.5}>
            <Badge color="blue" text={`${cat?.title ?? db.kind} ${db.version}`} />
            {db.topology_preview && <Badge color="darkgrey" text={db.topology_preview.label} />}
            {!cat?.deployable && cat && (
              <Badge color="orange" text={t('library.wizard.db.notDeployable')} />
            )}
          </Stack>
        }
        subtitle={db.description}
        actions={
          <>
            <FavoriteButton kind="database" id={db.id} value={db.is_favorite} />
            {canEdit && (
              <IconButton
                name="pen"
                tooltip={t('library.about.rename')}
                onClick={() => setRenaming(true)}
              />
            )}
            <Button
              variant="secondary"
              icon="vial"
              onClick={() =>
                void navigate({
                  to: '/t/$slug/library/tests/new',
                  params: { slug },
                  search: { database: db.id } as never,
                })
              }
            >
              {t('library.databases.newTest')}
            </Button>
            <EntityActionsMenu
              kind="database"
              entity={db}
              usages={db.usages}
              asButton
              onDeleted={() =>
                void navigate({ to: '/t/$slug/library/databases', params: { slug } })
              }
            />
          </>
        }
      />
      <TabsBar className={styles.tabs}>
        {tabs.map((tab) => (
          <Tab
            key={tab.id}
            label={tab.label}
            active={search.tab === tab.id}
            counter={tab.counter}
            onChangeTab={() => onSearchChange({ tab: tab.id })}
          />
        ))}
      </TabsBar>

      {search.tab === 'overview' && (
        <WithSidebar
          sidebar={
            <EntityAbout
              entity={db}
              canEdit={canEdit}
              onSave={(p) => patch.mutateAsync(p)}
              extra={[
                { label: t('common.fields.kind'), value: cat?.title ?? db.kind },
                { label: t('common.fields.version'), value: db.version },
                {
                  label: t('library.databases.schema'),
                  value: db.schema ? (
                    <span className={styles.mono}>
                      {db.schema.id}@{db.schema.version}
                    </span>
                  ) : (
                    '—'
                  ),
                },
                { label: t('library.columns.topology'), value: db.topology_preview?.label ?? '—' },
                {
                  label: t('library.columns.usages'),
                  value: (
                    <Button
                      size="sm"
                      variant="secondary"
                      fill="text"
                      onClick={() => onSearchChange({ tab: 'usages' })}
                    >
                      {t('library.usages.count', { count: db.usages?.length ?? 0 })}
                    </Button>
                  ),
                },
              ]}
            />
          }
        >
          <Panel
            title={t('library.topology.title')}
            description={t('library.topology.derived')}
            actions={
              <Button
                size="sm"
                variant="secondary"
                fill="text"
                icon="pen"
                onClick={() => onSearchChange({ tab: 'params' })}
              >
                {t('library.tabs.params')}
              </Button>
            }
          >
            <TopologyPreview topology={db.topology_preview} />
          </Panel>
          <Panel
            title={t('library.requirements.title')}
            description={t('library.requirements.derived')}
          >
            <RequirementsTable requirements={db.requirements} />
          </Panel>
          <Panel
            title={t('library.tabs.configs')}
            description={t('library.configs.overviewHint')}
            actions={
              <Button
                size="sm"
                variant="secondary"
                fill="text"
                icon="cog"
                onClick={() => onSearchChange({ tab: 'configs' })}
              >
                {t('common.actions.edit')}
              </Button>
            }
          >
            <ConfigsOverview db={db} />
          </Panel>
          {db.validation?.errors && db.validation.errors.length > 0 && (
            <Panel title={t('library.validation.title')}>
              <Stack direction="column" gap={0.5}>
                {db.validation.errors.map((e) => (
                  <Text
                    key={`${e.path}-${e.code}`}
                    color={e.severity === 'WARNING' ? 'warning' : 'error'}
                  >
                    {e.path}: {e.message ?? e.code}
                  </Text>
                ))}
              </Stack>
            </Panel>
          )}
        </WithSidebar>
      )}

      {search.tab === 'params' && (
        <ParamsTab db={db} cat={cat} canEdit={canEdit} onSave={(p) => patch.mutateAsync(p)} />
      )}

      {search.tab === 'configs' && (
        <ConfigsTab
          db={db}
          cat={cat}
          canEdit={canEdit}
          role={search.role}
          schemaId={search.schema}
          onPick={(role, schema) => onSearchChange({ role, schema })}
          onSave={(configs) => patch.mutateAsync({ configs })}
        />
      )}

      {search.tab === 'usages' && (
        <Panel
          title={t('library.usages.title')}
          description={t('library.usages.hint', { kind: t('library.kind.database') })}
        >
          <UsagesList usages={db.usages} emptyHint={t('library.usages.emptyDatabase')} />
        </Panel>
      )}

      {search.tab === 'diff' && (
        <Panel title={t('library.diff.title')}>
          <DiffPanel
            kind="database"
            currentId={db.id}
            currentName={db.name}
            otherId={search.diff}
            onOtherChange={(diff) => onSearchChange({ diff })}
          />
        </Panel>
      )}

      <RenameModal
        isOpen={renaming}
        name={db.name}
        onClose={() => setRenaming(false)}
        onSave={(name) => patch.mutateAsync({ name })}
      />
    </Page>
  )
}

function countOverrides(configs: Database['configs']): number {
  let n = 0
  for (const role of Object.values(configs ?? {}))
    for (const v of Object.values(role)) n += Object.keys(v ?? {}).length
  return n
}

function ConfigsOverview({ db }: { db: Database }) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const roles = Object.entries(db.effective_configs ?? {})
  if (!roles.length) return <Text color="secondary">{t('library.configs.none')}</Text>
  return (
    <div className={styles.cfgList}>
      {roles.map(([role, schemas]) => (
        <div key={role}>
          <span>{role}</span>
          {Object.keys(schemas).map((sid) => {
            const overrides = Object.keys(db.configs?.[role]?.[sid] ?? {}).length
            return (
              <Badge
                key={sid}
                color={overrides ? 'orange' : 'darkgrey'}
                text={
                  overrides
                    ? `${sid} · ${t('library.configs.overrides', { count: overrides })}`
                    : sid
                }
              />
            )
          })}
        </div>
      ))}
    </div>
  )
}

// ---------- Params tab: schema form with live server validation + preview ----------
function ParamsTab({
  db,
  cat,
  canEdit,
  onSave,
}: {
  db: Database
  cat: CatalogDatabase | undefined
  canEdit: boolean
  onSave: (p: Schemas['DatabasePatch']) => Promise<unknown>
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const { slug } = useTenant()
  const schemaId = cat?.params_schema ?? db.schema?.id
  const schemaQ = useQuery({ ...catalogQueries.schema(schemaId ?? ''), enabled: !!schemaId })
  const schema = schemaQ.data as Schema | undefined
  const [value, setValue] = useState<Values>(db.params)
  const full = useMemo(
    () => (schema ? applyDefaults(schema.fields, value) : value),
    [schema, value]
  )
  const dirty =
    JSON.stringify(full) !==
    JSON.stringify(schema ? applyDefaults(schema.fields, db.params) : db.params)
  const { errors, checking, hasErrors } = useSchemaValidate(schemaId, schema, full, dirty)
  const [preview, setPreview] = useState<Preview | undefined>()
  const previewM = useMutation({
    mutationFn: (body: DatabaseWrite) => libraryMutations.previewDatabase(slug, body),
    onSuccess: setPreview,
  })
  const runPreview = useDebouncedCallback((body: DatabaseWrite) => previewM.mutate(body), {
    wait: 400,
  })
  const serialized = JSON.stringify(full)
  // biome-ignore lint/correctness/useExhaustiveDependencies: preview follows the serialized value
  useEffect(() => {
    if (dirty)
      runPreview({
        name: db.name,
        kind: db.kind,
        version: String(full.version ?? db.version),
        params: full,
      })
    else setPreview(undefined)
  }, [serialized, dirty, runPreview])
  const [saving, setSaving] = useState(false)
  const save = async () => {
    setSaving(true)
    try {
      const version = typeof full.version === 'string' ? full.version : db.version
      await onSave({ params: full, version })
      toast.success(t('library.databases.paramsSaved'))
    } catch (e) {
      toast.error(e)
    } finally {
      setSaving(false)
    }
  }
  const shown = preview ?? { topology_preview: db.topology_preview, requirements: db.requirements }
  return (
    <div className={styles.two}>
      <Panel
        title={t('library.databases.paramsTitle', { kind: cat?.title ?? db.kind })}
        description={
          checking
            ? t('schema.validating')
            : hasErrors
              ? t('common.errors.validation')
              : dirty
                ? t('library.databases.unsaved')
                : t('schema.valid')
        }
        actions={
          <Stack gap={1}>
            <Button
              size="sm"
              variant="secondary"
              disabled={!dirty}
              onClick={() => setValue(db.params)}
            >
              {t('common.actions.reset')}
            </Button>
            <Button
              size="sm"
              disabled={!canEdit || !dirty || hasErrors || saving}
              onClick={() => void save()}
            >
              {t('common.actions.save')}
            </Button>
          </Stack>
        }
      >
        {schema ? (
          <SchemaForm
            schema={schema}
            value={full}
            onChange={setValue}
            errors={errors}
            readOnly={!canEdit}
          />
        ) : schemaQ.isError ? (
          <Text color="error">{t('library.databases.schemaMissing', { id: schemaId ?? '' })}</Text>
        ) : (
          <LoadingPlaceholder text={t('common.misc.loading')} />
        )}
      </Panel>
      <div className={styles.side}>
        <Panel
          title={t('library.topology.title')}
          description={
            previewM.isPending
              ? t('library.wizard.db.previewUpdating')
              : dirty
                ? t('library.databases.previewOf')
                : undefined
          }
          dense
        >
          <TopologyPreview topology={shown.topology_preview} compact />
        </Panel>
        <Panel title={t('library.requirements.title')} dense>
          <RequirementsTable requirements={shown.requirements} />
        </Panel>
      </div>
    </div>
  )
}

// ---------- Configs tab: per role/schema diff editor ----------
function ConfigsTab({
  db,
  cat,
  canEdit,
  role,
  schemaId,
  onPick,
  onSave,
}: {
  db: Database
  cat: CatalogDatabase | undefined
  canEdit: boolean
  role: string | undefined
  schemaId: string | undefined
  onPick: (role: string, schema: string) => void
  onSave: (configs: NonNullable<Database['configs']>) => Promise<unknown>
}) {
  const { t } = useTranslation()
  const roleOptions = useMemo(() => {
    const nodes = db.topology_preview?.nodes ?? []
    return nodes
      .map((n) => ({
        role: n.role,
        schemas: cat?.roles.find((r) => r.role === n.role)?.config_schemas ?? [],
      }))
      .filter((r) => r.schemas.length > 0)
  }, [db.topology_preview, cat])
  const currentRole = roleOptions.find((r) => r.role === role) ?? roleOptions[0]
  // Prefer the ids the server resolved (effective_configs) — side-car software has its own major.
  const resolved = Object.keys(db.effective_configs?.[currentRole?.role ?? ''] ?? {})
  const schemaIds = resolved.length
    ? resolved
    : (currentRole?.schemas ?? []).map((s) => configSchemaId(s, db.version))
  const currentSchema = schemaIds.includes(schemaId ?? '') ? (schemaId as string) : schemaIds[0]
  const schemaQ = useQuery({
    ...catalogQueries.schema(currentSchema ?? ''),
    enabled: !!currentSchema,
  })
  const schema = schemaQ.data as Schema | undefined
  const stored = db.configs?.[currentRole?.role ?? '']?.[currentSchema ?? ''] ?? {}
  const effective = db.effective_configs?.[currentRole?.role ?? '']?.[currentSchema ?? ''] ?? stored
  const [value, setValue] = useState<Values>(effective)
  const [showAll, setShowAll] = useState(false)
  const key = `${currentRole?.role}/${currentSchema}`
  // biome-ignore lint/correctness/useExhaustiveDependencies: reset local edits when the selection changes
  useEffect(() => {
    setValue(effective)
  }, [key])
  const base = useMemo(() => (schema ? defaults(schema.fields) : {}), [schema])
  const diff = useMemo(() => {
    const out: Values = {}
    for (const [k, v] of Object.entries(value))
      if (JSON.stringify(v) !== JSON.stringify(base[k]) && v !== undefined) out[k] = v
    return out
  }, [value, base])
  const dirty = JSON.stringify(diff) !== JSON.stringify(stored)
  const { errors, checking, hasErrors } = useSchemaValidate(currentSchema, schema, value, dirty)
  const [saving, setSaving] = useState(false)
  const write = async (next: Values | undefined) => {
    if (!currentRole || !currentSchema) return
    setSaving(true)
    try {
      const configs: NonNullable<Database['configs']> = { ...db.configs }
      const roleCfg = { ...configs[currentRole.role] }
      if (next && Object.keys(next).length) roleCfg[currentSchema] = next
      else delete roleCfg[currentSchema]
      if (Object.keys(roleCfg).length) configs[currentRole.role] = roleCfg
      else delete configs[currentRole.role]
      await onSave(configs)
      toast.success(next ? t('library.configs.saved') : t('library.configs.resetDone'))
    } catch (e) {
      toast.error(e)
    } finally {
      setSaving(false)
    }
  }
  if (!roleOptions.length)
    return (
      <Panel title={t('library.tabs.configs')}>
        <Text color="secondary">{t('library.configs.none')}</Text>
      </Panel>
    )
  return (
    <Panel
      title={t('library.configs.title')}
      description={t('library.configs.hint')}
      actions={
        <Stack gap={1} alignItems="center">
          <InlineSwitch
            label={t('schema.showAll')}
            showLabel
            value={showAll}
            onChange={(e) => setShowAll(e.currentTarget.checked)}
          />
          <Button
            size="sm"
            variant="secondary"
            icon="history"
            disabled={!canEdit || (!Object.keys(stored).length && !dirty) || saving}
            onClick={() => void write(undefined)}
          >
            {t('schema.reset')}
          </Button>
          <Button
            size="sm"
            disabled={!canEdit || !dirty || hasErrors || saving}
            onClick={() => void write(diff)}
          >
            {t('common.actions.save')}
          </Button>
        </Stack>
      }
    >
      <Stack gap={2} wrap="wrap" alignItems="flex-end">
        <Field label={t('library.configs.role')} noMargin>
          <Combobox
            width={24}
            options={roleOptions.map((r) => ({
              label: r.role,
              value: r.role,
              description: r.schemas.join(', '),
            }))}
            value={currentRole?.role ?? null}
            onChange={(o) =>
              onPick(
                o.value,
                Object.keys(db.effective_configs?.[o.value] ?? {})[0] ??
                  configSchemaId(
                    roleOptions.find((r) => r.role === o.value)?.schemas[0] ?? '',
                    db.version
                  )
              )
            }
          />
        </Field>
        <Field label={t('library.configs.schema')} noMargin>
          <Combobox
            width={30}
            options={schemaIds.map((s) => ({
              label: s,
              value: s,
              description: t('library.configs.overrides', {
                count: Object.keys(db.configs?.[currentRole?.role ?? '']?.[s] ?? {}).length,
              }),
            }))}
            value={currentSchema ?? null}
            onChange={(o) => currentRole && onPick(currentRole.role, o.value)}
          />
        </Field>
        <Text color="secondary" variant="bodySmall">
          {checking
            ? t('schema.validating')
            : t('library.configs.diffCount', { count: Object.keys(diff).length })}
          {dirty ? ` · ${t('library.databases.unsaved')}` : ''}
        </Text>
      </Stack>
      <div style={{ marginTop: 16 }}>
        {schema ? (
          <SchemaForm
            schema={schema}
            value={value}
            onChange={setValue}
            errors={errors}
            readOnly={!canEdit}
            diffOnly={!showAll}
            showComputed={showAll}
          />
        ) : schemaQ.isError ? (
          <Text color="error">
            {t('library.databases.schemaMissing', { id: currentSchema ?? '' })}
          </Text>
        ) : (
          <LoadingPlaceholder text={t('common.misc.loading')} />
        )}
        {schema && !showAll && Object.keys(diff).length === 0 && (
          <Text color="secondary">{t('library.configs.allDefault')}</Text>
        )}
      </div>
    </Panel>
  )
}
