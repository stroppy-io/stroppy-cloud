import { suiteMutations, suiteQueries } from '@api/queries/suites'
import type { Schemas } from '@api/types'
import { ErrorState } from '@app/ErrorState'
import { Page } from '@app/Page'
import { PageHeader } from '@app/PageHeader'
import { toast } from '@app/Toaster'
import { ConfirmAction } from '@components/ConfirmAction'
import { FavoriteButton } from '@components/FavoriteButton'
import { Badge, Button, Dropdown, Menu, Modal, Tab, TabContent, TabsBar } from '@grafana/ui'
import { useTenant } from '@hooks/useTenant'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { SuiteAxesTab } from './SuiteAxesTab'
import { SuiteCellsTab } from './SuiteCellsTab'
import { SuiteForm, suiteFormDefaults, toSuiteWrite } from './SuiteForm'
import { SuiteLaunchDialog } from './SuiteLaunchDialog'
import { downloadJson } from './SuiteListPage'
import { SuiteOverviewTab } from './SuiteOverviewTab'
import { SuiteRunsTab } from './SuiteRunsTab'

export type SuiteTab = 'overview' | 'axes' | 'cells' | 'runs'
export const SUITE_TABS: SuiteTab[] = ['overview', 'axes', 'cells', 'runs']
type Suite = Schemas['Suite']

export function SuiteDetailPage({
  id,
  tab,
  onTabChange,
}: {
  id: string
  tab: SuiteTab
  onTabChange: (tab: SuiteTab) => void
}) {
  const { t } = useTranslation()
  const { slug, can } = useTenant()
  const qc = useQueryClient()
  const navigate = useNavigate()
  const suite = useQuery(suiteQueries.detail(slug, id))
  const [launching, setLaunching] = useState(false)
  const [editing, setEditing] = useState(false)
  const invalidate = () => qc.invalidateQueries({ queryKey: ['t', slug, 'suites'] })
  const onError = (e: unknown) => toast.error(e)
  const clone = useMutation({
    mutationFn: (s: Suite) => suiteMutations.clone(slug, s.id),
    onSuccess: (s) => {
      toast.success(t('suites.toasts.cloned'), { description: s.name })
      void invalidate()
      void navigate({ to: '/t/$slug/suites/$id', params: { slug, id: s.id } })
    },
    onError,
  })
  const exportM = useMutation({
    mutationFn: (s: Suite) => suiteMutations.export(slug, s.id),
    onSuccess: (doc, s) => {
      downloadJson(s.name, doc)
      toast.success(t('suites.toasts.exported'))
    },
    onError,
  })
  const remove = useMutation({
    mutationFn: (s: Suite) => suiteMutations.remove(slug, s.id),
    onSuccess: () => {
      toast.success(t('suites.toasts.deleted'))
      void invalidate()
      void navigate({ to: '/t/$slug/suites', params: { slug } })
    },
    onError,
  })

  if (suite.isError)
    return (
      <Page>
        <ErrorState error={suite.error} onRetry={() => void suite.refetch()} />
      </Page>
    )
  const s = suite.data
  const enabled = s?.summary?.enabled_cell_count ?? 0
  const total = s?.summary?.cell_count ?? 0
  const canEdit = can('edit-library')

  return (
    <Page width="wide">
      <PageHeader
        breadcrumbs={[
          { label: t('suites.title'), to: '/t/$slug/suites', params: { slug } },
          { label: s?.name ?? '…' },
        ]}
        title={s?.name ?? '…'}
        icon="layer-group"
        subtitle={s?.description}
        badge={
          s ? (
            <Badge
              text={t('suites.cellsOf', { enabled, total })}
              color={enabled ? 'blue' : 'darkgrey'}
              icon="apps"
            />
          ) : undefined
        }
        actions={
          s ? (
            <>
              <FavoriteButton kind="suite" id={s.id} value={s.is_favorite} />
              <ConfirmAction
                title={t('common.confirm.deleteTitle', { name: s.name })}
                body={t('suites.confirm.deleteBody')}
                onConfirm={() => remove.mutateAsync(s)}
              >
                {(openDelete) => (
                  <Dropdown
                    overlay={
                      <Menu>
                        <Menu.Item
                          label={t('suites.actions.editMeta')}
                          icon="edit"
                          disabled={!canEdit}
                          onClick={() => setEditing(true)}
                        />
                        <Menu.Item
                          label={t('suites.actions.clone')}
                          icon="copy"
                          disabled={!canEdit}
                          onClick={() => clone.mutate(s)}
                        />
                        <Menu.Item
                          label={t('suites.actions.export')}
                          icon="download-alt"
                          onClick={() => exportM.mutate(s)}
                        />
                        <Menu.Divider />
                        <Menu.Item
                          label={t('suites.actions.delete')}
                          icon="trash-alt"
                          destructive
                          disabled={!canEdit}
                          onClick={openDelete}
                        />
                      </Menu>
                    }
                  >
                    <Button
                      variant="secondary"
                      icon="ellipsis-v"
                      aria-label={t('common.actions.more')}
                    />
                  </Dropdown>
                )}
              </ConfirmAction>
              <Button
                icon="play"
                disabled={!can('run') || enabled === 0}
                onClick={() => setLaunching(true)}
              >
                {t('suites.actions.launch')}
              </Button>
            </>
          ) : undefined
        }
      />
      <TabsBar>
        {SUITE_TABS.map((id) => (
          <Tab
            key={id}
            label={t(`suites.tabs.${id}`)}
            active={tab === id}
            counter={
              id === 'cells' ? total : id === 'runs' ? (s?.summary?.run_count ?? 0) : undefined
            }
            onChangeTab={() => onTabChange(id)}
          />
        ))}
      </TabsBar>
      <TabContent>
        {s && tab === 'overview' && (
          <SuiteOverviewTab suite={s} onEdit={() => setEditing(true)} onGoTab={onTabChange} />
        )}
        {s && tab === 'axes' && <SuiteAxesTab suite={s} readOnly={!canEdit} />}
        {s && tab === 'cells' && (
          <SuiteCellsTab suite={s} readOnly={!canEdit} onLaunch={() => setLaunching(true)} />
        )}
        {s && tab === 'runs' && <SuiteRunsTab suite={s} />}
      </TabContent>
      {launching && s && <SuiteLaunchDialog suite={s} onClose={() => setLaunching(false)} />}
      {editing && s && (
        <Modal title={t('suites.actions.editMeta')} isOpen onDismiss={() => setEditing(false)}>
          <SuiteForm
            initial={suiteFormDefaults(s)}
            submitLabel={t('common.actions.save')}
            showTests={false}
            onCancel={() => setEditing(false)}
            onSubmit={async (v) => {
              const w = toSuiteWrite(v, s)
              await suiteMutations.patch(slug, s.id, {
                name: w.name,
                description: w.description ?? '',
                tags: w.tags ?? {},
                concurrency: w.concurrency,
                defaults: w.defaults,
              })
              toast.success(t('suites.toasts.saved'))
              await invalidate()
              setEditing(false)
            }}
          />
        </Modal>
      )}
    </Page>
  )
}
