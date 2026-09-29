import { catalogQueries } from '@api/queries/catalog'
import { keys } from '@api/queries/keys'
import { libraryMutations } from '@api/queries/library'
import type { Protocol, Schemas, WorkloadWrite } from '@api/types'
import { toast } from '@app/Toaster'
import type { Values } from '@components/schema/engine'
import { TagsEditor } from '@components/TagsEditor'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import {
  Button,
  Combobox,
  Field,
  Input,
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
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { applyServerErrors, firstError, serverFormError } from '../shared/form'
import { Panel } from '../shared/Panel'
import { RequirementsTable } from '../shared/RequirementsTable'
import { newSegment, SegmentsEditor } from '../shared/SegmentsEditor'

type Workload = Schemas['Workload']
type Preview = Schemas['WorkloadPreview']

const PROTOCOLS: Protocol[] = [
  'pg',
  'mysql',
  'picodata',
  'ydb_grpc',
  'ydb_grpcs',
  'cockroach',
  'noop',
]

const getStyles = (theme: GrafanaTheme2) => ({
  two: css({
    display: 'grid',
    gridTemplateColumns: 'minmax(0, 1fr) 340px',
    gap: theme.spacing(2),
    alignItems: 'start',
    [theme.breakpoints.down('lg')]: { gridTemplateColumns: 'minmax(0, 1fr)' },
  }),
  footer: css({
    display: 'flex',
    justifyContent: 'flex-end',
    gap: theme.spacing(1),
    marginTop: theme.spacing(3),
    paddingTop: theme.spacing(2),
    borderTop: `1px solid ${theme.colors.border.weak}`,
  }),
})

export interface WorkloadFormResult {
  workload?: Workload
  spec?: WorkloadWrite
}

// New workload: name, stroppy version, protocol, first segment (schema form) with live requirements preview.
// `mode="inline"` returns the spec (embedded in the test wizard) instead of creating a record.
export function WorkloadForm({
  mode,
  onDone,
  onCancel,
  defaultProtocol,
}: {
  mode: 'create' | 'inline'
  onDone: (r: WorkloadFormResult) => void
  onCancel?: () => void
  defaultProtocol?: Protocol
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const { slug } = useTenant()
  const qc = useQueryClient()
  const stroppy = useQuery(catalogQueries.stroppy())
  const defaultVersion =
    stroppy.data?.versions.find((v) => v.default)?.version ??
    stroppy.data?.versions[0]?.version ??
    ''
  const [segments, setSegments] = useState<Values[]>([newSegment(0)])
  const [preview, setPreview] = useState<Preview | undefined>()

  const schema = useMemo(
    () =>
      z.object({
        // OpenAPI WorkloadWrite
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
        stroppy_version: z.string().min(1, t('library.workloads.form.versionRequired')),
        protocol: z.enum(PROTOCOLS as [Protocol, ...Protocol[]]),
      }),
    [t]
  )
  const create = useMutation({
    mutationFn: (body: WorkloadWrite) => libraryMutations.createWorkload(slug, body),
  })
  const form = useForm({
    defaultValues: {
      name: '',
      description: '',
      tags: {} as Record<string, string>,
      stroppy_version: defaultVersion,
      protocol: (defaultProtocol ?? 'pg') as Protocol,
    },
    validators: { onSubmit: schema },
    onSubmit: async ({ value, formApi }) => {
      const body: WorkloadWrite = {
        name: value.name.trim().replace(/\s+/g, ' '),
        description: value.description.trim() || undefined,
        tags: Object.keys(value.tags).length ? value.tags : undefined,
        stroppy_version: value.stroppy_version,
        protocol: value.protocol,
        segments,
        options: {
          connection: { kind: value.protocol.startsWith('ydb') ? 'ydb' : value.protocol },
        },
      }
      if (mode === 'inline') {
        onDone({ spec: body })
        return
      }
      try {
        const wl = await create.mutateAsync(body)
        void qc.invalidateQueries({ queryKey: keys.t(slug) })
        toast.success(t('library.workloads.created', { name: wl.name }))
        onDone({ workload: wl })
      } catch (e) {
        applyServerErrors(formApi, e)
      }
    },
  })
  // the version list arrives async — fill the default once
  useEffect(() => {
    if (defaultVersion && !form.getFieldValue('stroppy_version'))
      form.setFieldValue('stroppy_version', defaultVersion)
  }, [defaultVersion, form])

  const previewM = useMutation({
    mutationFn: (body: WorkloadWrite) => libraryMutations.previewWorkload(slug, body),
    onSuccess: setPreview,
  })
  const runPreview = useDebouncedCallback((body: WorkloadWrite) => previewM.mutate(body), {
    wait: 500,
  })
  const serialized = JSON.stringify(segments)
  // biome-ignore lint/correctness/useExhaustiveDependencies: preview follows the serialized segments
  useEffect(() => {
    runPreview({
      name: 'preview',
      stroppy_version: form.getFieldValue('stroppy_version') || defaultVersion,
      protocol: form.getFieldValue('protocol'),
      segments,
    })
  }, [serialized, runPreview])

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        void form.handleSubmit()
      }}
    >
      <div className={styles.two}>
        <Stack direction="column" gap={2}>
          <Panel title={t('library.workloads.form.general')}>
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
                      placeholder="tpcc-100wh-64vu-10m"
                      value={field.state.value}
                      onBlur={field.handleBlur}
                      onChange={(e) => field.handleChange(e.currentTarget.value)}
                    />
                  </Field>
                )
              }}
            </form.Field>
            <Stack gap={2} wrap="wrap">
              <form.Field name="stroppy_version">
                {(field) => {
                  const err = firstError(field.state.meta)
                  return (
                    <Field
                      label={t('library.columns.stroppy')}
                      required
                      invalid={!!err}
                      error={err}
                      htmlFor={field.name}
                      description={t('library.workloads.form.versionHint')}
                    >
                      <Combobox
                        id={field.name}
                        width={28}
                        loading={stroppy.isPending}
                        options={(stroppy.data?.versions ?? []).map((v) => ({
                          label: v.default ? `${v.version} ★` : v.version,
                          value: v.version,
                          description: v.image,
                        }))}
                        value={field.state.value || null}
                        onChange={(o) => field.handleChange(o.value)}
                      />
                    </Field>
                  )
                }}
              </form.Field>
              <form.Field name="protocol">
                {(field) => {
                  const ver = stroppy.data?.versions.find(
                    (v) => v.version === form.getFieldValue('stroppy_version')
                  )
                  const allowed = ver?.protocols ?? PROTOCOLS
                  return (
                    <Field
                      label={t('library.columns.protocol')}
                      required
                      htmlFor={field.name}
                      description={t('library.workloads.form.protocolHint')}
                    >
                      <RadioButtonGroup
                        id={field.name}
                        options={PROTOCOLS.map((p) => ({
                          label: p,
                          value: p,
                          disabled: !allowed.includes(p),
                        }))}
                        value={field.state.value}
                        onChange={(v) => field.handleChange(v)}
                      />
                    </Field>
                  )
                }}
              </form.Field>
            </Stack>
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
                    placeholder="script=tpcc"
                  />
                </Field>
              )}
            </form.Field>
          </Panel>
          <Panel
            title={t('library.workloads.form.firstSegment')}
            description={t('library.workloads.form.firstSegmentHint')}
          >
            <SegmentsEditor
              segments={segments}
              onChange={setSegments}
              selected={0}
              onSelect={() => undefined}
              single
            />
          </Panel>
          <form.Subscribe selector={(s) => serverFormError(s.errorMap.onServer)}>
            {(err) => (err ? <Text color="error">{err}</Text> : null)}
          </form.Subscribe>
        </Stack>
        <Stack direction="column" gap={2}>
          <Panel
            title={t('library.requirements.title')}
            description={
              previewM.isPending
                ? t('library.wizard.db.previewUpdating')
                : t('library.workloads.form.runnerHint')
            }
            dense
          >
            <RequirementsTable requirements={preview?.requirements} />
          </Panel>
          {preview?.validation.errors && preview.validation.errors.length > 0 && (
            <Panel title={t('library.validation.title')} dense>
              <Stack direction="column" gap={0.5}>
                {preview.validation.errors
                  .filter((e) => e.path !== 'name')
                  .map((e) => (
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
        </Stack>
      </div>
      <div className={styles.footer}>
        {onCancel && (
          <Button type="button" variant="secondary" onClick={onCancel}>
            {t('common.actions.cancel')}
          </Button>
        )}
        <form.Subscribe selector={(s) => s.isSubmitting}>
          {(submitting) => (
            <Button type="submit" icon={mode === 'inline' ? 'check' : 'plus'} disabled={submitting}>
              {mode === 'inline' ? t('library.wizard.wl.useInline') : t('library.workloads.create')}
            </Button>
          )}
        </form.Subscribe>
      </div>
    </form>
  )
}
