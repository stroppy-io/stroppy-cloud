import { scheduleMutations } from '@api/queries/suites'
import { Page } from '@app/Page'
import { PageHeader } from '@app/PageHeader'
import { toast } from '@app/Toaster'
import { useTenant } from '@hooks/useTenant'
import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { ScheduleForm, scheduleFormDefaults } from './ScheduleForm'

export function ScheduleCreatePage({ kind, target }: { kind?: 'test' | 'suite'; target?: string }) {
  const { t } = useTranslation()
  const { slug } = useTenant()
  const qc = useQueryClient()
  const navigate = useNavigate()
  return (
    <Page width="wide">
      <PageHeader
        breadcrumbs={[
          { label: t('schedules.title'), to: '/t/$slug/schedules', params: { slug } },
          { label: t('schedules.form.title') },
        ]}
        title={t('schedules.form.title')}
        subtitle={t('schedules.form.subtitle')}
        icon="clock-nine"
      />
      <ScheduleForm
        initial={scheduleFormDefaults(undefined, { kind, target })}
        submitLabel={t('schedules.actions.create')}
        onCancel={() => void navigate({ to: '/t/$slug/schedules', params: { slug } })}
        onSubmit={async (w) => {
          const s = await scheduleMutations.create(slug, w)
          toast.success(t('schedules.toasts.created'), { description: s.name })
          await qc.invalidateQueries({ queryKey: ['t', slug, 'schedules'] })
          await navigate({ to: '/t/$slug/schedules/$id', params: { slug, id: s.id } })
        }}
      />
    </Page>
  )
}
