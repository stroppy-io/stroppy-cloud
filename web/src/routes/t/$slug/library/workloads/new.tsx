import { Page } from '@app/Page'
import { PageHeader } from '@app/PageHeader'
import { WorkloadForm } from '@components/library/workloads/WorkloadForm'
import { createFileRoute } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

export const Route = createFileRoute('/t/$slug/library/workloads/new')({
  staticData: { crumb: 'library.workloads.new' },
  component: () => {
    const { t } = useTranslation()
    const { slug } = Route.useParams()
    const navigate = Route.useNavigate()
    return (
      <Page>
        <PageHeader
          title={t('library.workloads.new')}
          icon="bolt"
          breadcrumbs={[
            { label: t('nav.library') },
            { label: t('nav.workloads'), to: '/t/$slug/library/workloads', params: { slug } },
            { label: t('common.actions.create') },
          ]}
        />
        <WorkloadForm
          mode="create"
          onCancel={() => void navigate({ to: '/t/$slug/library/workloads', params: { slug } })}
          onDone={({ workload }) => {
            if (workload)
              void navigate({
                to: '/t/$slug/library/workloads/$id',
                params: { slug, id: workload.id },
              })
          }}
        />
      </Page>
    )
  },
})
