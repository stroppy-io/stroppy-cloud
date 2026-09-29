import { catalogQueries } from '@api/queries/catalog'
import { keys } from '@api/queries/keys'
import { libraryMutations, workloadQueries } from '@api/queries/library'
import type { Protocol, Schemas } from '@api/types'
import { Page } from '@app/Page'
import { PageHeader } from '@app/PageHeader'
import { toast } from '@app/Toaster'
import { FavoriteButton } from '@components/FavoriteButton'
import { KeyValueList } from '@components/KeyValueList'
import { applyDefaults, type Schema, type Values } from '@components/schema/engine'
import { SchemaForm } from '@components/schema/SchemaForm'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import {
  Badge,
  Button,
  Combobox,
  Field,
  IconButton,
  LoadingPlaceholder,
  RadioButtonGroup,
  Stack,
  Tab,
  TabsBar,
  Text,
  useStyles2,
} from '@grafana/ui'
import { useSchemaValidate } from '@hooks/useSchemaValidate'
import { useTenant } from '@hooks/useTenant'
import { useMutation, useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { DiffPanel } from '../shared/DiffPanel'
import { EntityAbout, RenameModal } from '../shared/EntityAbout'
import { EntityActionsMenu } from '../shared/EntityActionsMenu'
import { Panel, WithSidebar } from '../shared/Panel'
import { RequirementsTable } from '../shared/RequirementsTable'
import { SegmentsEditor } from '../shared/SegmentsEditor'
import { UsagesList } from '../shared/UsagesList'
import { summarizeSegment } from './WorkloadListPage'

type Workload = Schemas['Workload']

const PROTOCOLS: Protocol[] = [
  'pg',
  'mysql',
  'picodata',
  'ydb_grpc',
  'ydb_grpcs',
  'cockroach',
  'noop',
]
const OPTION_FIELDS = ['driver', 'connection', 'baseline']

export const workloadDetailSearchSchema = z.object({
  tab: z
    .enum(['overview', 'segments', 'options', 'usages', 'diff'])
    .default('overview')
    .catch('overview'),
  seg: z.number().int().min(0).optional().catch(undefined),
  diff: z.string().optional().catch(undefined),
})
export type WorkloadDetailSearch = z.infer<typeof workloadDetailSearchSchema>

const getStyles = (theme: GrafanaTheme2) => ({
  tabs: css({ marginBottom: theme.spacing(2) }),
  table: css({
    width: '100%',
    borderCollapse: 'collapse',
    fontSize: theme.typography.bodySmall.fontSize,
    'th, td': {
      padding: theme.spacing(0.75, 1),
      borderBottom: `1px solid ${theme.colors.border.weak}`,
      textAlign: 'left',
      whiteSpace: 'nowrap',
    },
    th: { color: theme.colors.text.secondary, fontWeight: theme.typography.fontWeightMedium },
    'tr:last-child td': { borderBottom: 0 },
    'td.mono': { fontFamily: theme.typography.fontFamilyMonospace },
  }),
  mono: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
  }),
})

function optionsSummary(o: Record<string, unknown> | undefined) {
  const driver = (o?.driver ?? {}) as Record<string, unknown>
  const connection = (o?.connection ?? {}) as Record<string, unknown>
  const baseline = (o?.baseline ?? {}) as Record<string, unknown>
  return { driver, connection, baseline }
}

export function WorkloadDetailPage({
  id,
  search,
  onSearchChange,
}: {
  id: string
  search: WorkloadDetailSearch
  onSearchChange: (next: Partial<WorkloadDetailSearch>) => void
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const { slug, can } = useTenant()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const { data: wl } = useSuspenseQuery(workloadQueries.detail(slug, id))
  const canEdit = can('edit-library')
  const [renaming, setRenaming] = useState(false)
  const patch = useMutation({
    mutationFn: (body: Schemas['WorkloadPatch']) => libraryMutations.patchWorkload(slug, id, body),
    onSuccess: (next) => {
      qc.setQueryData(workloadQueries.detail(slug, id).queryKey, next)
      void qc.invalidateQueries({ queryKey: [...keys.t(slug), 'workloads'] })
      void qc.invalidateQueries({ queryKey: [...keys.t(slug), 'tests'] })
    },
  })
  const tabs: { id: WorkloadDetailSearch['tab']; label: string; counter?: number }[] = [
    { id: 'overview', label: t('library.tabs.overview') },
    { id: 'segments', label: t('library.tabs.segments'), counter: wl.segments.length },
    { id: 'options', label: t('library.tabs.options') },
    { id: 'usages', label: t('library.tabs.usages'), counter: wl.usages?.length ?? 0 },
    { id: 'diff', label: t('library.tabs.diff') },
  ]
  const { driver, connection, baseline } = optionsSummary(wl.options)

  return (
    <Page width="wide">
      <PageHeader
        title={wl.name}
        icon="bolt"
        breadcrumbs={[
          { label: t('nav.library') },
          { label: t('nav.workloads'), to: '/t/$slug/library/workloads', params: { slug } },
          { label: wl.name },
        ]}
        badge={
          <Stack gap={0.5}>
            <Badge color="purple" text={wl.protocol} />
            <Badge color="darkgrey" text={`stroppy ${wl.stroppy_version}`} />
          </Stack>
        }
        subtitle={wl.description}
        actions={
          <>
            <FavoriteButton kind="workload" id={wl.id} value={wl.is_favorite} />
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
                  search: { workload: wl.id } as never,
                })
              }
            >
              {t('library.workloads.newTest')}
            </Button>
            <EntityActionsMenu
              kind="workload"
              entity={wl}
              usages={wl.usages}
              asButton
              onDeleted={() =>
                void navigate({ to: '/t/$slug/library/workloads', params: { slug } })
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
              entity={wl}
              canEdit={canEdit}
              onSave={(p) => patch.mutateAsync(p)}
              extra={[
                { label: t('library.columns.stroppy'), value: wl.stroppy_version },
                { label: t('library.columns.protocol'), value: wl.protocol },
                {
                  label: t('library.databases.schema'),
                  value: wl.schema ? (
                    <span className={styles.mono}>
                      {wl.schema.id}@{wl.schema.version}
                    </span>
                  ) : (
                    '—'
                  ),
                },
                {
                  label: t('library.columns.usages'),
                  value: (
                    <Button
                      size="sm"
                      variant="secondary"
                      fill="text"
                      onClick={() => onSearchChange({ tab: 'usages' })}
                    >
                      {t('library.usages.count', { count: wl.usages?.length ?? 0 })}
                    </Button>
                  ),
                },
              ]}
            />
          }
        >
          <Panel
            title={t('library.tabs.segments')}
            description={t('library.segments.hint')}
            actions={
              <Button
                size="sm"
                variant="secondary"
                fill="text"
                icon="pen"
                onClick={() => onSearchChange({ tab: 'segments' })}
              >
                {t('common.actions.edit')}
              </Button>
            }
          >
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>#</th>
                  <th>{t('library.segments.name')}</th>
                  <th>{t('library.columns.script')}</th>
                  <th>{t('library.segments.executor')}</th>
                  <th>{t('library.segments.vus')}</th>
                  <th>{t('library.segments.limit')}</th>
                </tr>
              </thead>
              <tbody>
                {wl.segments.map((s, i) => {
                  const sum = summarizeSegment(s)
                  return (
                    <tr key={`${sum.name}-${i}`}>
                      <td>{i + 1}</td>
                      <td>
                        <Text weight="medium">{sum.name}</Text>
                      </td>
                      <td className="mono">{sum.script ?? '—'}</td>
                      <td>{sum.executor ?? '—'}</td>
                      <td>{sum.vus ?? '—'}</td>
                      <td>{sum.limit ?? '—'}</td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </Panel>
          <Panel
            title={t('library.tabs.options')}
            actions={
              <Button
                size="sm"
                variant="secondary"
                fill="text"
                icon="cog"
                onClick={() => onSearchChange({ tab: 'options' })}
              >
                {t('common.actions.edit')}
              </Button>
            }
          >
            <Stack gap={4} wrap="wrap">
              <KeyValueList
                title={t('library.options.driver')}
                items={[
                  {
                    label: t('library.options.insertMethod'),
                    value: String(driver.default_insert_method ?? t('schema.auto')),
                  },
                  {
                    label: t('library.options.bulkSize'),
                    value: String(driver.bulk_size ?? t('schema.auto')),
                  },
                ]}
              />
              <KeyValueList
                title={t('library.options.connection')}
                items={[
                  { label: t('common.fields.kind'), value: String(connection.kind ?? wl.protocol) },
                ]}
              />
              <KeyValueList
                title={t('library.options.baseline')}
                items={[
                  {
                    label: t('common.fields.enabled'),
                    value: baseline.enabled === false ? t('common.misc.no') : t('common.misc.yes'),
                  },
                  {
                    label: t('library.options.tiers'),
                    value: Array.isArray(baseline.tiers)
                      ? (baseline.tiers as string[]).join(', ')
                      : t('schema.auto'),
                  },
                  {
                    label: t('library.options.quick'),
                    value: baseline.quick ? t('common.misc.yes') : t('common.misc.no'),
                  },
                ]}
              />
            </Stack>
          </Panel>
          <Panel
            title={t('library.requirements.title')}
            description={t('library.workloads.form.runnerHint')}
          >
            <RequirementsTable requirements={wl.requirements} />
          </Panel>
        </WithSidebar>
      )}

      {search.tab === 'segments' && (
        <SegmentsTab
          wl={wl}
          canEdit={canEdit}
          selected={search.seg ?? 0}
          onSelect={(seg) => onSearchChange({ seg })}
          onSave={(segments) => patch.mutateAsync({ segments })}
        />
      )}

      {search.tab === 'options' && (
        <OptionsTab wl={wl} canEdit={canEdit} onSave={(p) => patch.mutateAsync(p)} />
      )}

      {search.tab === 'usages' && (
        <Panel
          title={t('library.usages.title')}
          description={t('library.usages.hint', { kind: t('library.kind.workload') })}
        >
          <UsagesList usages={wl.usages} emptyHint={t('library.usages.emptyWorkload')} />
        </Panel>
      )}

      {search.tab === 'diff' && (
        <Panel title={t('library.diff.title')}>
          <DiffPanel
            kind="workload"
            currentId={wl.id}
            currentName={wl.name}
            otherId={search.diff}
            onOtherChange={(diff) => onSearchChange({ diff })}
          />
        </Panel>
      )}

      <RenameModal
        isOpen={renaming}
        name={wl.name}
        onClose={() => setRenaming(false)}
        onSave={(name) => patch.mutateAsync({ name })}
      />
    </Page>
  )
}

function SegmentsTab({
  wl,
  canEdit,
  selected,
  onSelect,
  onSave,
}: {
  wl: Workload
  canEdit: boolean
  selected: number
  onSelect: (i: number) => void
  onSave: (segments: Values[]) => Promise<unknown>
}) {
  const { t } = useTranslation()
  const [segments, setSegments] = useState<Values[]>(wl.segments)
  const dirty = JSON.stringify(segments) !== JSON.stringify(wl.segments)
  const [saving, setSaving] = useState(false)
  const save = async () => {
    setSaving(true)
    try {
      await onSave(segments)
      toast.success(t('library.segments.saved'))
    } catch (e) {
      toast.error(e)
    } finally {
      setSaving(false)
    }
  }
  return (
    <Panel
      title={t('library.segments.title')}
      description={dirty ? t('library.databases.unsaved') : t('library.segments.hint')}
      actions={
        <Stack gap={1}>
          <Button
            size="sm"
            variant="secondary"
            disabled={!dirty}
            onClick={() => setSegments(wl.segments)}
          >
            {t('common.actions.reset')}
          </Button>
          <Button
            size="sm"
            disabled={!canEdit || !dirty || saving || segments.length === 0}
            onClick={() => void save()}
          >
            {t('common.actions.save')}
          </Button>
        </Stack>
      }
    >
      <SegmentsEditor
        segments={segments}
        onChange={setSegments}
        selected={selected}
        onSelect={onSelect}
        readOnly={!canEdit}
      />
    </Panel>
  )
}

function OptionsTab({
  wl,
  canEdit,
  onSave,
}: {
  wl: Workload
  canEdit: boolean
  onSave: (p: Schemas['WorkloadPatch']) => Promise<unknown>
}) {
  const { t } = useTranslation()
  const stroppy = useQuery(catalogQueries.stroppy())
  const schemaQ = useQuery(catalogQueries.schema('workload.stroppy'))
  const fullSchema = schemaQ.data as Schema | undefined
  const schema = useMemo<Schema | undefined>(
    () =>
      fullSchema
        ? { ...fullSchema, fields: fullSchema.fields.filter((f) => OPTION_FIELDS.includes(f.name)) }
        : undefined,
    [fullSchema]
  )
  const [version, setVersion] = useState(wl.stroppy_version)
  const [protocol, setProtocol] = useState<Protocol>(wl.protocol)
  const [options, setOptions] = useState<Values>(wl.options ?? {})
  const full = useMemo(
    () => (schema ? applyDefaults(schema.fields, options) : options),
    [schema, options]
  )
  const dirty =
    version !== wl.stroppy_version ||
    protocol !== wl.protocol ||
    JSON.stringify(full) !==
      JSON.stringify(schema ? applyDefaults(schema.fields, wl.options ?? {}) : wl.options)
  const validateValue = useMemo(
    () => ({ stroppy_version: version, protocol, segments: wl.segments, ...full }),
    [version, protocol, wl.segments, full]
  )
  const { errors, checking, hasErrors } = useSchemaValidate(
    'workload.stroppy',
    fullSchema,
    validateValue,
    dirty
  )
  const [saving, setSaving] = useState(false)
  const save = async () => {
    setSaving(true)
    try {
      await onSave({ stroppy_version: version, protocol, options: full })
      toast.success(t('library.options.saved'))
    } catch (e) {
      toast.error(e)
    } finally {
      setSaving(false)
    }
  }
  const ver = stroppy.data?.versions.find((v) => v.version === version)
  const allowed = ver?.protocols ?? PROTOCOLS
  return (
    <Panel
      title={t('library.options.title')}
      description={
        checking
          ? t('schema.validating')
          : hasErrors
            ? t('common.errors.validation')
            : dirty
              ? t('library.databases.unsaved')
              : t('library.options.hint')
      }
      actions={
        <Stack gap={1}>
          <Button
            size="sm"
            variant="secondary"
            disabled={!dirty}
            onClick={() => {
              setVersion(wl.stroppy_version)
              setProtocol(wl.protocol)
              setOptions(wl.options ?? {})
            }}
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
      <Stack gap={3} wrap="wrap">
        <Field
          label={t('library.columns.stroppy')}
          description={t('library.workloads.form.versionHint')}
        >
          <Combobox
            width={28}
            disabled={!canEdit}
            loading={stroppy.isPending}
            options={(stroppy.data?.versions ?? []).map((v) => ({
              label: v.default ? `${v.version} ★` : v.version,
              value: v.version,
              description: v.image,
            }))}
            value={version}
            onChange={(o) => setVersion(o.value)}
          />
        </Field>
        <Field
          label={t('library.columns.protocol')}
          description={t('library.workloads.form.protocolHint')}
        >
          <RadioButtonGroup
            disabled={!canEdit}
            options={PROTOCOLS.map((p) => ({ label: p, value: p, disabled: !allowed.includes(p) }))}
            value={protocol}
            onChange={setProtocol}
          />
        </Field>
      </Stack>
      {schema ? (
        <SchemaForm
          schema={schema}
          value={full}
          onChange={setOptions}
          errors={errors}
          readOnly={!canEdit}
        />
      ) : (
        <LoadingPlaceholder text={t('common.misc.loading')} />
      )}
    </Panel>
  )
}
