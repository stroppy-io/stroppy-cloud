import { isApiError } from '@api/errors'
import { suiteMutations, suiteQueries } from '@api/queries/suites'
import type { Schemas } from '@api/types'
import { toast } from '@app/Toaster'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Alert, Button, Checkbox, Field, Input, Modal, Stack, Text, useStyles2 } from '@grafana/ui'
import { useTenant } from '@hooks/useTenant'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

type Suite = Schemas['Suite']

const getStyles = (theme: GrafanaTheme2) => ({
  cells: css({
    maxHeight: 180,
    overflow: 'auto',
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    padding: theme.spacing(0.5, 1),
    fontSize: theme.typography.bodySmall.fontSize,
    fontFamily: theme.typography.fontFamilyMonospace,
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))',
    gap: theme.spacing(0.25, 1),
  }),
  invalid: css({ color: theme.colors.error.text }),
})

// Launch confirmation: run name, concurrency, defaults, list of enabled cells and totals.
export function SuiteLaunchDialog({ suite, onClose }: { suite: Suite; onClose: () => void }) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const { slug } = useTenant()
  const qc = useQueryClient()
  const navigate = useNavigate()
  const runCount = suite.summary?.run_count ?? 0
  const [name, setName] = useState(`${suite.name} #${runCount + 1}`)
  const [concurrency, setConcurrency] = useState(String(suite.concurrency ?? 1))
  const [keep, setKeep] = useState(
    suite.defaults?.keep && suite.defaults.keep !== '0s' ? suite.defaults.keep : ''
  )
  const [rating, setRating] = useState<Schemas['RatingFlags']>({
    tenant: suite.defaults?.rating?.tenant ?? true,
    global: suite.defaults?.rating?.global ?? false,
  })
  const enabled = useMemo(() => (suite.cells ?? []).filter((c) => c.enabled), [suite.cells])
  const invalid = enabled.filter((c) => c.validation && !c.validation.fits)
  const previewBody = useMemo<Schemas['SuiteWrite']>(
    () => ({
      name: suite.name,
      tests: suite.tests,
      axes: suite.axes,
      cells: suite.cells,
      concurrency: Number(concurrency) || suite.concurrency || 1,
      defaults: suite.defaults,
    }),
    [suite, concurrency]
  )
  const preview = useQuery(suiteQueries.preview(slug, previewBody))
  const [error, setError] = useState<string | undefined>()
  const launch = useMutation({
    mutationFn: () =>
      suiteMutations.launch(slug, suite.id, {
        name: name.trim() || undefined,
        concurrency: Number(concurrency) || undefined,
        keep: keep.trim() || undefined,
        rating,
      }),
    onSuccess: (sr) => {
      toast.success(t('suites.toasts.launched'), { description: sr.name })
      void qc.invalidateQueries({ queryKey: ['t', slug, 'suite-runs'] })
      void qc.invalidateQueries({ queryKey: ['t', slug, 'suites'] })
      void qc.invalidateQueries({ queryKey: ['t', slug, 'runs'] })
      onClose()
      void navigate({ to: '/t/$slug/suite-runs/$id', params: { slug, id: sr.id } })
    },
    onError: (e) => {
      const msg = isApiError(e)
        ? (e.validation?.errors?.map((x) => x.message).join('; ') ?? e.detail ?? e.title)
        : String(e)
      setError(msg)
      toast.error(e, { title: t('suites.toasts.launchFailed') })
    },
  })
  const concurrencyNum = Number(concurrency)
  const concurrencyBad =
    !Number.isInteger(concurrencyNum) || concurrencyNum < 1 || concurrencyNum > 16
  const keepBad = keep.trim() !== '' && !/^\d+(h|m|s)$/.test(keep.trim())

  return (
    <Modal title={t('suites.launch.title', { name: suite.name })} isOpen onDismiss={onClose}>
      <Stack direction="column" gap={1}>
        <Field label={t('suites.launch.name')} htmlFor="launch-name">
          <Input id="launch-name" value={name} onChange={(e) => setName(e.currentTarget.value)} />
        </Field>
        <Stack gap={2}>
          <Field
            label={t('suites.launch.concurrency')}
            htmlFor="launch-concurrency"
            invalid={concurrencyBad}
            error={concurrencyBad ? t('suites.form.errors.concurrency') : undefined}
          >
            <Input
              id="launch-concurrency"
              type="number"
              min={1}
              max={16}
              width={12}
              value={concurrency}
              onChange={(e) => setConcurrency(e.currentTarget.value)}
            />
          </Field>
          <Field
            label={t('suites.launch.keep')}
            htmlFor="launch-keep"
            invalid={keepBad}
            error={keepBad ? t('suites.form.errors.keep') : undefined}
          >
            <Input
              id="launch-keep"
              width={20}
              placeholder={t('suites.launch.keepPlaceholder')}
              value={keep}
              onChange={(e) => setKeep(e.currentTarget.value)}
            />
          </Field>
        </Stack>
        <Field label={t('suites.launch.rating')}>
          <Stack gap={2}>
            <Checkbox
              label={t('suites.form.ratingTenant')}
              value={!!rating.tenant}
              onChange={(e) => setRating({ ...rating, tenant: e.currentTarget.checked })}
            />
            <Checkbox
              label={t('suites.form.ratingGlobal')}
              value={!!rating.global}
              onChange={(e) => setRating({ ...rating, global: e.currentTarget.checked })}
            />
          </Stack>
        </Field>
        {enabled.length === 0 ? (
          <Alert severity="warning" title={t('suites.launch.noCells')} />
        ) : (
          <>
            <Text weight="medium">{t('suites.launch.cellsTitle', { count: enabled.length })}</Text>
            <div className={styles.cells}>
              {enabled.map((c) => (
                <span
                  key={c.id}
                  className={c.validation && !c.validation.fits ? styles.invalid : undefined}
                  title={c.validation?.issues?.map((i) => i.message).join('\n')}
                >
                  {c.name ?? c.id}
                </span>
              ))}
            </div>
            {preview.data?.totals && (
              <Text color="secondary" variant="bodySmall">
                {t('suites.launch.totals', {
                  machines: preview.data.totals.machines ?? 0,
                  cpu: preview.data.totals.cpu ?? 0,
                  ram: preview.data.totals.memory_gb ?? 0,
                })}
                {preview.data.totals.estimated_duration
                  ? ` · ~${preview.data.totals.estimated_duration}`
                  : ''}
              </Text>
            )}
          </>
        )}
        {invalid.length > 0 && (
          <Alert
            severity="error"
            title={t('suites.launch.invalidCells', { count: invalid.length })}
          >
            {invalid.map((c) => (
              <div key={c.id}>
                {c.name ?? c.id}: {c.validation?.issues?.[0]?.message}
              </div>
            ))}
          </Alert>
        )}
        {error && <Alert severity="error" title={error} />}
      </Stack>
      <Modal.ButtonRow>
        <Button variant="secondary" fill="outline" onClick={onClose}>
          {t('common.actions.cancel')}
        </Button>
        <Button
          icon="play"
          disabled={
            launch.isPending ||
            enabled.length === 0 ||
            invalid.length > 0 ||
            concurrencyBad ||
            keepBad
          }
          onClick={() => launch.mutate()}
        >
          {t('suites.launch.submit')}
        </Button>
      </Modal.ButtonRow>
    </Modal>
  )
}
