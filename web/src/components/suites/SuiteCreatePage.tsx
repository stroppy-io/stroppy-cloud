import { suiteMutations } from '@api/queries/suites'
import { Page } from '@app/Page'
import { PageHeader } from '@app/PageHeader'
import { toast } from '@app/Toaster'
import { useTenant } from '@hooks/useTenant'
import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { SuiteForm, suiteFormDefaults, toSuiteWrite } from './SuiteForm'

export function SuiteCreatePage() {
  const { t } = useTranslation()
  const { slug } = useTenant()
  const qc = useQueryClient()
  const navigate = useNavigate()
  return (
    <Page width="narrow">
      <PageHeader
        breadcrumbs={[
          { label: t('suites.title'), to: '/t/$slug/suites', params: { slug } },
          { label: t('suites.form.title') },
        ]}
        title={t('suites.form.title')}
        subtitle={t('suites.form.subtitle')}
        icon="layer-group"
      />
      <SuiteForm
        initial={suiteFormDefaults()}
        submitLabel={t('suites.form.createAndContinue')}
        onCancel={() => void navigate({ to: '/t/$slug/suites', params: { slug } })}
        onSubmit={async (v) => {
          const suite = await suiteMutations.create(slug, toSuiteWrite(v))
          toast.success(t('suites.toasts.created'), { description: suite.name })
          await qc.invalidateQueries({ queryKey: ['t', slug, 'suites'] })
          await navigate({
            to: '/t/$slug/suites/$id',
            params: { slug, id: suite.id },
            search: { tab: 'axes' },
          })
        }}
      />
    </Page>
  )
}
