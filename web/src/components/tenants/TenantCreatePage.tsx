import { tenantMutations, tenantQueries } from '@api/queries/tenants'
import { AppLink } from '@app/AppLink'
import { Page } from '@app/Page'
import { PageHeader } from '@app/PageHeader'
import { toast } from '@app/Toaster'
import { ServerErrorAlert } from '@components/ServerErrorAlert'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Alert, Button, Field, Icon, Input, Stack, Text, TextArea, useStyles2 } from '@grafana/ui'
import { applyServerErrors, firstError, SLUG_RE, slugify } from '@helpers/form'
import { useMe } from '@hooks/useMe'
import { revalidateLogic, useForm } from '@tanstack/react-form'
import { useDebouncedCallback } from '@tanstack/react-pacer'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

const getStyles = (theme: GrafanaTheme2) => ({
  card: css({
    maxWidth: 640,
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    padding: theme.spacing(3),
  }),
  availability: css({ display: 'inline-flex', alignItems: 'center', gap: theme.spacing(0.5) }),
})

export function TenantCreatePage() {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const me = useMe()
  const qc = useQueryClient()
  const navigate = useNavigate()
  const owned = me.tenants.find((m) => m.tenant.id === me.owned_tenant_id)?.tenant
  const [slugTouched, setSlugTouched] = useState(false)
  const [suggestFor, setSuggestFor] = useState<string | undefined>(undefined)
  const suggestion = useQuery({ ...tenantQueries.suggestName(suggestFor), enabled: !owned })
  const debouncedSuggest = useDebouncedCallback(
    (name: string) => setSuggestFor(name || undefined),
    {
      wait: 350,
    }
  )

  const schema = useMemo(
    () =>
      z.object({
        // TenantCreate.name
        name: z
          .string()
          .trim()
          .min(2, t('common.validation.min', { min: 2 }))
          .max(64),
        // TenantCreate.slug — lowercase, digits, dashes
        slug: z.string().trim().regex(SLUG_RE, t('common.validation.slug')),
        description: z
          .string()
          .trim()
          .max(500, t('common.validation.max', { max: 500 })),
      }),
    [t]
  )
  const form = useForm({
    defaultValues: { name: '', slug: '', description: '' },
    validationLogic: revalidateLogic({ mode: 'blur', modeAfterSubmission: 'change' }),
    validators: { onDynamic: schema },
    onSubmit: async ({ value, formApi }) => {
      try {
        const tenant = await tenantMutations.create({
          name: value.name.trim(),
          slug: value.slug.trim(),
          description: value.description.trim() || undefined,
        })
        await qc.invalidateQueries({ queryKey: ['me'] })
        toast.success(t('tenants.create.created', { name: tenant.name }))
        void navigate({ to: '/t/$slug', params: { slug: tenant.slug } })
      } catch (e) {
        applyServerErrors(formApi, e, t('common.errors.generic'))
      }
    },
  })

  // The server's suggestion answers "is this slug free?" for the current name/slug.
  const suggested = suggestion.data as
    | { name: string; slug: string; available?: boolean }
    | undefined

  return (
    <Page width="narrow">
      <PageHeader
        title={t('tenants.create.title')}
        subtitle={t('tenants.create.subtitle')}
        icon="building"
        breadcrumbs={
          owned ? [{ label: owned.name, to: '/t/$slug', params: { slug: owned.slug } }] : undefined
        }
      />
      {owned ? (
        <Alert severity="info" title={t('tenants.create.ownedTitle')}>
          <Stack direction="column" gap={1}>
            <span>{t('tenants.create.ownedBody', { name: owned.name })}</span>
            <span>
              <AppLink to="/t/$slug" params={{ slug: owned.slug }}>
                {t('tenants.create.openOwned', { name: owned.name })}
              </AppLink>
            </span>
          </Stack>
        </Alert>
      ) : (
        <div className={styles.card}>
          <form
            onSubmit={(e) => {
              e.preventDefault()
              void form.handleSubmit()
            }}
          >
            <Stack direction="column" gap={1}>
              {me.tenants.length === 0 && (
                <Alert severity="info" title={t('tenants.create.firstTitle')}>
                  {t('tenants.create.firstBody')}
                </Alert>
              )}
              <form.Field
                name="name"
                listeners={{
                  onChange: ({ value }) => {
                    if (!slugTouched) form.setFieldValue('slug', slugify(value))
                    debouncedSuggest(value)
                  },
                }}
              >
                {(field) => (
                  <Field
                    label={t('common.fields.name')}
                    description={t('tenants.create.nameHint')}
                    invalid={!!firstError(field.state.meta)}
                    error={firstError(field.state.meta)}
                    htmlFor={field.name}
                    required
                  >
                    <Input
                      id={field.name}
                      autoFocus
                      placeholder={suggested?.name ?? t('tenants.create.namePlaceholder')}
                      value={field.state.value}
                      onBlur={field.handleBlur}
                      onChange={(e) => field.handleChange(e.currentTarget.value)}
                    />
                  </Field>
                )}
              </form.Field>
              {suggested && !form.state.values.name && (
                <Text color="secondary" variant="bodySmall">
                  {t('tenants.create.suggestion')}{' '}
                  <Button
                    size="sm"
                    variant="secondary"
                    fill="text"
                    type="button"
                    onClick={() => {
                      form.setFieldValue('name', suggested.name)
                      form.setFieldValue('slug', suggested.slug)
                    }}
                  >
                    {suggested.name}
                  </Button>
                </Text>
              )}
              <form.Field
                name="slug"
                listeners={{
                  onChange: ({ value }) => {
                    setSlugTouched(true)
                    debouncedSuggest(value)
                  },
                  onBlur: ({ value, fieldApi }) => fieldApi.setValue(slugify(value)),
                }}
              >
                {(field) => {
                  const err = firstError(field.state.meta)
                  const known = suggested && slugify(suggestFor ?? '') === field.state.value
                  const taken = known && suggested.slug !== field.state.value
                  return (
                    <Field
                      label={t('tenants.create.slug')}
                      description={t('tenants.create.slugHint')}
                      invalid={!!err || !!taken}
                      error={
                        err ??
                        (taken ? t('tenants.create.slugTaken', { alt: suggested.slug }) : undefined)
                      }
                      htmlFor={field.name}
                      required
                    >
                      <Input
                        id={field.name}
                        prefix="/t/"
                        value={field.state.value}
                        onBlur={field.handleBlur}
                        onChange={(e) => field.handleChange(e.currentTarget.value)}
                        suffix={
                          field.state.value && known && !taken ? (
                            <span className={styles.availability}>
                              <Icon name="check" />
                              <Text color="success" variant="bodySmall">
                                {t('tenants.create.available')}
                              </Text>
                            </span>
                          ) : suggestion.isFetching && field.state.value ? (
                            <Icon name="sync" />
                          ) : undefined
                        }
                      />
                    </Field>
                  )
                }}
              </form.Field>
              <form.Field name="description">
                {(field) => (
                  <Field
                    label={t('common.fields.description')}
                    invalid={!!firstError(field.state.meta)}
                    error={firstError(field.state.meta)}
                    htmlFor={field.name}
                  >
                    <TextArea
                      id={field.name}
                      rows={3}
                      placeholder={t('tenants.create.descriptionPlaceholder')}
                      value={field.state.value}
                      onBlur={field.handleBlur}
                      onChange={(e) => field.handleChange(e.currentTarget.value)}
                    />
                  </Field>
                )}
              </form.Field>
              <Text color="secondary" variant="bodySmall">
                {t('tenants.create.whatHappens')}
              </Text>
              <form.Subscribe selector={(s) => [s.isSubmitting, s.errorMap.onServer] as const}>
                {([isSubmitting, serverError]) => (
                  <Stack direction="column" gap={1}>
                    <ServerErrorAlert error={serverError} />
                    <Stack gap={1}>
                      <Button type="submit" icon="building" disabled={isSubmitting}>
                        {isSubmitting ? t('common.misc.saving') : t('tenants.create.cta')}
                      </Button>
                      {me.tenants.length > 0 && (
                        <Button
                          type="button"
                          variant="secondary"
                          onClick={() => void navigate({ to: '/' })}
                        >
                          {t('common.actions.cancel')}
                        </Button>
                      )}
                    </Stack>
                  </Stack>
                )}
              </form.Subscribe>
            </Stack>
          </form>
        </div>
      )}
    </Page>
  )
}
