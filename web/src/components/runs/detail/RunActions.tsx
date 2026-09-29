import { keys } from '@api/queries/keys'
import { runMutations } from '@api/queries/runs'
import type { Run } from '@api/types'
import { toast } from '@app/Toaster'
import { ConfirmAction } from '@components/ConfirmAction'
import { FavoriteButton } from '@components/FavoriteButton'
import { ShareModal } from '@components/runs/ShareModal'
import { Button, ButtonGroup, Dropdown, Menu, Tooltip } from '@grafana/ui'
import { isTerminal } from '@helpers/run-status'
import { useTenant } from '@hooks/useTenant'
import { downloadText } from '@lib/download'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { KeepExtendModal } from './KeepExtendModal'
import { type RerunBody, RerunDrawer } from './RerunDrawer'
import { SaveAsTestModal } from './SaveAsTestModal'

type Dialog = 'none' | 'clone' | 'resume' | 'saveAsTest' | 'share' | 'keep'

// Header actions of a run, gated by role. Every mutation invalidates the tenant cache and toasts.
export function RunActions({ run }: { run: Run }) {
  const { t } = useTranslation()
  const { slug, can } = useTenant()
  const qc = useQueryClient()
  const navigate = useNavigate()
  const [dialog, setDialog] = useState<Dialog>('none')
  const invalidate = () => qc.invalidateQueries({ queryKey: keys.t(slug) })
  const terminal = isTerminal(run.status)
  const active = run.status === 'running' || run.status === 'pending'
  const canRun = can('run')
  const canCancel = can('cancel-any-run', run.author.id)

  const cancel = useMutation({
    mutationFn: () => runMutations.cancel(slug, run.id),
    onSuccess: () => {
      toast.success(t('runs.toasts.cancelRequested'))
      void invalidate()
    },
    onError: (e) => toast.error(e),
  })
  const rerun = useMutation({
    mutationFn: (body: RerunBody) => runMutations.rerun(slug, run.id, body),
    onSuccess: (created, body) => {
      toast.success(
        body.resume && !run.stand_kept
          ? t('runs.toasts.rerunDegraded')
          : t('runs.toasts.rerunStarted', { name: created.name })
      )
      void invalidate()
      setDialog('none')
      void navigate({ to: '/t/$slug/runs/$id', params: { slug, id: created.id } })
    },
    onError: (e) => toast.error(e),
  })
  const saveAsTest = useMutation({
    mutationFn: (body: { name: string; save_database_as?: string; save_workload_as?: string }) =>
      runMutations.saveAsTest(slug, run.id, body),
    onSuccess: (test) => {
      toast.success(t('runs.toasts.savedAsTest', { name: test.name }), {
        action: {
          label: t('common.actions.open'),
          onClick: () =>
            void navigate({ to: '/t/$slug/library/tests/$id', params: { slug, id: test.id } }),
        },
      })
      void invalidate()
      setDialog('none')
    },
  })
  const keepExtend = useMutation({
    mutationFn: (duration: string) => runMutations.keepExtend(slug, run.id, duration),
    onSuccess: () => {
      toast.success(t('runs.toasts.keepExtended'))
      void invalidate()
      setDialog('none')
    },
  })
  const keepRelease = useMutation({
    mutationFn: () => runMutations.keepRelease(slug, run.id),
    onSuccess: () => {
      toast.success(t('runs.toasts.keepReleased'))
      void invalidate()
    },
    onError: (e) => toast.error(e),
  })
  const remove = useMutation({
    mutationFn: () => runMutations.remove(slug, run.id),
    onSuccess: () => {
      toast.success(t('runs.toasts.deleted', { name: run.name }))
      void invalidate()
      void navigate({ to: '/t/$slug/runs', params: { slug }, search: {} as never })
    },
    onError: (e) => toast.error(e),
  })
  const exportRun = useMutation({
    mutationFn: async (format: 'json' | 'md') => {
      const res = (await runMutations.export(slug, run.id, format)) as unknown
      const safe = run.name.replace(/[^\w.-]+/g, '_')
      if (format === 'json') {
        downloadText(`${safe}.json`, JSON.stringify(res, null, 2), 'application/json')
      } else {
        const content =
          typeof res === 'string'
            ? res
            : ((res as { content?: string })?.content ?? JSON.stringify(res, null, 2))
        downloadText(`${safe}.md`, content, 'text/markdown')
      }
    },
    onSuccess: () => toast.success(t('runs.toasts.exported')),
    onError: (e) => toast.error(e),
  })

  const resumeItem = (
    <Menu.Item
      label={t('runs.actions.rerunResume')}
      description={
        run.stand_kept ? t('runs.actions.rerunResumeHint') : t('runs.actions.rerunResumeDegraded')
      }
      icon="history"
      disabled={!canRun || !terminal}
      onClick={() => setDialog('resume')}
    />
  )

  return (
    <>
      {active && (
        <Tooltip
          content={canCancel ? t('runs.actions.cancelHint') : t('runs.actions.noPermission')}
        >
          <Button
            variant="destructive"
            fill="outline"
            icon="times"
            disabled={!canCancel || cancel.isPending || run.status === 'cancelling'}
            onClick={() => cancel.mutate()}
          >
            {t('common.actions.cancel')}
          </Button>
        </Tooltip>
      )}
      {terminal && (
        <ButtonGroup>
          <Tooltip content={canRun ? t('runs.actions.rerunHint') : t('runs.actions.noPermission')}>
            <Button
              icon="repeat"
              disabled={!canRun || rerun.isPending}
              onClick={() => rerun.mutate({})}
            >
              {t('runs.actions.rerun')}
            </Button>
          </Tooltip>
          <Dropdown
            overlay={
              <Menu>
                <Menu.Item
                  label={t('runs.actions.clone')}
                  description={t('runs.actions.cloneHint')}
                  icon="copy"
                  disabled={!canRun}
                  onClick={() => setDialog('clone')}
                />
                {resumeItem}
              </Menu>
            }
          >
            <Button icon="angle-down" aria-label={t('common.actions.more')} disabled={!canRun} />
          </Dropdown>
        </ButtonGroup>
      )}
      <Button
        variant="secondary"
        icon="share-alt"
        disabled={!can('share')}
        onClick={() => setDialog('share')}
      >
        {t('common.actions.share')}
      </Button>
      <FavoriteButton kind="run" id={run.id} value={run.is_favorite} size="lg" />
      <Dropdown
        overlay={
          <Menu>
            <Menu.Group label={t('runs.actions.groupRun')}>
              <Menu.Item
                label={t('runs.actions.saveAsTest')}
                description={t('runs.actions.saveAsTestHint')}
                icon="save"
                disabled={!canRun}
                onClick={() => setDialog('saveAsTest')}
              />
              {!terminal && (
                <Menu.Item
                  label={t('runs.actions.clone')}
                  description={t('runs.actions.cloneHint')}
                  icon="copy"
                  disabled={!canRun}
                  onClick={() => setDialog('clone')}
                />
              )}
            </Menu.Group>
            <Menu.Group label={t('runs.actions.groupKeep')}>
              <Menu.Item
                label={t('runs.keep.extend')}
                description={t('runs.keep.extendHint')}
                icon="lock"
                disabled={!canRun}
                onClick={() => setDialog('keep')}
              />
              <Menu.Item
                label={t('runs.keep.release')}
                description={t('runs.keep.releaseHint')}
                icon="unlock"
                disabled={!canRun || !run.stand_kept || keepRelease.isPending}
                onClick={() => keepRelease.mutate()}
              />
            </Menu.Group>
            <Menu.Group label={t('common.actions.export')}>
              <Menu.Item
                label={t('runs.actions.exportJson')}
                icon="brackets-curly"
                onClick={() => exportRun.mutate('json')}
              />
              <Menu.Item
                label={t('runs.actions.exportMd')}
                icon="file-alt"
                onClick={() => exportRun.mutate('md')}
              />
            </Menu.Group>
            <Menu.Divider />
            <Menu.Item
              label={t('common.actions.compare')}
              icon="columns"
              onClick={() =>
                void navigate({
                  to: '/t/$slug/compare',
                  params: { slug },
                  search: { runs: [run.id] } as never,
                })
              }
            />
            <ConfirmAction
              title={t('common.confirm.deleteTitle', { name: run.name })}
              body={t('runs.actions.deleteBody')}
              onConfirm={() => remove.mutateAsync()}
            >
              {(open) => (
                <Menu.Item
                  label={t('common.actions.delete')}
                  description={terminal ? undefined : t('runs.actions.deleteNotTerminal')}
                  icon="trash-alt"
                  destructive
                  disabled={!canCancel || !terminal}
                  onClick={open}
                />
              )}
            </ConfirmAction>
          </Menu>
        }
      >
        <Button variant="secondary" icon="ellipsis-v" aria-label={t('common.actions.more')} />
      </Dropdown>

      {(dialog === 'clone' || dialog === 'resume') && (
        <RerunDrawer
          run={run}
          mode={dialog}
          onClose={() => setDialog('none')}
          onSubmit={(body) => rerun.mutateAsync(body)}
        />
      )}
      <SaveAsTestModal
        run={run}
        isOpen={dialog === 'saveAsTest'}
        onClose={() => setDialog('none')}
        onSubmit={(body) => saveAsTest.mutateAsync(body)}
      />
      <KeepExtendModal
        run={run}
        isOpen={dialog === 'keep'}
        onClose={() => setDialog('none')}
        onSubmit={(d) => keepExtend.mutateAsync(d)}
      />
      <ShareModal
        isOpen={dialog === 'share'}
        onClose={() => setDialog('none')}
        defaultTitle={run.name}
        onCreate={async (input) => {
          const share = await runMutations.share(slug, run.id, {
            scope: input.scope,
            ttl: input.ttl,
            title: input.title || undefined,
          })
          void invalidate()
          return share
        }}
      />
    </>
  )
}
