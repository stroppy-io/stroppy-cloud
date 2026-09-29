import { catalogQueries } from '@api/queries/catalog'
import { keys } from '@api/queries/keys'
import {
  databaseQueries,
  libraryMutations,
  testQueries,
  workloadQueries,
} from '@api/queries/library'
import { providerQueries } from '@api/queries/settings'
import type { DatabaseWrite, Schemas, TestValidation, WorkloadWrite } from '@api/types'
import { ErrorState } from '@app/ErrorState'
import { Page } from '@app/Page'
import { PageHeader } from '@app/PageHeader'
import { toast } from '@app/Toaster'
import { StatusBadge } from '@components/StatusBadge'
import { TagsEditor } from '@components/TagsEditor'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import {
  Alert,
  Badge,
  Button,
  Combobox,
  Field,
  Icon,
  Input,
  LoadingPlaceholder,
  RadioButtonGroup,
  Stack,
  Text,
  TextArea,
  useStyles2,
} from '@grafana/ui'
import { useTenant } from '@hooks/useTenant'
import { useForm } from '@tanstack/react-form'
import { useDebouncedCallback } from '@tanstack/react-pacer'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { DatabaseWizard } from '../databases/DatabaseWizard'
import { applyServerErrors, firstError, serverFormError } from '../shared/form'
import { LibraryPicker } from '../shared/LibraryPicker'
import { Panel } from '../shared/Panel'
import { RequirementsTable } from '../shared/RequirementsTable'
import { SizesEditor } from '../shared/SizesEditor'
import { TopologyPreview } from '../shared/TopologyPreview'
import { ValidationIssues } from '../shared/ValidationIssues'
import { WorkloadForm } from '../workloads/WorkloadForm'
import { KEEP_RE } from './LaunchDrawer'
import { stepForPath } from './TestDetailPage'

type Test = Schemas['Test']
type TestSpec = Schemas['TestSpec']
type RoleSizes = Schemas['RoleSizes']
type RefValue = { ref: { id: string } } | { inline: Record<string, unknown> }

export const testWizardSearchSchema = z.object({
  id: z.string().optional().catch(undefined),
  step: z.number().int().min(1).max(4).default(1).catch(1),
  database: z.string().optional().catch(undefined),
  workload: z.string().optional().catch(undefined),
})
export type TestWizardSearch = z.infer<typeof testWizardSearchSchema>

const getStyles = (theme: GrafanaTheme2) => ({
  steps: css({
    display: 'flex',
    gap: theme.spacing(0.5),
    marginBottom: theme.spacing(2),
    flexWrap: 'wrap',
  }),
  step: css({
    display: 'inline-flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    padding: theme.spacing(0.75, 1.5),
    borderRadius: theme.shape.radius.default,
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    border: `1px solid ${theme.colors.border.weak}`,
    background: 'transparent',
    cursor: 'default',
  }),
  stepClickable: css({ cursor: 'pointer', '&:hover': { background: theme.colors.action.hover } }),
  stepActive: css({
    color: theme.colors.text.primary,
    borderColor: theme.colors.primary.border,
    background: theme.colors.action.selected,
  }),
  num: css({
    width: 20,
    height: 20,
    borderRadius: 999,
    display: 'inline-grid',
    placeItems: 'center',
    background: theme.colors.background.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
  }),
  two: css({
    display: 'grid',
    gridTemplateColumns: 'minmax(0, 1fr) 360px',
    gap: theme.spacing(2),
    alignItems: 'start',
    [theme.breakpoints.down('lg')]: { gridTemplateColumns: 'minmax(0, 1fr)' },
  }),
  side: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(2) }),
  footer: css({
    display: 'flex',
    justifyContent: 'space-between',
    gap: theme.spacing(1),
    marginTop: theme.spacing(3),
    paddingTop: theme.spacing(2),
    borderTop: `1px solid ${theme.colors.border.weak}`,
  }),
  summary: css({
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))',
    gap: theme.spacing(2),
  }),
  mono: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
  }),
})

type RefMode = 'ref' | 'inline'

// Wizard: creates a draft on the first step and PATCHes per step. Steps live in `?step=`, the draft in `?id=`.
export function TestWizard({
  search,
  onSearchChange,
}: {
  search: TestWizardSearch
  onSearchChange: (next: Partial<TestWizardSearch>) => void
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const { slug } = useTenant()
  const qc = useQueryClient()
  const navigate = useNavigate()
  const draft = useQuery({ ...testQueries.detail(slug, search.id ?? ''), enabled: !!search.id })
  const test = draft.data
  const step = search.step

  const create = useMutation({
    mutationFn: (body: Schemas['TestWrite']) => libraryMutations.createTest(slug, body),
  })
  const patch = useMutation({
    mutationFn: ({ body, finalize }: { body: Schemas['TestPatch']; finalize?: boolean }) =>
      libraryMutations.patchTest(slug, search.id ?? '', body, finalize),
    onSuccess: (next) => {
      qc.setQueryData(testQueries.detail(slug, next.id).queryKey, next)
      void qc.invalidateQueries({ queryKey: [...keys.t(slug), 'tests'] })
    },
  })
  const busy = create.isPending || patch.isPending

  // Persist a step's payload: create the draft on the first save, PATCH afterwards.
  const save = async (body: Schemas['TestPatch'], nextStep?: number) => {
    try {
      if (!search.id) {
        const created = await create.mutateAsync({ name: t('library.wizard.untitled'), ...body })
        void qc.invalidateQueries({ queryKey: [...keys.t(slug), 'tests'] })
        onSearchChange({
          id: created.id,
          step: nextStep ?? step,
          database: undefined,
          workload: undefined,
        })
        return created
      }
      const next = await patch.mutateAsync({ body })
      if (nextStep) onSearchChange({ step: nextStep })
      return next
    } catch (e) {
      toast.error(e)
      return undefined
    }
  }
  const goTo = (n: number) => onSearchChange({ step: n })
  const canGo = (n: number) =>
    n === 1 ||
    (!!test &&
      (n === 2
        ? !!test.database
        : n === 3
          ? !!test.database && !!test.workload
          : !!test.database && !!test.workload))
  const steps = [
    t('library.wizard.steps.database'),
    t('library.wizard.steps.workload'),
    t('library.wizard.steps.sizes'),
    t('library.wizard.steps.review'),
  ]

  if (search.id && draft.isPending) return <LoadingPlaceholder text={t('common.misc.loading')} />
  if (search.id && draft.isError)
    return <ErrorState error={draft.error} onRetry={() => void draft.refetch()} />

  return (
    <Page width="wide">
      <PageHeader
        title={test?.name ?? t('library.tests.new')}
        icon="vial"
        breadcrumbs={[
          { label: t('nav.library') },
          { label: t('nav.tests'), to: '/t/$slug/library/tests', params: { slug } },
          { label: test?.name ?? t('common.actions.create') },
        ]}
        badge={test ? <StatusBadge status={test.status} /> : undefined}
        subtitle={search.id ? t('library.wizard.draftHint') : t('library.wizard.intro')}
        actions={
          test ? (
            <Button
              variant="secondary"
              icon="eye"
              onClick={() =>
                void navigate({ to: '/t/$slug/library/tests/$id', params: { slug, id: test.id } })
              }
            >
              {t('library.wizard.openDetail')}
            </Button>
          ) : undefined
        }
      />
      <div className={styles.steps}>
        {steps.map((label, i) => {
          const n = i + 1
          const clickable = canGo(n) && n !== step
          return (
            <button
              type="button"
              key={label}
              className={cx(
                styles.step,
                n === step && styles.stepActive,
                clickable && styles.stepClickable
              )}
              onClick={clickable ? () => goTo(n) : undefined}
              disabled={!canGo(n)}
              aria-current={n === step ? 'step' : undefined}
            >
              <span className={styles.num}>{n < step ? <Icon name="check" size="xs" /> : n}</span>
              {label}
            </button>
          )
        })}
      </div>

      {step === 1 && (
        <RefStep
          kind="database"
          test={test}
          initialRef={search.database}
          busy={busy}
          onSubmit={(database) => void save({ database: database as TestSpec['database'] }, 2)}
          onCancel={() => void navigate({ to: '/t/$slug/library/tests', params: { slug } })}
        />
      )}
      {step === 2 && (
        <RefStep
          kind="workload"
          test={test}
          initialRef={search.workload}
          busy={busy}
          onSubmit={(workload) => void save({ workload: workload as TestSpec['workload'] }, 3)}
          onBack={() => goTo(1)}
        />
      )}
      {step === 3 && test && (
        <SizesStep
          test={test}
          busy={busy}
          onBack={() => goTo(2)}
          onSubmit={(body) => void save(body, 4)}
        />
      )}
      {step === 4 && test && (
        <ReviewStep
          test={test}
          busy={busy}
          onBack={() => goTo(3)}
          onGoTo={(n) => goTo(n)}
          onSaveDraft={async (body) => {
            const r = await save(body)
            if (r) toast.success(t('library.wizard.savedDraft'))
          }}
          onFinalize={async (body, launch) => {
            const next = await patch.mutateAsync({ body, finalize: true })
            toast.success(t('library.wizard.ready', { name: next.name }))
            void navigate({
              to: '/t/$slug/library/tests/$id',
              params: { slug, id: next.id },
              search: launch ? ({ launch: true } as never) : ({} as never),
            })
          }}
        />
      )}
    </Page>
  )
}

// ---------- step 1/2: pick an existing record or describe one inline ----------
function RefStep({
  kind,
  test,
  initialRef,
  busy,
  onSubmit,
  onBack,
  onCancel,
}: {
  kind: 'database' | 'workload'
  test: Test | undefined
  initialRef?: string
  busy: boolean
  onSubmit: (ref: RefValue) => void
  onBack?: () => void
  onCancel?: () => void
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const { slug } = useTenant()
  const current = kind === 'database' ? test?.database : test?.workload
  const [mode, setMode] = useState<RefMode>(current && 'inline' in current ? 'inline' : 'ref')
  const [refId, setRefId] = useState<string | null>(
    current && 'ref' in current ? current.ref.id : (initialRef ?? null)
  )
  const [inlineSpec, setInlineSpec] = useState<DatabaseWrite | WorkloadWrite | undefined>(
    current && 'inline' in current
      ? ({ name: '', ...current.inline } as DatabaseWrite | WorkloadWrite)
      : undefined
  )
  const dbDetail = useQuery({
    ...databaseQueries.detail(slug, refId ?? ''),
    enabled: kind === 'database' && !!refId,
  })
  const wlDetail = useQuery({
    ...workloadQueries.detail(slug, refId ?? ''),
    enabled: kind === 'workload' && !!refId,
  })
  const picked = kind === 'database' ? dbDetail.data : wlDetail.data
  const submit = () => {
    if (mode === 'ref' && refId) onSubmit({ ref: { id: refId } })
    else if (mode === 'inline' && inlineSpec) {
      const {
        name: _n,
        description: _d,
        tags: _t,
        ...spec
      } = inlineSpec as DatabaseWrite & WorkloadWrite
      onSubmit({ inline: spec as Record<string, unknown> })
    }
  }
  const ready = mode === 'ref' ? !!refId : !!inlineSpec
  return (
    <div>
      <div className={styles.two}>
        <Panel
          title={t(`library.wizard.${kind === 'database' ? 'pickDatabase' : 'pickWorkload'}`)}
          description={t(
            `library.wizard.${kind === 'database' ? 'pickDatabaseHint' : 'pickWorkloadHint'}`
          )}
        >
          <Stack direction="column" gap={2}>
            <RadioButtonGroup
              options={[
                { label: t('library.wizard.fromLibrary'), value: 'ref' as RefMode, icon: 'book' },
                { label: t('library.wizard.inline'), value: 'inline' as RefMode, icon: 'plus' },
              ]}
              value={mode}
              onChange={setMode}
            />
            {mode === 'ref' ? (
              <Stack direction="column" gap={2}>
                <LibraryPicker kind={kind} value={refId} onChange={setRefId} autoFocus width={48} />
                {picked && kind === 'database' && 'kind' in picked && (
                  <Stack direction="column" gap={1}>
                    <Stack gap={1} alignItems="center">
                      <Text weight="medium">{picked.name}</Text>
                      <Badge color="blue" text={`${picked.kind} ${picked.version}`} />
                      {picked.description && <Text color="secondary">{picked.description}</Text>}
                    </Stack>
                    <TopologyPreview topology={picked.topology_preview} compact />
                  </Stack>
                )}
                {picked && kind === 'workload' && 'protocol' in picked && (
                  <Stack direction="column" gap={1}>
                    <Stack gap={1} alignItems="center">
                      <Text weight="medium">{picked.name}</Text>
                      <Badge color="purple" text={picked.protocol} />
                      <Badge color="darkgrey" text={`stroppy ${picked.stroppy_version}`} />
                    </Stack>
                    <Text color="secondary">
                      {t('library.wizard.segmentsSummary', {
                        count: picked.segments.length,
                        scripts: picked.segments
                          .map(
                            (s) => (s as { workload?: { script?: string } }).workload?.script ?? '?'
                          )
                          .join(', '),
                      })}
                    </Text>
                  </Stack>
                )}
              </Stack>
            ) : inlineSpec ? (
              <Alert
                severity="success"
                title={t('library.wizard.inlineReady')}
                buttonContent={t('common.actions.edit')}
                onRemove={() => setInlineSpec(undefined)}
              >
                <span className={styles.mono}>
                  {kind === 'database'
                    ? `${(inlineSpec as DatabaseWrite).kind} ${(inlineSpec as DatabaseWrite).version}`
                    : `${(inlineSpec as WorkloadWrite).protocol} · stroppy ${(inlineSpec as WorkloadWrite).stroppy_version} · ${(inlineSpec as WorkloadWrite).segments.length} seg`}
                </span>
              </Alert>
            ) : kind === 'database' ? (
              <DatabaseWizard mode="inline" onDone={({ spec }) => spec && setInlineSpec(spec)} />
            ) : (
              <WorkloadForm
                mode="inline"
                onDone={({ spec }) => spec && setInlineSpec(spec)}
                defaultProtocol={test?.resolved?.database ? undefined : 'pg'}
              />
            )}
          </Stack>
        </Panel>
        <div className={styles.side}>
          {picked && 'requirements' in picked && (
            <Panel title={t('library.requirements.title')} dense>
              <RequirementsTable requirements={picked.requirements} />
            </Panel>
          )}
          {kind === 'workload' && test?.resolved?.database && (
            <Panel title={t('library.kind.database')} dense>
              <Stack gap={1} alignItems="center">
                <Icon name="database" />
                <Text>{test.resolved.database.name}</Text>
                <Badge
                  color="blue"
                  text={`${test.resolved.database.kind} ${test.resolved.database.version}`}
                />
              </Stack>
              <Text color="secondary" variant="bodySmall">
                {t('library.wizard.protocolsHint', { kind: test.resolved.database.kind })}
              </Text>
            </Panel>
          )}
        </div>
      </div>
      <div className={styles.footer}>
        <div>
          {onCancel && (
            <Button variant="secondary" fill="text" onClick={onCancel}>
              {t('common.actions.cancel')}
            </Button>
          )}
        </div>
        <Stack gap={1}>
          {onBack && (
            <Button variant="secondary" icon="arrow-left" onClick={onBack}>
              {t('common.actions.back')}
            </Button>
          )}
          <Button onClick={submit} disabled={!ready || busy}>
            {t('common.actions.next')}
          </Button>
        </Stack>
      </div>
    </div>
  )
}

// ---------- step 3: provider + sizes with live :validate ----------
function SizesStep({
  test,
  busy,
  onBack,
  onSubmit,
}: {
  test: Test
  busy: boolean
  onBack: () => void
  onSubmit: (body: Schemas['TestPatch']) => void
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const { slug } = useTenant()
  const providers = useQuery(providerQueries.list(slug))
  const catalogProviders = useQuery(catalogQueries.providers())
  const [providerId, setProviderId] = useState<string | null>(test.provider_profile_id ?? null)
  const [sizes, setSizes] = useState<RoleSizes>(test.sizes ?? {})
  const [keep, setKeep] = useState(test.keep ?? '0s')
  const [validation, setValidation] = useState<TestValidation | undefined>()
  const validate = useMutation({
    mutationFn: (spec: Schemas['TestWrite']) => libraryMutations.validateTest(slug, spec),
    onSuccess: setValidation,
  })
  const run = useDebouncedCallback((spec: Schemas['TestWrite']) => validate.mutate(spec), {
    wait: 400,
  })
  const serialized = JSON.stringify({ sizes, providerId, keep })
  // biome-ignore lint/correctness/useExhaustiveDependencies: validate on change
  useEffect(() => {
    run({
      name: test.name,
      database: test.database,
      workload: test.workload,
      sizes,
      provider_profile_id: providerId,
      keep: KEEP_RE.test(keep) ? keep || '0s' : undefined,
      rating: test.rating,
    })
  }, [serialized, run])
  // pre-fill sizes with the suggested ones when nothing was chosen yet
  useEffect(() => {
    if (Object.keys(sizes).length || !validation) return
    const next: RoleSizes = {}
    for (const i of validation.validation.issues ?? [])
      if (i.code === 'REQUIRED' && i.path.startsWith('sizes.') && i.suggested)
        next[i.path.split('.')[1]] = { size: String(i.suggested) as Schemas['Size'] }
    if (Object.keys(next).length) setSizes(next)
  }, [validation, sizes])
  const profile = providers.data?.data.find((p) => p.id === providerId)
  const catProvider =
    catalogProviders.data?.data.find((c) => c.kind === profile?.kind) ??
    catalogProviders.data?.data[0]
  const issues = validation?.validation.issues ?? []
  const keepOk = KEEP_RE.test(keep)
  return (
    <div>
      <div className={styles.two}>
        <Stack direction="column" gap={2}>
          <Panel
            title={t('library.wizard.providerTitle')}
            description={t('library.wizard.providerHint')}
          >
            <Combobox
              width={40}
              isClearable
              loading={providers.isPending}
              placeholder={t('library.tests.pickAtLaunch')}
              options={(providers.data?.data ?? []).map((p) => ({
                label: p.name,
                value: p.id,
                description: `${p.kind} · ${p.status}`,
              }))}
              value={providerId}
              onChange={(o) => setProviderId(o?.value ?? null)}
            />
          </Panel>
          <Panel
            title={t('common.fields.sizes')}
            description={
              validate.isPending ? t('schema.validating') : t('library.wizard.sizesHint')
            }
          >
            <SizesEditor
              value={sizes}
              onChange={setSizes}
              requirements={validation?.requirements ?? test.requirements}
              sizeTables={catProvider?.sizes}
              diskTypes={catProvider?.disk_types}
              issues={issues}
            />
          </Panel>
          <Panel title={t('library.launch.keep')} description={t('library.launch.keepHint')}>
            <Input
              width={16}
              placeholder="0s"
              value={keep}
              invalid={!keepOk}
              onChange={(e) => setKeep(e.currentTarget.value)}
            />
          </Panel>
        </Stack>
        <div className={styles.side}>
          <Panel title={t('library.requirements.title')} dense>
            <RequirementsTable
              requirements={validation?.requirements ?? test.requirements}
              sizes={sizes}
              sizeTable={catProvider?.sizes?.db}
            />
          </Panel>
          <Panel title={t('library.validation.title')} dense>
            <ValidationIssues issues={issues} compact />
          </Panel>
        </div>
      </div>
      <div className={styles.footer}>
        <div />
        <Stack gap={1}>
          <Button variant="secondary" icon="arrow-left" onClick={onBack}>
            {t('common.actions.back')}
          </Button>
          <Button
            disabled={busy || !keepOk}
            onClick={() => onSubmit({ sizes, provider_profile_id: providerId, keep: keep || '0s' })}
          >
            {t('common.actions.next')}
          </Button>
        </Stack>
      </div>
    </div>
  )
}

// ---------- step 4: name + review + finalize ----------
function ReviewStep({
  test,
  busy,
  onBack,
  onGoTo,
  onSaveDraft,
  onFinalize,
}: {
  test: Test
  busy: boolean
  onBack: () => void
  onGoTo: (step: number) => void
  onSaveDraft: (body: Schemas['TestPatch']) => Promise<void>
  onFinalize: (body: Schemas['TestPatch'], launch: boolean) => Promise<void>
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const { slug } = useTenant()
  const providers = useQuery(providerQueries.list(slug))
  const [validation, setValidation] = useState<TestValidation | undefined>()
  const validate = useMutation({
    mutationFn: (spec: Schemas['TestWrite']) => libraryMutations.validateTest(slug, spec),
    onSuccess: setValidation,
  })
  // biome-ignore lint/correctness/useExhaustiveDependencies: validate the saved draft once on entry
  useEffect(() => {
    validate.mutate({
      name: test.name,
      database: test.database,
      workload: test.workload,
      sizes: test.sizes,
      provider_profile_id: test.provider_profile_id,
      keep: test.keep,
      rating: test.rating,
    })
  }, [test.updated_at])
  const [action, setAction] = useState<'draft' | 'ready' | 'launch'>('ready')
  const schema = useMemo(
    () =>
      z.object({
        // OpenAPI TestWrite.name
        name: z
          .string()
          .trim()
          .min(1, t('common.validation.required'))
          .max(120, t('common.validation.max', { max: 120 })),
        description: z
          .string()
          .trim()
          .max(2000, t('common.validation.max', { max: 2000 })),
        tags: z.record(z.string(), z.string()),
      }),
    [t]
  )
  const form = useForm({
    defaultValues: {
      name: test.name === t('library.wizard.untitled') ? '' : test.name,
      description: test.description ?? '',
      tags: test.tags ?? {},
    },
    validators: { onSubmit: schema },
    onSubmit: async ({ value, formApi }) => {
      const body: Schemas['TestPatch'] = {
        name: value.name.trim().replace(/\s+/g, ' '),
        description: value.description.trim(),
        tags: value.tags,
      }
      try {
        if (action === 'draft') await onSaveDraft(body)
        else await onFinalize(body, action === 'launch')
      } catch (e) {
        applyServerErrors(formApi, e)
      }
    },
  })
  const issues = validation?.validation.issues ?? test.validation?.issues ?? []
  const fits = validation ? validation.validation.fits : test.validation?.fits
  const db = validation?.resolved?.database ?? test.resolved?.database
  const wl = validation?.resolved?.workload ?? test.resolved?.workload
  const provider = providers.data?.data.find((p) => p.id === test.provider_profile_id)
  const submitAs = (a: 'draft' | 'ready' | 'launch') => {
    setAction(a)
    window.setTimeout(() => void form.handleSubmit(), 0)
  }
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        void form.handleSubmit()
      }}
    >
      <div className={styles.two}>
        <Stack direction="column" gap={2}>
          <Panel title={t('library.wizard.nameTitle')}>
            <form.Field name="name">
              {(field) => {
                const err = firstError(field.state.meta)
                return (
                  <Field
                    label={t('common.fields.name')}
                    required
                    invalid={!!err}
                    error={err}
                    htmlFor={field.name}
                  >
                    <Input
                      id={field.name}
                      autoFocus
                      placeholder={`${db?.name ?? 'db'} × ${wl?.name ?? 'workload'}`}
                      value={field.state.value}
                      onBlur={field.handleBlur}
                      onChange={(e) => field.handleChange(e.currentTarget.value)}
                    />
                  </Field>
                )
              }}
            </form.Field>
            <form.Field name="description">
              {(field) => (
                <Field label={t('common.fields.description')} htmlFor={field.name}>
                  <TextArea
                    id={field.name}
                    rows={2}
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.currentTarget.value)}
                  />
                </Field>
              )}
            </form.Field>
            <form.Field name="tags">
              {(field) => (
                <Field label={t('common.fields.tags')} description={t('library.about.tagsHint')}>
                  <TagsEditor
                    value={field.state.value}
                    onChange={(v) => field.handleChange(v)}
                    placeholder="purpose=smoke"
                  />
                </Field>
              )}
            </form.Field>
            <form.Subscribe selector={(s) => serverFormError(s.errorMap.onServer)}>
              {(err) => (err ? <Text color="error">{err}</Text> : null)}
            </form.Subscribe>
          </Panel>
          <div className={styles.summary}>
            <Panel
              title={t('library.kind.database')}
              dense
              actions={
                <Button
                  size="sm"
                  fill="text"
                  variant="secondary"
                  icon="pen"
                  onClick={() => onGoTo(1)}
                >
                  {t('common.actions.edit')}
                </Button>
              }
            >
              {db ? (
                <Stack direction="column" gap={1}>
                  <Stack gap={1} alignItems="center">
                    <Text weight="medium">{db.name}</Text>
                    <Badge color="blue" text={`${db.kind} ${db.version}`} />
                  </Stack>
                  <TopologyPreview topology={db.topology_preview} compact />
                </Stack>
              ) : (
                <Text color="secondary">—</Text>
              )}
            </Panel>
            <Panel
              title={t('library.kind.workload')}
              dense
              actions={
                <Button
                  size="sm"
                  fill="text"
                  variant="secondary"
                  icon="pen"
                  onClick={() => onGoTo(2)}
                >
                  {t('common.actions.edit')}
                </Button>
              }
            >
              {wl ? (
                <Stack direction="column" gap={0.5}>
                  <Stack gap={1} alignItems="center">
                    <Text weight="medium">{wl.name}</Text>
                    <Badge color="purple" text={wl.protocol} />
                  </Stack>
                  <Text color="secondary" variant="bodySmall">
                    {t('library.wizard.segmentsSummary', {
                      count: wl.segments.length,
                      scripts: wl.segments
                        .map(
                          (s) => (s as { workload?: { script?: string } }).workload?.script ?? '?'
                        )
                        .join(', '),
                    })}
                  </Text>
                </Stack>
              ) : (
                <Text color="secondary">—</Text>
              )}
            </Panel>
          </div>
          <Panel
            title={t('library.wizard.steps.sizes')}
            dense
            actions={
              <Button
                size="sm"
                fill="text"
                variant="secondary"
                icon="pen"
                onClick={() => onGoTo(3)}
              >
                {t('common.actions.edit')}
              </Button>
            }
          >
            <Stack direction="column" gap={1}>
              <Stack gap={2} wrap="wrap">
                <Text>
                  {t('common.fields.provider')}:{' '}
                  <Text weight="medium">{provider?.name ?? t('library.tests.pickAtLaunch')}</Text>
                </Text>
                <Text>
                  {t('library.launch.keep')}: <Text weight="medium">{test.keep ?? '0s'}</Text>
                </Text>
              </Stack>
              <RequirementsTable
                requirements={validation?.requirements ?? test.requirements}
                sizes={test.sizes}
                sizeTable={undefined}
              />
              <Stack gap={1} wrap="wrap">
                {Object.entries(test.sizes ?? {}).map(([role, s]) => (
                  <Badge
                    key={role}
                    color="darkgrey"
                    text={`${role}: ${s.size}${s.disk?.gb ? ` · ${s.disk.gb} GB` : ''}`}
                  />
                ))}
              </Stack>
            </Stack>
          </Panel>
        </Stack>
        <div className={styles.side}>
          <Panel
            title={t('library.validation.title')}
            description={
              validate.isPending
                ? t('schema.validating')
                : fits
                  ? t('library.validation.fits')
                  : t('library.validation.doesNotFitShort')
            }
            dense
          >
            <ValidationIssues issues={issues} compact onGoTo={(p) => onGoTo(stepForPath(p))} />
          </Panel>
        </div>
      </div>
      <div className={styles.footer}>
        <Button
          type="button"
          variant="secondary"
          fill="text"
          disabled={busy}
          onClick={() => submitAs('draft')}
        >
          {t('library.wizard.saveDraft')}
        </Button>
        <Stack gap={1}>
          <Button type="button" variant="secondary" icon="arrow-left" onClick={onBack}>
            {t('common.actions.back')}
          </Button>
          <Button
            type="button"
            variant="secondary"
            icon="check"
            disabled={busy}
            onClick={() => submitAs('ready')}
          >
            {t('library.wizard.markReady')}
          </Button>
          <Button
            type="button"
            icon="play"
            disabled={busy || fits === false}
            onClick={() => submitAs('launch')}
          >
            {t('library.wizard.launchNow')}
          </Button>
        </Stack>
      </div>
    </form>
  )
}
