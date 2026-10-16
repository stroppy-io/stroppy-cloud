import { adminMutations, adminQueries } from '@api/queries/admin'
import { catalogQueries } from '@api/queries/catalog'
import { toast } from '@app/Toaster'
import { CodeEditorLazy } from '@components/code/CodeEditorLazy'
import { RelativeTime } from '@components/RelativeTime'
import { ServerErrorAlert } from '@components/ServerErrorAlert'
import { type Schema, type Values, validateLocal } from '@components/schema/engine'
import { SchemaForm } from '@components/schema/SchemaForm'
import { css } from '@emotion/css'
import {
  Alert,
  Button,
  Field,
  FieldSet,
  LoadingPlaceholder,
  RadioButtonGroup,
  Stack,
  Switch,
  Text,
  useStyles2,
} from '@grafana/ui'
import { applyServerErrors, REWARD_EARLY } from '@helpers/form'
import { useForm } from '@tanstack/react-form'
import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

const CATALOG_SCHEMA = 'system.stroppy_catalog'

const getStyles = () => ({
  form: css({ maxWidth: 820 }),
})

export function AdminSettingsPage() {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const qc = useQueryClient()
  const settings = useSuspenseQuery(adminQueries.settings()).data
  const catalogSchema = useQuery({ ...catalogQueries.schema(CATALOG_SCHEMA), retry: false })
  const schemaBody = catalogSchema.data as Schema | undefined
  const [catalog, setCatalog] = useState<Values>((settings.stroppy_catalog ?? {}) as Values)
  const [catalogText, setCatalogText] = useState(() =>
    JSON.stringify(settings.stroppy_catalog ?? {}, null, 2)
  )
  const [catalogError, setCatalogError] = useState<string>()
  const catalogLocalErrors = useMemo(
    () => (schemaBody ? validateLocal(schemaBody.fields, catalog) : {}),
    [schemaBody, catalog]
  )

  const schema = useMemo(
    () =>
      z.object({
        tenant_creation: z.enum(['anyone', 'admin_only']),
        public_rating_enabled: z.boolean(),
        examples_enabled: z.boolean(),
      }),
    []
  )
  const form = useForm({
    defaultValues: {
      tenant_creation: settings.tenant_creation,
      public_rating_enabled: settings.public_rating_enabled,
      examples_enabled: settings.examples_enabled,
    },
    ...REWARD_EARLY,
    validators: { onDynamic: schema },
    onSubmit: async ({ value, formApi }) => {
      let nextCatalog: Values | undefined = catalog
      if (!schemaBody) {
        try {
          nextCatalog = JSON.parse(catalogText) as Values
          setCatalogError(undefined)
        } catch (e) {
          setCatalogError(t('admin.settings.catalogInvalid', { error: (e as Error).message }))
          return
        }
      } else if (Object.keys(catalogLocalErrors).length) {
        setCatalogError(t('common.errors.validation'))
        return
      }
      try {
        const next = await adminMutations.patchSettings({
          tenant_creation: value.tenant_creation,
          public_rating_enabled: value.public_rating_enabled,
          examples_enabled: value.examples_enabled,
          stroppy_catalog: nextCatalog,
        })
        qc.setQueryData(adminQueries.settings().queryKey, next)
        await Promise.all([
          qc.invalidateQueries({ queryKey: ['admin', 'tenants'] }),
          qc.invalidateQueries({ queryKey: ['public'] }),
          qc.invalidateQueries({ queryKey: ['catalog', 'stroppy'] }),
        ])
        formApi.reset(value)
        toast.success(t('admin.settings.saved'))
      } catch (e) {
        applyServerErrors(formApi, e, t('common.errors.generic'))
      }
    },
  })

  return (
    <form
      className={styles.form}
      onSubmit={(e) => {
        e.preventDefault()
        void form.handleSubmit()
      }}
    >
      <Stack direction="column" gap={1}>
        {settings.updated_at && (
          <Text color="secondary" variant="bodySmall">
            {t('common.fields.updated')} <RelativeTime value={settings.updated_at} />
            {settings.updated_by?.display_name ? ` · ${settings.updated_by.display_name}` : ''}
          </Text>
        )}
        <FieldSet label={t('admin.settings.platform')}>
          <form.Field name="tenant_creation">
            {(field) => (
              <Field
                label={t('admin.settings.tenantCreation')}
                description={t('admin.settings.tenantCreationHint')}
              >
                <RadioButtonGroup
                  options={[
                    { label: t('admin.settings.tenantCreationAnyone'), value: 'anyone' as const },
                    {
                      label: t('admin.settings.tenantCreationAdmin'),
                      value: 'admin_only' as const,
                    },
                  ]}
                  value={field.state.value}
                  onChange={(v) => field.handleChange(v)}
                />
              </Field>
            )}
          </form.Field>
          <form.Field name="public_rating_enabled">
            {(field) => (
              <Field
                label={t('admin.settings.publicRating')}
                description={t('admin.settings.publicRatingHint')}
                htmlFor={field.name}
              >
                <Switch
                  id={field.name}
                  value={field.state.value}
                  onChange={(e) => field.handleChange(e.currentTarget.checked)}
                />
              </Field>
            )}
          </form.Field>
          <form.Field name="examples_enabled">
            {(field) => (
              <Field
                label={t('admin.settings.examples')}
                description={t('admin.settings.examplesHint')}
                htmlFor={field.name}
              >
                <Switch
                  id={field.name}
                  value={field.state.value}
                  onChange={(e) => field.handleChange(e.currentTarget.checked)}
                />
              </Field>
            )}
          </form.Field>
        </FieldSet>

        <FieldSet label={t('admin.settings.catalog')}>
          <Text color="secondary" variant="bodySmall">
            {t('admin.settings.catalogHint')}
          </Text>
          {catalogSchema.isPending && <LoadingPlaceholder text={t('common.misc.loading')} />}
          {schemaBody ? (
            <SchemaForm
              schema={schemaBody}
              value={catalog}
              errors={catalogLocalErrors}
              onChange={(v) => {
                setCatalog(v)
                setCatalogError(undefined)
              }}
            />
          ) : (
            !catalogSchema.isPending && (
              <CodeEditorLazy
                value={catalogText}
                language="json"
                height={320}
                readOnly={false}
                onBlur={(v) => {
                  setCatalogText(v)
                  try {
                    JSON.parse(v)
                    setCatalogError(undefined)
                  } catch (e) {
                    setCatalogError(
                      t('admin.settings.catalogInvalid', { error: (e as Error).message })
                    )
                  }
                }}
              />
            )
          )}
          {catalogError && <Alert severity="error" title={catalogError} />}
        </FieldSet>

        <form.Subscribe selector={(s) => [s.isSubmitting, s.errorMap.onServer] as const}>
          {([isSubmitting, serverError]) => (
            <Stack direction="column" gap={1}>
              <ServerErrorAlert error={serverError} />
              <Stack gap={1}>
                <Button type="submit" icon="save" disabled={isSubmitting}>
                  {isSubmitting ? t('common.misc.saving') : t('common.actions.save')}
                </Button>
                <Button
                  type="button"
                  variant="secondary"
                  onClick={() => {
                    form.reset()
                    setCatalog((settings.stroppy_catalog ?? {}) as Values)
                    setCatalogText(JSON.stringify(settings.stroppy_catalog ?? {}, null, 2))
                    setCatalogError(undefined)
                  }}
                >
                  {t('common.actions.reset')}
                </Button>
              </Stack>
            </Stack>
          )}
        </form.Subscribe>
      </Stack>
    </form>
  )
}
