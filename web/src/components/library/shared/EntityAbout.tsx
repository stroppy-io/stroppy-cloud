import type { Entity } from '@api/types'
import { toast } from '@app/Toaster'
import { CopyText } from '@components/CopyText'
import { KeyValueList } from '@components/KeyValueList'
import { RelativeTime } from '@components/RelativeTime'
import { TagsEditor } from '@components/TagsEditor'
import { UserLabel } from '@components/UserAvatar'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Button, Field, Input, Modal, Stack, Text, TextArea, useStyles2 } from '@grafana/ui'
import { formatDateTime } from '@helpers/time'
import { useForm } from '@tanstack/react-form'
import { type ReactNode, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { applyServerErrors, firstError, serverFormError } from './form'
import { TagsCell } from './TagsCell'

export interface EntityPatch {
  name?: string
  description?: string
  tags?: Record<string, string>
}

const getStyles = (theme: GrafanaTheme2) => ({
  side: css({
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(2),
    padding: theme.spacing(2),
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
  }),
  desc: css({ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }),
  muted: css({ color: theme.colors.text.secondary, fontStyle: 'italic' }),
})

// GitHub-style "About" sidebar: description + tags (inline edit) and metadata.
export function EntityAbout({
  entity,
  canEdit,
  onSave,
  extra,
}: {
  entity: Entity
  canEdit: boolean
  onSave: (patch: EntityPatch) => Promise<unknown>
  extra?: { label: ReactNode; value: ReactNode }[]
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const [editing, setEditing] = useState(false)
  const schema = useMemo(
    () =>
      z.object({
        // OpenAPI EntityPatch.description: free text
        description: z
          .string()
          .trim()
          .max(2000, t('common.validation.max', { max: 2000 })),
        tags: z.record(z.string(), z.string()),
      }),
    [t]
  )
  const form = useForm({
    defaultValues: { description: entity.description ?? '', tags: entity.tags ?? {} },
    validators: { onSubmit: schema },
    onSubmit: async ({ value, formApi }) => {
      try {
        await onSave({ description: value.description.trim(), tags: value.tags })
        toast.success(t('library.about.saved'))
        setEditing(false)
      } catch (e) {
        applyServerErrors(formApi, e)
      }
    },
  })
  return (
    <aside className={styles.side}>
      <Stack justifyContent="space-between" alignItems="center">
        <Text element="h3" variant="h5">
          {t('library.about.title')}
        </Text>
        {canEdit && !editing && (
          <Button
            size="sm"
            variant="secondary"
            fill="text"
            icon="pen"
            onClick={() => {
              form.reset({ description: entity.description ?? '', tags: entity.tags ?? {} })
              setEditing(true)
            }}
          >
            {t('common.actions.edit')}
          </Button>
        )}
      </Stack>
      {editing ? (
        <form
          onSubmit={(e) => {
            e.preventDefault()
            void form.handleSubmit()
          }}
        >
          <form.Field name="description">
            {(field) => {
              const err = firstError(field.state.meta)
              return (
                <Field
                  label={t('common.fields.description')}
                  invalid={!!err}
                  error={err}
                  htmlFor={field.name}
                >
                  <TextArea
                    id={field.name}
                    rows={4}
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.currentTarget.value)}
                    placeholder={t('library.about.descriptionPlaceholder')}
                  />
                </Field>
              )
            }}
          </form.Field>
          <form.Field name="tags">
            {(field) => (
              <Field
                label={t('common.fields.tags')}
                description={t('library.about.tagsHint')}
                htmlFor={field.name}
              >
                <TagsEditor
                  value={field.state.value}
                  onChange={(v) => field.handleChange(v)}
                  placeholder="env=prod"
                />
              </Field>
            )}
          </form.Field>
          <form.Subscribe
            selector={(s) => [s.isSubmitting, serverFormError(s.errorMap.onServer)] as const}
          >
            {([submitting, serverError]) => (
              <Stack direction="column" gap={1}>
                {serverError ? <Text color="error">{serverError}</Text> : null}
                <Stack gap={1}>
                  <Button type="submit" size="sm" disabled={submitting}>
                    {t('common.actions.save')}
                  </Button>
                  <Button
                    type="button"
                    size="sm"
                    variant="secondary"
                    onClick={() => setEditing(false)}
                  >
                    {t('common.actions.cancel')}
                  </Button>
                </Stack>
              </Stack>
            )}
          </form.Subscribe>
        </form>
      ) : (
        <>
          <div className={entity.description ? styles.desc : styles.muted}>
            {entity.description || t('library.about.noDescription')}
          </div>
          {entity.tags && Object.keys(entity.tags).length > 0 ? (
            <TagsCell tags={entity.tags} max={12} />
          ) : (
            <span className={styles.muted}>{t('library.about.noTags')}</span>
          )}
        </>
      )}
      <KeyValueList
        items={[
          ...(extra ?? []),
          {
            label: t('common.fields.author'),
            value: <UserLabel user={entity.author} />,
          },
          {
            label: t('common.fields.created'),
            value: (
              <span title={formatDateTime(entity.created_at)}>
                {formatDateTime(entity.created_at)}
              </span>
            ),
          },
          { label: t('common.fields.updated'), value: <RelativeTime value={entity.updated_at} /> },
          { label: t('common.fields.id'), value: <CopyText value={entity.id} /> },
        ]}
      />
    </aside>
  )
}

// Rename dialog used from the page header.
export function RenameModal({
  isOpen,
  name,
  onClose,
  onSave,
}: {
  isOpen: boolean
  name: string
  onClose: () => void
  onSave: (name: string) => Promise<unknown>
}) {
  const { t } = useTranslation()
  const schema = useMemo(
    () =>
      z.object({
        // OpenAPI EntityPatch.name
        name: z
          .string()
          .trim()
          .min(1, t('common.validation.required'))
          .max(120, t('common.validation.max', { max: 120 })),
      }),
    [t]
  )
  const form = useForm({
    defaultValues: { name },
    validators: { onSubmit: schema },
    onSubmit: async ({ value, formApi }) => {
      try {
        await onSave(value.name.trim().replace(/\s+/g, ' '))
        toast.success(t('library.about.renamed'))
        onClose()
      } catch (e) {
        applyServerErrors(formApi, e)
      }
    },
  })
  return (
    <Modal isOpen={isOpen} title={t('library.about.rename')} onDismiss={onClose}>
      <form
        onSubmit={(e) => {
          e.preventDefault()
          void form.handleSubmit()
        }}
      >
        <form.Field name="name">
          {(field) => {
            const err = firstError(field.state.meta)
            return (
              <Field
                label={t('common.fields.name')}
                invalid={!!err}
                error={err}
                htmlFor={field.name}
              >
                <Input
                  id={field.name}
                  autoFocus
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.currentTarget.value)}
                />
              </Field>
            )
          }}
        </form.Field>
        <Modal.ButtonRow>
          <Button variant="secondary" type="button" onClick={onClose}>
            {t('common.actions.cancel')}
          </Button>
          <form.Subscribe selector={(s) => s.isSubmitting}>
            {(submitting) => (
              <Button type="submit" disabled={submitting}>
                {t('common.actions.save')}
              </Button>
            )}
          </form.Subscribe>
        </Modal.ButtonRow>
      </form>
    </Modal>
  )
}
