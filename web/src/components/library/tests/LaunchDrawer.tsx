import { isApiError } from '@api/errors'
import { catalogQueries } from '@api/queries/catalog'
import { keys } from '@api/queries/keys'
import { libraryMutations } from '@api/queries/library'
import { providerQueries } from '@api/queries/settings'
import type { LaunchOverrides, Schemas, TestValidation } from '@api/types'
import { toast } from '@app/Toaster'
import { TagsEditor } from '@components/TagsEditor'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import {
  Alert,
  Button,
  Combobox,
  Drawer,
  Field,
  Input,
  Stack,
  Switch,
  Text,
  useStyles2,
} from '@grafana/ui'
import { useTenant } from '@hooks/useTenant'
import { useForm, useStore } from '@tanstack/react-form'
import { useDebouncedCallback } from '@tanstack/react-pacer'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { applyServerErrors, firstError, serverFormError } from '../shared/form'
import { SizesEditor } from '../shared/SizesEditor'
import { ValidationIssues } from '../shared/ValidationIssues'

type Test = Schemas['Test']
type RoleSizes = Schemas['RoleSizes']

const getStyles = (theme: GrafanaTheme2) => ({
  section: css({
    paddingBottom: theme.spacing(2),
    marginBottom: theme.spacing(2),
    borderBottom: `1px solid ${theme.colors.border.weak}`,
  }),
  footer: css({
    display: 'flex',
    justifyContent: 'flex-end',
    gap: theme.spacing(1),
    paddingTop: theme.spacing(2),
  }),
  estimate: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
    color: theme.colors.text.secondary,
  }),
})

// Go-duration ("30m", "2h", "1h30m") or "0s"/empty for "do not keep".
export const KEEP_RE = /^(\d+h)?(\d+m)?(\d+s)?$/

// Launch a test with overrides: provider, sizes per role, keep, name, labels, rating. Validates live.
export function LaunchDrawer({
  test,
  isOpen,
  onClose,
}: {
  test: Test
  isOpen: boolean
  onClose: () => void
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const { slug } = useTenant()
  const qc = useQueryClient()
  const navigate = useNavigate()
  const providers = useQuery(providerQueries.list(slug))
  const catalogProviders = useQuery(catalogQueries.providers())
  const [sizes, setSizes] = useState<RoleSizes>(test.sizes ?? {})
  const [validation, setValidation] = useState<TestValidation | undefined>()
  const runCount = test.summary?.run_count ?? 0

  const schema = useMemo(
    () =>
      z.object({
        // OpenAPI LaunchOverrides
        name: z
          .string()
          .trim()
          .max(120, t('common.validation.max', { max: 120 })),
        provider_profile_id: z.string().min(1, t('library.launch.providerRequired')),
        keep: z.string().trim().regex(KEEP_RE, t('common.validation.duration')),
        labels: z.record(z.string(), z.string()),
        rating_tenant: z.boolean(),
        rating_global: z.boolean(),
      }),
    [t]
  )
  const launch = useMutation({
    mutationFn: (body: LaunchOverrides) => libraryMutations.launchTest(slug, test.id, body),
  })
  const form = useForm({
    defaultValues: {
      name: `${test.name} #${runCount + 1}`,
      provider_profile_id: test.provider_profile_id ?? '',
      keep: test.keep ?? '0s',
      labels: {} as Record<string, string>,
      rating_tenant: test.rating?.tenant ?? true,
      rating_global: test.rating?.global ?? false,
    },
    validators: { onSubmit: schema },
    onSubmit: async ({ value, formApi }) => {
      try {
        const run = await launch.mutateAsync({
          name: value.name.trim() || undefined,
          provider_profile_id: value.provider_profile_id,
          sizes,
          keep: value.keep.trim() || '0s',
          labels: Object.keys(value.labels).length ? value.labels : undefined,
          rating: { tenant: value.rating_tenant, global: value.rating_global },
        })
        void qc.invalidateQueries({ queryKey: keys.t(slug) })
        toast.success(t('library.launch.started', { name: run.name }))
        onClose()
        void navigate({ to: '/t/$slug/runs/$id', params: { slug, id: run.id } })
      } catch (e) {
        applyServerErrors(formApi, e)
      }
    },
  })

  // live fit check with the overrides applied
  const validate = useMutation({
    mutationFn: (spec: Schemas['TestWrite']) => libraryMutations.validateTest(slug, spec),
    onSuccess: setValidation,
  })
  const runValidate = useDebouncedCallback((spec: Schemas['TestWrite']) => validate.mutate(spec), {
    wait: 400,
  })
  const providerId = useStore(form.store, (s) => s.values.provider_profile_id)
  const keep = useStore(form.store, (s) => s.values.keep)
  const serialized = JSON.stringify({ sizes, providerId, keep })
  // biome-ignore lint/correctness/useExhaustiveDependencies: validate when overrides change
  useEffect(() => {
    if (!isOpen) return
    runValidate({
      name: test.name,
      database: test.database,
      workload: test.workload,
      sizes,
      provider_profile_id: providerId || null,
      keep: KEEP_RE.test(keep) ? keep || '0s' : undefined,
      rating: test.rating,
    })
  }, [serialized, isOpen, runValidate])

  const readyProviders = (providers.data?.data ?? []).filter((p) => p.status === 'ready')
  const profile = providers.data?.data.find((p) => p.id === providerId)
  const catProvider =
    catalogProviders.data?.data.find((c) => c.kind === profile?.kind) ??
    catalogProviders.data?.data[0]
  const issues = validation?.validation.issues ?? []
  const blocking = issues.filter((i) => i.severity === 'ERROR')
  const machines = validation?.estimated?.machines ?? []
  const totalCpu = machines.reduce((a, m) => a + (m.cpu ?? 0) * (m.count ?? 0), 0)
  const totalMem = machines.reduce((a, m) => a + (m.memory_gb ?? 0) * (m.count ?? 0), 0)
  const launchErr = launch.error

  if (!isOpen) return null
  return (
    <Drawer
      title={t('library.launch.title', { name: test.name })}
      subtitle={t('library.launch.subtitle')}
      size="md"
      onClose={onClose}
    >
      <form
        onSubmit={(e) => {
          e.preventDefault()
          void form.handleSubmit()
        }}
      >
        <div className={styles.section}>
          <form.Field name="name">
            {(field) => {
              const err = firstError(field.state.meta)
              return (
                <Field
                  label={t('library.launch.runName')}
                  invalid={!!err}
                  error={err}
                  htmlFor={field.name}
                >
                  <Input
                    id={field.name}
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.currentTarget.value)}
                  />
                </Field>
              )
            }}
          </form.Field>
          <form.Field name="provider_profile_id">
            {(field) => {
              const err = firstError(field.state.meta)
              return (
                <Field
                  label={t('common.fields.provider')}
                  required
                  invalid={!!err}
                  error={err}
                  htmlFor={field.name}
                  description={t('library.launch.providerHint')}
                >
                  <Combobox
                    id={field.name}
                    loading={providers.isPending}
                    placeholder={t('library.launch.pickProvider')}
                    options={readyProviders.map((p) => ({
                      label: p.name,
                      value: p.id,
                      description: p.kind,
                    }))}
                    value={field.state.value || null}
                    onChange={(o) => field.handleChange(o.value)}
                  />
                </Field>
              )
            }}
          </form.Field>
        </div>

        <div className={styles.section}>
          <Field label={t('common.fields.sizes')} description={t('library.launch.sizesHint')}>
            <SizesEditor
              value={sizes}
              onChange={setSizes}
              requirements={validation?.requirements ?? test.requirements}
              sizeTables={catProvider?.sizes}
              diskTypes={catProvider?.disk_types}
              issues={issues}
            />
          </Field>
          {machines.length > 0 && (
            <div className={styles.estimate}>
              {t('library.launch.estimate', {
                machines: machines.reduce((a, m) => a + (m.count ?? 0), 0),
                cpu: totalCpu,
                mem: totalMem,
              })}
            </div>
          )}
        </div>

        <div className={styles.section}>
          <Stack gap={3} wrap="wrap">
            <form.Field name="keep">
              {(field) => {
                const err = firstError(field.state.meta)
                return (
                  <Field
                    label={t('library.launch.keep')}
                    invalid={!!err}
                    error={err}
                    htmlFor={field.name}
                    description={t('library.launch.keepHint')}
                  >
                    <Input
                      id={field.name}
                      width={16}
                      placeholder="0s"
                      value={field.state.value}
                      onBlur={field.handleBlur}
                      onChange={(e) => field.handleChange(e.currentTarget.value)}
                    />
                  </Field>
                )
              }}
            </form.Field>
            <form.Field name="rating_tenant">
              {(field) => (
                <Field label={t('library.tests.ratingTenant')} htmlFor={field.name}>
                  <Switch
                    id={field.name}
                    value={field.state.value}
                    onChange={(e) => field.handleChange(e.currentTarget.checked)}
                  />
                </Field>
              )}
            </form.Field>
            <form.Field name="rating_global">
              {(field) => (
                <Field label={t('library.tests.ratingGlobal')} htmlFor={field.name}>
                  <Switch
                    id={field.name}
                    value={field.state.value}
                    onChange={(e) => field.handleChange(e.currentTarget.checked)}
                  />
                </Field>
              )}
            </form.Field>
          </Stack>
          <form.Field name="labels">
            {(field) => (
              <Field label={t('common.fields.labels')} description={t('library.launch.labelsHint')}>
                <TagsEditor
                  value={field.state.value}
                  onChange={(v) => field.handleChange(v)}
                  placeholder="team=core"
                />
              </Field>
            )}
          </form.Field>
        </div>

        <div className={styles.section}>
          <Text weight="medium">{t('library.validation.title')}</Text>
          <div style={{ marginTop: 8 }}>
            <ValidationIssues
              issues={issues}
              compact
              onApplySuggested={(i) => {
                const role = i.path.split('.')[1]
                if (!role) return
                if (i.path.endsWith('.disk.gb'))
                  setSizes({
                    ...sizes,
                    [role]: {
                      ...sizes[role],
                      size: sizes[role]?.size ?? 'M',
                      disk: { ...sizes[role]?.disk, gb: Number(i.suggested) },
                    },
                  })
                else
                  setSizes({
                    ...sizes,
                    [role]: { ...sizes[role], size: String(i.suggested) as Schemas['Size'] },
                  })
              }}
            />
          </div>
        </div>

        {launchErr && (
          <Alert
            severity="error"
            title={isApiError(launchErr) ? launchErr.title : t('common.errors.generic')}
          >
            {isApiError(launchErr) ? launchErr.detail : String(launchErr)}
            {isApiError(launchErr) && launchErr.validation?.errors?.length ? (
              <ul>
                {launchErr.validation.errors.map((e) => (
                  <li key={`${e.path}-${e.code}`}>
                    <code>{e.path}</code>: {e.message ?? e.code}
                  </li>
                ))}
              </ul>
            ) : null}
          </Alert>
        )}
        <form.Subscribe
          selector={(s) => [s.isSubmitting, serverFormError(s.errorMap.onServer)] as const}
        >
          {([submitting, formError]) => (
            <div className={styles.footer}>
              {formError && !launchErr && <Text color="error">{formError}</Text>}
              <Button type="button" variant="secondary" onClick={onClose}>
                {t('common.actions.cancel')}
              </Button>
              <Button type="submit" icon="play" disabled={submitting || validate.isPending}>
                {blocking.length ? t('library.launch.launchAnyway') : t('common.actions.launch')}
              </Button>
            </div>
          )}
        </form.Subscribe>
      </form>
    </Drawer>
  )
}
