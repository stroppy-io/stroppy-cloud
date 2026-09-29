import { isApiError } from '@api/errors'
import { testQueries } from '@api/queries/library'
import type { Schemas } from '@api/types'
import { TagsEditor } from '@components/TagsEditor'
import {
  Alert,
  Button,
  Checkbox,
  Field,
  FieldSet,
  Input,
  MultiCombobox,
  Stack,
  TextArea,
} from '@grafana/ui'
import { firstError } from '@helpers/form'
import { useTenant } from '@hooks/useTenant'
import { useForm } from '@tanstack/react-form'
import { useQuery } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

type Suite = Schemas['Suite']

export interface SuiteFormValues {
  name: string
  description: string
  tags: Record<string, string>
  testIds: string[]
  concurrency: number
  keep: string
  ratingTenant: boolean
  ratingGlobal: boolean
}

export function suiteFormDefaults(suite?: Suite): SuiteFormValues {
  return {
    name: suite?.name ?? '',
    description: suite?.description ?? '',
    tags: suite?.tags ?? {},
    testIds: (suite?.tests ?? []).flatMap((x) => ('ref' in x ? [x.ref.id] : [])),
    concurrency: suite?.concurrency ?? 2,
    keep: suite?.defaults?.keep && suite.defaults.keep !== '0s' ? suite.defaults.keep : '',
    ratingTenant: suite?.defaults?.rating?.tenant ?? true,
    ratingGlobal: suite?.defaults?.rating?.global ?? false,
  }
}

// Form values → SuiteWrite (the spec-level fields; axes/cells are edited on their own tabs).
export function toSuiteWrite(v: SuiteFormValues, base?: Suite): Schemas['SuiteWrite'] {
  return {
    name: v.name.trim().replace(/\s+/g, ' '),
    description: v.description.trim() || undefined,
    tags: Object.keys(v.tags).length ? v.tags : undefined,
    tests: v.testIds.map((id) => ({ ref: { id } })),
    axes: base?.axes ?? {},
    cells: base?.cells,
    concurrency: v.concurrency,
    defaults: {
      keep: v.keep.trim() || undefined,
      rating: { tenant: v.ratingTenant, global: v.ratingGlobal },
    },
  }
}

// Name / description / tags / tests / concurrency / defaults. Used by the create page and the
// "Edit details" modal. Domain checks (test existence, quotas) stay on the server.
export function SuiteForm({
  initial,
  submitLabel,
  onSubmit,
  onCancel,
  showTests = true,
}: {
  initial: SuiteFormValues
  submitLabel: string
  onSubmit: (values: SuiteFormValues) => Promise<unknown>
  onCancel?: () => void
  showTests?: boolean
}) {
  const { t } = useTranslation()
  const { slug } = useTenant()
  const tests = useQuery(testQueries.options(slug))
  const [serverError, setServerError] = useState<string | undefined>()

  const schema = useMemo(
    () =>
      z.object({
        name: z
          .string()
          .trim()
          .min(1, t('suites.form.errors.name'))
          .max(120, t('suites.form.errors.name')), // SuiteWrite.name
        description: z.string(),
        tags: z.record(z.string(), z.string()),
        testIds: showTests
          ? z.array(z.string()).min(1, t('suites.form.errors.tests'))
          : z.array(z.string()),
        concurrency: z
          .number()
          .int()
          .min(1, t('suites.form.errors.concurrency'))
          .max(16, t('suites.form.errors.concurrency')), // SuiteSpec.concurrency
        keep: z
          .string()
          .trim()
          .regex(/^(\d+(h|m|s))?$/, t('suites.form.errors.keep')), // Go duration
        ratingTenant: z.boolean(),
        ratingGlobal: z.boolean(),
      }),
    [t, showTests]
  )

  const form = useForm({
    defaultValues: initial,
    validators: { onBlur: schema, onSubmit: schema },
    onSubmit: async ({ value, formApi }) => {
      setServerError(undefined)
      try {
        await onSubmit(value)
      } catch (e) {
        if (isApiError(e) && e.validation?.errors?.length) {
          const fields: Record<string, string> = {}
          for (const issue of e.validation.errors) {
            const path =
              issue.path === 'defaults.keep'
                ? 'keep'
                : issue.path === 'tests'
                  ? 'testIds'
                  : issue.path
            fields[path] = issue.message ?? issue.code
          }
          formApi.setErrorMap({ onServer: { fields, form: e.detail } as never })
        } else setServerError(isApiError(e) ? (e.detail ?? e.title) : String(e))
      }
    },
  })

  const testOptions = useMemo(
    () =>
      (tests.data?.data ?? []).map((x) => ({
        label: x.name,
        value: x.id,
        description: [x.summary?.db_kind, x.summary?.db_version, x.summary?.topology_label]
          .filter(Boolean)
          .join(' · '),
      })),
    [tests.data]
  )
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        void form.handleSubmit()
      }}
      noValidate
    >
      <form.Field name="name">
        {(field) => (
          <Field
            label={t('suites.form.name')}
            htmlFor={field.name}
            required
            invalid={field.state.meta.errors.length > 0}
            error={firstError(field.state.meta)}
          >
            <Input
              id={field.name}
              value={field.state.value}
              placeholder={t('suites.form.namePlaceholder')}
              onBlur={field.handleBlur}
              onChange={(e) => field.handleChange(e.currentTarget.value)}
              autoFocus
            />
          </Field>
        )}
      </form.Field>
      <form.Field name="description">
        {(field) => (
          <Field label={t('suites.form.description')} htmlFor={field.name}>
            <TextArea
              id={field.name}
              rows={3}
              value={field.state.value}
              placeholder={t('suites.form.descriptionPlaceholder')}
              onBlur={field.handleBlur}
              onChange={(e) => field.handleChange(e.currentTarget.value)}
            />
          </Field>
        )}
      </form.Field>
      <form.Field name="tags">
        {(field) => (
          <Field label={t('suites.form.tags')} htmlFor={field.name}>
            <TagsEditor
              value={field.state.value}
              onChange={field.handleChange}
              placeholder={t('suites.form.tagsPlaceholder')}
            />
          </Field>
        )}
      </form.Field>
      {showTests && (
        <form.Field name="testIds">
          {(field) => (
            <Field
              label={t('suites.form.tests')}
              description={t('suites.form.testsHint')}
              htmlFor={field.name}
              required
              invalid={field.state.meta.errors.length > 0}
              error={firstError(field.state.meta)}
            >
              <MultiCombobox
                id={field.name}
                options={testOptions}
                value={field.state.value}
                placeholder={t('suites.form.testsPlaceholder')}
                loading={tests.isPending}
                onBlur={field.handleBlur}
                onChange={(opts) => field.handleChange(opts.map((o) => o.value))}
              />
            </Field>
          )}
        </form.Field>
      )}
      <FieldSet label={t('suites.overview.defaults')}>
        <Stack gap={2} wrap="wrap">
          <form.Field name="concurrency">
            {(field) => (
              <Field
                label={t('suites.form.concurrency')}
                description={t('suites.form.concurrencyHint')}
                htmlFor={field.name}
                invalid={field.state.meta.errors.length > 0}
                error={firstError(field.state.meta)}
              >
                <Input
                  id={field.name}
                  type="number"
                  min={1}
                  max={16}
                  width={12}
                  value={String(field.state.value)}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(Number(e.currentTarget.value))}
                />
              </Field>
            )}
          </form.Field>
          <form.Field name="keep">
            {(field) => (
              <Field
                label={t('suites.form.keep')}
                description={t('suites.form.keepHint')}
                htmlFor={field.name}
                invalid={field.state.meta.errors.length > 0}
                error={firstError(field.state.meta)}
              >
                <Input
                  id={field.name}
                  width={16}
                  placeholder="2h"
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.currentTarget.value)}
                />
              </Field>
            )}
          </form.Field>
        </Stack>
        <Field label={t('suites.form.rating')}>
          <Stack gap={2}>
            <form.Field name="ratingTenant">
              {(field) => (
                <Checkbox
                  id={field.name}
                  label={t('suites.form.ratingTenant')}
                  value={field.state.value}
                  onChange={(e) => field.handleChange(e.currentTarget.checked)}
                />
              )}
            </form.Field>
            <form.Field name="ratingGlobal">
              {(field) => (
                <Checkbox
                  id={field.name}
                  label={t('suites.form.ratingGlobal')}
                  value={field.state.value}
                  onChange={(e) => field.handleChange(e.currentTarget.checked)}
                />
              )}
            </form.Field>
          </Stack>
        </Field>
      </FieldSet>
      {serverError && <Alert severity="error" title={serverError} />}
      <form.Subscribe selector={(s) => [s.isSubmitting, s.errorMap.onServer] as const}>
        {([isSubmitting, onServer]) => (
          <Stack direction="column" gap={1}>
            {typeof onServer === 'string' && <Alert severity="error" title={onServer} />}
            <Stack gap={1} justifyContent="flex-end">
              {onCancel && (
                <Button type="button" variant="secondary" fill="outline" onClick={onCancel}>
                  {t('common.actions.cancel')}
                </Button>
              )}
              <Button
                type="submit"
                disabled={isSubmitting}
                icon={isSubmitting ? 'spinner' : undefined}
              >
                {submitLabel}
              </Button>
            </Stack>
          </Stack>
        )}
      </form.Subscribe>
    </form>
  )
}
