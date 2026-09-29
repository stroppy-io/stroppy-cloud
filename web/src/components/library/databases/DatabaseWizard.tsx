import { catalogQueries } from '@api/queries/catalog'
import { keys } from '@api/queries/keys'
import { libraryMutations } from '@api/queries/library'
import type { CatalogDatabase, DatabaseWrite, Schemas } from '@api/types'
import { toast } from '@app/Toaster'
import { applyDefaults, type Schema, type Values } from '@components/schema/engine'
import { SchemaForm } from '@components/schema/SchemaForm'
import { TagsEditor } from '@components/TagsEditor'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import {
  Badge,
  Button,
  Card,
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
import { useSchemaValidate } from '@hooks/useSchemaValidate'
import { useTenant } from '@hooks/useTenant'
import { useForm } from '@tanstack/react-form'
import { useDebouncedCallback } from '@tanstack/react-pacer'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { applyServerErrors, firstError, serverFormError } from '../shared/form'
import { Panel } from '../shared/Panel'
import { RequirementsTable } from '../shared/RequirementsTable'
import { TopologyPreview } from '../shared/TopologyPreview'

type Database = Schemas['Database']
type Preview = Schemas['DatabasePreview']

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
  cards: css({
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))',
    gap: theme.spacing(1),
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
  mono: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
  }),
})

export interface DatabaseWizardResult {
  database?: Database
  spec?: DatabaseWrite
}

// 4 steps: kind → version + topology template → params (live :preview) → name & create.
// `mode="inline"` returns the spec instead of creating a record (embedded in the test wizard).
export function DatabaseWizard({
  mode,
  onDone,
  onCancel,
  initialKind,
}: {
  mode: 'create' | 'inline'
  onDone: (result: DatabaseWizardResult) => void
  onCancel?: () => void
  initialKind?: string
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const { slug } = useTenant()
  const qc = useQueryClient()
  const catalog = useQuery(catalogQueries.databases())
  const [step, setStep] = useState(initialKind ? 2 : 1)
  const [kind, setKind] = useState<string | undefined>(initialKind)
  const [version, setVersion] = useState<string | undefined>()
  const [template, setTemplate] = useState<string | undefined>()
  const [params, setParams] = useState<Values>({})
  const cat = catalog.data?.data.find((c) => c.kind === kind)
  const schemaId = cat?.params_schema
  const schemaQ = useQuery({ ...catalogQueries.schema(schemaId ?? ''), enabled: !!schemaId })
  const schema = schemaQ.data as Schema | undefined
  const hasVersionField = !!schema?.fields.some((f) => f.name === 'version')
  const fullParams = useMemo(
    () => (schema ? applyDefaults(schema.fields, params) : params),
    [schema, params]
  )
  const { errors, checking } = useSchemaValidate(schemaId, schema, fullParams, step === 3)

  // live topology / requirements preview from the server
  const [preview, setPreview] = useState<Preview | undefined>()
  const previewM = useMutation({
    mutationFn: (body: DatabaseWrite) => libraryMutations.previewDatabase(slug, body),
    onSuccess: setPreview,
  })
  const runPreview = useDebouncedCallback((body: DatabaseWrite) => previewM.mutate(body), {
    wait: 400,
  })
  const serializedParams = JSON.stringify(fullParams)
  // biome-ignore lint/correctness/useExhaustiveDependencies: re-run when the serialized params change
  useEffect(() => {
    if (kind && version && step >= 2)
      runPreview({
        name: 'preview',
        kind: kind as DatabaseWrite['kind'],
        version,
        params: fullParams,
      })
  }, [serializedParams, kind, version, step, runPreview])

  const pickKind = (c: CatalogDatabase) => {
    setKind(c.kind)
    const v = c.versions.find((x) => x.default)?.version ?? c.versions[0]?.version
    setVersion(v)
    const tpl = c.topologies[0]
    setTemplate(tpl?.id)
    setParams({ ...(tpl?.params ?? {}), ...(v ? { version: v } : {}) })
    setPreview(undefined)
    setStep(2)
  }
  const pickTemplate = (id: string) => {
    setTemplate(id)
    const tpl = cat?.topologies.find((x) => x.id === id)
    setParams({ ...(tpl?.params ?? {}), ...(version ? { version } : {}) })
  }
  const pickVersion = (v: string) => {
    setVersion(v)
    setParams((p) => ({ ...p, version: v }))
  }
  const specParams = useMemo(() => {
    const p = { ...fullParams }
    if (!hasVersionField) delete p.version
    return p
  }, [fullParams, hasVersionField])

  const create = useMutation({
    mutationFn: (body: DatabaseWrite) => libraryMutations.createDatabase(slug, body),
  })
  const nameSchema = useMemo(
    () =>
      z.object({
        // OpenAPI DatabaseWrite.name
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
    defaultValues: { name: '', description: '', tags: {} as Record<string, string> },
    validators: { onSubmit: nameSchema },
    onSubmit: async ({ value, formApi }) => {
      if (!kind || !version) return
      const body: DatabaseWrite = {
        name: value.name.trim().replace(/\s+/g, ' '),
        description: value.description.trim() || undefined,
        tags: Object.keys(value.tags).length ? value.tags : undefined,
        kind: kind as DatabaseWrite['kind'],
        version,
        params: specParams,
      }
      if (mode === 'inline') {
        onDone({ spec: body })
        return
      }
      try {
        const db = await create.mutateAsync(body)
        void qc.invalidateQueries({ queryKey: keys.t(slug) })
        toast.success(t('library.databases.created', { name: db.name }))
        onDone({ database: db })
      } catch (e) {
        applyServerErrors(formApi, e)
      }
    },
  })

  const steps = [
    t('library.wizard.db.kind'),
    t('library.wizard.db.version'),
    t('library.wizard.db.params'),
    mode === 'inline' ? t('library.wizard.db.review') : t('library.wizard.db.name'),
  ]
  const canGo = (n: number) => n === 1 || (!!kind && (n <= 3 || !!version))

  return (
    <div>
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
              onClick={clickable ? () => setStep(n) : undefined}
              disabled={!canGo(n)}
              aria-current={n === step ? 'step' : undefined}
            >
              <span className={styles.num}>{n < step ? <Icon name="check" size="xs" /> : n}</span>
              {label}
              {n === 1 && kind && <Badge text={cat?.title ?? kind} color="blue" />}
              {n === 2 && version && step > 2 && (
                <Badge text={`${version} · ${template ?? ''}`} color="darkgrey" />
              )}
            </button>
          )
        })}
      </div>

      {step === 1 && (
        <div>
          {catalog.isPending ? (
            <LoadingPlaceholder text={t('common.misc.loading')} />
          ) : (
            <div className={styles.cards}>
              {(catalog.data?.data ?? []).map((c) => (
                <Card
                  key={c.kind}
                  isSelected={kind === c.kind}
                  onClick={() => pickKind(c)}
                  noMargin
                >
                  <Card.Heading>{c.title}</Card.Heading>
                  <Card.Figure>
                    <Icon name={c.deployable ? 'database' : 'external-link-alt'} size="xl" />
                  </Card.Figure>
                  <Card.Description>
                    {t('library.wizard.db.kindDesc', {
                      versions: c.versions
                        .map((v) => v.version)
                        .slice(0, 4)
                        .join(', '),
                      topologies: c.topologies.length,
                    })}
                  </Card.Description>
                  <Card.Tags>
                    <Stack gap={0.5} wrap="wrap">
                      {c.protocols.map((p) => (
                        <Badge key={p} text={p} color="darkgrey" />
                      ))}
                      {!c.deployable && (
                        <Badge text={t('library.wizard.db.notDeployable')} color="orange" />
                      )}
                    </Stack>
                  </Card.Tags>
                </Card>
              ))}
            </div>
          )}
        </div>
      )}

      {step === 2 && cat && (
        <div className={styles.two}>
          <Stack direction="column" gap={3}>
            <Field
              label={t('common.fields.version')}
              description={t('library.wizard.db.versionHint')}
            >
              <RadioButtonGroup
                options={cat.versions.map((v) => ({
                  label: v.default ? `${v.version} ★` : v.version,
                  value: v.version,
                  description: v.image,
                }))}
                value={version}
                onChange={pickVersion}
              />
            </Field>
            <Field
              label={t('library.wizard.db.template')}
              description={t('library.wizard.db.templateHint')}
            >
              <div className={styles.cards}>
                {cat.topologies.map((tp) => (
                  <Card
                    key={tp.id}
                    isSelected={template === tp.id}
                    onClick={() => pickTemplate(tp.id)}
                    noMargin
                  >
                    <Card.Heading>{tp.title ?? tp.id}</Card.Heading>
                    {tp.description && <Card.Description>{tp.description}</Card.Description>}
                    <Card.Meta>
                      <span className={styles.mono}>
                        {Object.entries(tp.params ?? {})
                          .map(([k, v]) => `${k}=${String(v)}`)
                          .join(' ')}
                      </span>
                    </Card.Meta>
                  </Card>
                ))}
              </div>
            </Field>
          </Stack>
          <PreviewSide preview={preview} pending={previewM.isPending} />
        </div>
      )}

      {step === 3 && cat && (
        <div className={styles.two}>
          <Panel
            title={t('library.wizard.db.paramsTitle', { kind: cat.title })}
            description={
              checking
                ? t('schema.validating')
                : Object.keys(errors).length
                  ? t('common.errors.validation')
                  : t('schema.valid')
            }
          >
            {schema ? (
              <SchemaForm schema={schema} value={fullParams} onChange={setParams} errors={errors} />
            ) : (
              <LoadingPlaceholder text={t('common.misc.loading')} />
            )}
          </Panel>
          <PreviewSide preview={preview} pending={previewM.isPending} />
        </div>
      )}

      {step === 4 && cat && (
        <form
          onSubmit={(e) => {
            e.preventDefault()
            void form.handleSubmit()
          }}
        >
          <div className={styles.two}>
            <Panel
              title={
                mode === 'inline' ? t('library.wizard.db.review') : t('library.wizard.db.nameTitle')
              }
            >
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
                        placeholder={`${kind}-${version}-${template ?? 'custom'}`}
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
                      rows={3}
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
                      placeholder="env=baseline"
                    />
                  </Field>
                )}
              </form.Field>
              <form.Subscribe selector={(s) => serverFormError(s.errorMap.onServer)}>
                {(err) => (err ? <Text color="error">{err}</Text> : null)}
              </form.Subscribe>
            </Panel>
            <PreviewSide preview={preview} pending={previewM.isPending} />
          </div>
        </form>
      )}

      <div className={styles.footer}>
        <Stack gap={1}>
          {onCancel && (
            <Button variant="secondary" fill="text" onClick={onCancel}>
              {t('common.actions.cancel')}
            </Button>
          )}
        </Stack>
        <Stack gap={1}>
          {step > 1 && (
            <Button variant="secondary" icon="arrow-left" onClick={() => setStep(step - 1)}>
              {t('common.actions.back')}
            </Button>
          )}
          {step < 4 && step > 1 && (
            <Button
              onClick={() => setStep(step + 1)}
              disabled={!kind || !version || (step === 3 && Object.keys(errors).length > 0)}
            >
              {t('common.actions.next')}
            </Button>
          )}
          {step === 4 && (
            <form.Subscribe selector={(s) => s.isSubmitting}>
              {(submitting) => (
                <Button
                  icon={mode === 'inline' ? 'check' : 'plus'}
                  disabled={submitting}
                  onClick={() => void form.handleSubmit()}
                >
                  {mode === 'inline'
                    ? t('library.wizard.db.useInline')
                    : t('library.databases.create')}
                </Button>
              )}
            </form.Subscribe>
          )}
        </Stack>
      </div>
    </div>
  )
}

function PreviewSide({ preview, pending }: { preview: Preview | undefined; pending: boolean }) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  return (
    <div className={styles.side}>
      <Panel
        title={t('library.topology.title')}
        description={
          pending ? t('library.wizard.db.previewUpdating') : preview?.topology_preview?.label
        }
        dense
      >
        {preview ? (
          <TopologyPreview topology={preview.topology_preview} compact />
        ) : (
          <LoadingPlaceholder text="" />
        )}
      </Panel>
      <Panel title={t('library.requirements.title')} dense>
        <RequirementsTable requirements={preview?.requirements} />
      </Panel>
      {preview?.validation.errors && preview.validation.errors.length > 0 && (
        <Panel title={t('library.validation.title')} dense>
          <Stack direction="column" gap={0.5}>
            {preview.validation.errors.map((e) => (
              <Text
                key={`${e.path}-${e.code}`}
                color={e.severity === 'WARNING' ? 'warning' : 'error'}
                variant="bodySmall"
              >
                {e.path}: {e.message ?? e.code}
              </Text>
            ))}
          </Stack>
        </Panel>
      )}
    </div>
  )
}
