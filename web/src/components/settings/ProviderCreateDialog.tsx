import { catalogQueries } from '@api/queries/catalog'
import { settingsMutations } from '@api/queries/settings'
import type { CatalogProvider, ProviderKind } from '@api/types'
import { toast } from '@app/Toaster'
import { defaults, type Schema, type Values } from '@components/schema/engine'
import { SchemaForm } from '@components/schema/SchemaForm'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import {
  Alert,
  Button,
  Field,
  Input,
  LoadingPlaceholder,
  Modal,
  Stack,
  Text,
  useStyles2,
} from '@grafana/ui'
import { serverErrors } from '@helpers/form'
import { useSchemaValidate } from '@hooks/useSchemaValidate'
import { useTenant } from '@hooks/useTenant'
import { useMutation, useQuery } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { KIND_LABEL } from './ProvidersPage'

type Step = 'kind' | 'settings' | 'credentials' | 'name'
const STEPS: Step[] = ['kind', 'settings', 'credentials', 'name']

const getStyles = (theme: GrafanaTheme2) => ({
  steps: css({
    display: 'flex',
    gap: theme.spacing(1),
    alignItems: 'center',
    fontSize: theme.typography.bodySmall.fontSize,
    color: theme.colors.text.secondary,
    marginBottom: theme.spacing(1),
  }),
  step: css({ display: 'inline-flex', alignItems: 'center', gap: theme.spacing(0.5) }),
  stepNum: css({
    width: 20,
    height: 20,
    borderRadius: '50%',
    display: 'inline-grid',
    placeItems: 'center',
    border: `1px solid ${theme.colors.border.medium}`,
  }),
  stepActive: css({
    color: theme.colors.text.primary,
    fontWeight: theme.typography.fontWeightMedium,
    span: {
      borderColor: theme.colors.primary.border,
      background: theme.colors.primary.transparent,
    },
  }),
  kinds: css({
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))',
    gap: theme.spacing(1.5),
  }),
  kind: css({
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(1),
    padding: theme.spacing(2),
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.secondary,
    cursor: 'pointer',
    textAlign: 'left',
    color: theme.colors.text.primary,
    font: 'inherit',
    '&:hover': { borderColor: theme.colors.border.strong },
  }),
  kindSelected: css({
    borderColor: theme.colors.primary.border,
    boxShadow: `0 0 0 1px ${theme.colors.primary.border}`,
  }),
  logo: css({
    width: 40,
    height: 40,
    borderRadius: theme.shape.radius.default,
    display: 'grid',
    placeItems: 'center',
    background: theme.colors.background.primary,
    border: `1px solid ${theme.colors.border.weak}`,
    fontWeight: theme.typography.fontWeightBold,
  }),
  body: css({ minHeight: 240 }),
})

export function ProviderCreateDialog({
  onClose,
  onCreated,
}: {
  onClose: () => void
  onCreated: () => Promise<void>
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const { slug } = useTenant()
  const catalog = useQuery(catalogQueries.providers())
  const [step, setStep] = useState<Step>('kind')
  const [kind, setKind] = useState<ProviderKind>()
  const [settings, setSettings] = useState<Values>()
  const [credentials, setCredentials] = useState<Values>()
  const [name, setName] = useState('')
  const [nameError, setNameError] = useState<string>()
  const [formError, setFormError] = useState<string>()
  const [touched, setTouched] = useState({ settings: false, credentials: false })

  const provider: CatalogProvider | undefined = catalog.data?.data.find((p) => p.kind === kind)
  const settingsSchema = useQuery({
    ...catalogQueries.schema(provider?.settings_schema ?? ''),
    enabled: !!provider?.settings_schema,
  })
  const credentialsSchema = useQuery({
    ...catalogQueries.schema(provider?.credentials_schema ?? ''),
    enabled: !!provider?.credentials_schema,
  })
  const sSchema = settingsSchema.data as Schema | undefined
  const cSchema = credentialsSchema.data as Schema | undefined
  const settingsValue = useMemo(
    () => settings ?? (sSchema ? defaults(sSchema.fields) : undefined),
    [settings, sSchema]
  )
  const credentialsValue = useMemo(
    () => credentials ?? (cSchema ? defaults(cSchema.fields) : undefined),
    [credentials, cSchema]
  )
  const sValidation = useSchemaValidate(
    provider?.settings_schema,
    sSchema,
    settingsValue,
    step === 'settings' && touched.settings
  )
  const cValidation = useSchemaValidate(
    provider?.credentials_schema,
    cSchema,
    credentialsValue,
    step === 'credentials' && touched.credentials
  )

  const create = useMutation({
    mutationFn: () =>
      settingsMutations.createProvider(slug, {
        name: name.trim(),
        kind: kind as ProviderKind,
        settings: settingsValue ?? {},
        credentials: credentialsValue ?? {},
      }),
    onSuccess: async (p) => {
      toast.success(t('settings.providers.created', { name: p.name }))
      await onCreated()
    },
    onError: (e) => {
      const s = serverErrors(e, t('common.errors.generic'))
      setNameError(s.fields.name)
      setFormError(s.form)
      if (s.fields.name) setStep('name')
    },
  })

  const idx = STEPS.indexOf(step)
  const canNext =
    step === 'kind'
      ? !!kind
      : step === 'settings'
        ? !!sSchema && !sValidation.hasErrors
        : step === 'credentials'
          ? !!cSchema && !cValidation.hasErrors
          : name.trim().length >= 2
  const next = () => {
    if (step === 'settings') setTouched((x) => ({ ...x, settings: true }))
    if (step === 'credentials') setTouched((x) => ({ ...x, credentials: true }))
    if (!canNext) return
    if (step === 'name') {
      if (name.trim().length < 2) {
        setNameError(t('common.validation.min', { min: 2 }))
        return
      }
      create.mutate()
      return
    }
    setStep(STEPS[idx + 1])
  }

  return (
    <Modal isOpen title={t('settings.providers.createTitle')} onDismiss={onClose}>
      <nav className={styles.steps} aria-label={t('settings.providers.steps')}>
        {STEPS.map((s, i) => (
          <span key={s} className={cx(styles.step, s === step && styles.stepActive)}>
            <span className={styles.stepNum}>{i + 1}</span>
            {t(`settings.providers.step.${s}`)}
            {i < STEPS.length - 1 && <span aria-hidden> ›</span>}
          </span>
        ))}
      </nav>
      <div className={styles.body}>
        {step === 'kind' && (
          <Stack direction="column" gap={1}>
            <Text color="secondary">{t('settings.providers.kindHint')}</Text>
            {catalog.isPending && <LoadingPlaceholder text={t('common.misc.loading')} />}
            <div className={styles.kinds}>
              {catalog.data?.data.map((p) => (
                <button
                  key={p.kind}
                  type="button"
                  className={cx(styles.kind, kind === p.kind && styles.kindSelected)}
                  onClick={() => {
                    if (kind !== p.kind) {
                      setSettings(undefined)
                      setCredentials(undefined)
                      setTouched({ settings: false, credentials: false })
                    }
                    setKind(p.kind)
                  }}
                  aria-pressed={kind === p.kind}
                >
                  <span className={styles.logo} aria-hidden>
                    {KIND_LABEL[p.kind] ?? p.kind}
                  </span>
                  <Text weight="medium">{p.title}</Text>
                  <Text color="secondary" variant="bodySmall">
                    {t(`settings.providers.kindDesc.${p.kind}`, {
                      defaultValue: t('settings.providers.kindDescGeneric', {
                        count: p.locations.length,
                      }),
                    })}
                  </Text>
                </button>
              ))}
            </div>
          </Stack>
        )}
        {step === 'settings' && (
          <Stack direction="column" gap={1}>
            <Text color="secondary">{t('settings.providers.settingsHint')}</Text>
            {settingsSchema.isPending && <LoadingPlaceholder text={t('common.misc.loading')} />}
            {settingsSchema.isError && (
              <Alert severity="error" title={t('settings.providers.schemaMissing')} />
            )}
            {sSchema && settingsValue && (
              <SchemaForm
                schema={sSchema}
                value={settingsValue}
                errors={touched.settings ? sValidation.errors : undefined}
                onChange={(v) => {
                  setSettings(v)
                  setTouched((x) => ({ ...x, settings: true }))
                }}
              />
            )}
          </Stack>
        )}
        {step === 'credentials' && (
          <Stack direction="column" gap={1}>
            <Alert severity="info" title={t('settings.providers.credentialsTitle')}>
              {t('settings.providers.credentialsHint')}
            </Alert>
            {credentialsSchema.isPending && <LoadingPlaceholder text={t('common.misc.loading')} />}
            {credentialsSchema.isError && (
              <Alert severity="error" title={t('settings.providers.schemaMissing')} />
            )}
            {cSchema && credentialsValue && (
              <SchemaForm
                schema={cSchema}
                value={credentialsValue}
                errors={touched.credentials ? cValidation.errors : undefined}
                onChange={(v) => {
                  setCredentials(v)
                  setTouched((x) => ({ ...x, credentials: true }))
                }}
              />
            )}
          </Stack>
        )}
        {step === 'name' && (
          <Stack direction="column" gap={1}>
            <Field
              label={t('settings.providers.name')}
              description={t('settings.providers.nameHint')}
              invalid={!!nameError}
              error={nameError}
              required
            >
              <Input
                autoFocus
                placeholder={kind === 'aws' ? 'aws-eu-central' : 'yc-benchmarks'}
                value={name}
                onChange={(e) => {
                  setName(e.currentTarget.value)
                  setNameError(undefined)
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') next()
                }}
              />
            </Field>
            <Alert severity="info" title={t('settings.providers.afterCreateTitle')}>
              {t('settings.providers.afterCreateBody')}
            </Alert>
            {formError && <Alert severity="error" title={formError} />}
          </Stack>
        )}
      </div>
      <Modal.ButtonRow
        leftItems={
          idx > 0 ? (
            <Button
              variant="secondary"
              fill="text"
              icon="arrow-left"
              onClick={() => setStep(STEPS[idx - 1])}
            >
              {t('common.actions.back')}
            </Button>
          ) : undefined
        }
      >
        <Button variant="secondary" onClick={onClose}>
          {t('common.actions.cancel')}
        </Button>
        <Button
          onClick={next}
          disabled={create.isPending || (step !== 'settings' && step !== 'credentials' && !canNext)}
          icon={step === 'name' ? 'cloud' : undefined}
        >
          {step === 'name'
            ? create.isPending
              ? t('common.misc.saving')
              : t('settings.providers.createCta')
            : t('common.actions.next')}
        </Button>
      </Modal.ButtonRow>
    </Modal>
  )
}
