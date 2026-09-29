import { Page } from '@app/Page'
import { PageHeader } from '@app/PageHeader'
import { DatabaseWizard } from '@components/library/databases/DatabaseWizard'
import { createFileRoute } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

export const Route = createFileRoute('/t/$slug/library/databases/new')({
  staticData: { crumb: 'library.databases.new' },
  validateSearch: z.object({ kind: z.string().optional().catch(undefined) }),
  component: () => {
    const { t } = useTranslation()
    const { slug } = Route.useParams()
    const { kind } = Route.useSearch()
    const navigate = Route.useNavigate()
    return (
      <Page>
        <PageHeader
          title={t('library.databases.new')}
          icon="database"
          breadcrumbs={[
            { label: t('nav.library') },
            { label: t('nav.databases'), to: '/t/$slug/library/databases', params: { slug } },
            { label: t('common.actions.create') },
          ]}
        />
        <DatabaseWizard
          mode="create"
          initialKind={kind}
          onCancel={() => void navigate({ to: '/t/$slug/library/databases', params: { slug } })}
          onDone={({ database }) => {
            if (database)
              void navigate({
                to: '/t/$slug/library/databases/$id',
                params: { slug, id: database.id },
              })
          }}
        />
      </Page>
    )
  },
})
