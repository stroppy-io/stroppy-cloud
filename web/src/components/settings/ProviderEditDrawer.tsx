import { catalogQueries } from '@api/queries/catalog'
import { settingsMutations } from '@api/queries/settings'
import type { ProviderProfile } from '@api/types'
import { toast } from '@app/Toaster'
import { defaults, type Schema, type Values } from '@components/schema/engine'
import { SchemaForm } from '@components/schema/SchemaForm'
import {
  Alert,
  Button,
  CollapsableSection,
  Drawer,
  Field,
  Input,
  LoadingPlaceholder,
  Stack,
  Switch,
  Text,
} from '@grafana/ui'
import { serverErrors } from '@helpers/form'
import { useSchemaValidate } from '@hooks/useSchemaValidate'
import { useTenant } from '@hooks/useTenant'
import { useMutation, useQuery } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

export function ProviderEditDrawer({
  provider,
  onClose,
  onSaved,
}: {
  provider: ProviderProfile
  onClose: () => void
  onSaved: () => Promise<void>
}) {
  const { t } = useTranslation()
  const { slug } = useTenant()
  const catalog = useQuery(catalogQueries.providers())
  const cp = catalog.data?.data.find((p) => p.kind === provider.kind)
  const settingsSchema = useQuery({
    ...catalogQueries.schema(cp?.settings_schema ?? ''),
    enabled: !!cp?.settings_schema,
  })
  const credentialsSchema = useQuery({
    ...catalogQueries.schema(cp?.credentials_schema ?? ''),
    enabled: !!cp?.credentials_schema,
  })
  const sSchema = settingsSchema.data as Schema | undefined
  const cSchema = credentialsSchema.data as Schema | undefined
  const [name, setName] = useState(provider.name)
  const [nameError, setNameError] = useState<string>()
  const [settings, setSettings] = useState<Values>((provider.settings ?? {}) as Values)
  const [replaceCreds, setReplaceCreds] = useState(false)
  const [credentials, setCredentials] = useState<Values>()
  const [formError, setFormError] = useState<string>()
  const credentialsValue = useMemo(
    () => credentials ?? (cSchema ? defaults(cSchema.fields) : undefined),
    [credentials, cSchema]
  )
  const sValidation = useSchemaValidate(cp?.settings_schema, sSchema, settings)
  const cValidation = useSchemaValidate(
    cp?.credentials_schema,
    cSchema,
    credentialsValue,
    replaceCreds
  )

  const save = useMutation({
    mutationFn: () =>
      settingsMutations.patchProvider(slug, provider.id, {
        name: name.trim(),
        settings,
        ...(replaceCreds ? { credentials: credentialsValue ?? {} } : {}),
      }),
    onSuccess: async () => {
      toast.success(
        replaceCreds ? t('settings.providers.savedReverify') : t('settings.providers.saved')
      )
      await onSaved()
    },
    onError: (e) => {
      const s = serverErrors(e, t('common.errors.generic'))
      setNameError(s.fields.name)
      setFormError(s.form)
    },
  })
  const blocked =
    sValidation.hasErrors || (replaceCreds && cValidation.hasErrors) || name.trim().length < 2

  return (
    <Drawer
      title={t('settings.providers.editTitle', { name: provider.name })}
      subtitle={cp?.title ?? provider.kind}
      size="md"
      onClose={onClose}
    >
      <Stack direction="column" gap={2}>
        <Field
          label={t('settings.providers.name')}
          invalid={!!nameError}
          error={nameError}
          required
        >
          <Input
            value={name}
            onChange={(e) => {
              setName(e.currentTarget.value)
              setNameError(undefined)
            }}
          />
        </Field>
        <Text element="h3" variant="h5">
          {t('settings.providers.step.settings')}
        </Text>
        {settingsSchema.isPending && <LoadingPlaceholder text={t('common.misc.loading')} />}
        {sSchema && (
          <SchemaForm
            schema={sSchema}
            value={settings}
            errors={sValidation.errors}
            onChange={setSettings}
          />
        )}
        <CollapsableSection
          label={t('settings.providers.replaceCredentials')}
          isOpen={replaceCreds}
          onToggle={(open) => setReplaceCreds(open)}
        >
          <Stack direction="column" gap={1}>
            <Field
              label={t('settings.providers.replaceCredentialsSwitch')}
              description={t('settings.providers.replaceCredentialsHint')}
            >
              <Switch
                value={replaceCreds}
                onChange={(e) => setReplaceCreds(e.currentTarget.checked)}
              />
            </Field>
            {replaceCreds && cSchema && credentialsValue && (
              <SchemaForm
                schema={cSchema}
                value={credentialsValue}
                errors={cValidation.errors}
                onChange={setCredentials}
              />
            )}
          </Stack>
        </CollapsableSection>
        {formError && <Alert severity="error" title={formError} />}
        <Stack gap={1}>
          <Button icon="save" disabled={blocked || save.isPending} onClick={() => save.mutate()}>
            {save.isPending ? t('common.misc.saving') : t('common.actions.save')}
          </Button>
          <Button variant="secondary" onClick={onClose}>
            {t('common.actions.cancel')}
          </Button>
        </Stack>
      </Stack>
    </Drawer>
  )
}
