import type { CatalogDatabase, Schemas } from '@api/types'
import { toast } from '@app/Toaster'
import { CodeEditorLazy } from '@components/code/CodeEditorLazy'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Button, Field, Input, RadioButtonGroup, Stack, Text, useStyles2 } from '@grafana/ui'
import { parseDocument, toYaml } from '@helpers/yaml'
import { useSchemaValidate } from '@hooks/useSchemaValidate'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Panel } from '../shared/Panel'
import { FORM_KEYS, formPath, type Runtime, RuntimeForm } from './RuntimeForm'
import { RuntimeHelp } from './RuntimeHelp'

// "Custom build" of a database: an image instead of the catalog one, and
// runtime tweaks (test.runtime@1) laid over the containers the recipe makes.
// Nothing here is needed for an ordinary run, so both start empty.

type Database = Schemas['Database']

const RUNTIME_SCHEMA = 'test.runtime@1'

const getStyles = (theme: GrafanaTheme2) => ({
  root: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(2) }),
  yaml: css({
    display: 'grid',
    gridTemplateColumns: 'minmax(0, 1fr) 400px',
    gap: theme.spacing(2),
    alignItems: 'start',
    [theme.breakpoints.down('lg')]: { gridTemplateColumns: 'minmax(0, 1fr)' },
  }),
  mono: css({ fontFamily: theme.typography.fontFamilyMonospace }),
  issues: css({
    display: 'grid',
    gap: theme.spacing(0.5),
    marginTop: theme.spacing(2),
    fontSize: theme.typography.bodySmall.fontSize,
  }),
})

/** The catalog image of a database version, the one a custom image replaces. */
export function catalogImage(cat: CatalogDatabase | undefined, version: string): string {
  return cat?.versions.find((v) => v.version === version)?.image ?? ''
}

/** How many build settings differ from the catalog: image and runtime. */
export function buildChanges(db: Database): number {
  return (db.image ? 1 : 0) + (isEmptyRuntime(db.runtime) ? 0 : 1)
}

function isEmptyRuntime(v: unknown): boolean {
  return v == null || (typeof v === 'object' && Object.keys(v).length === 0)
}

export function DatabaseBuildTab({
  db,
  cat,
  canEdit,
  onSave,
}: {
  db: Database
  cat: CatalogDatabase | undefined
  canEdit: boolean
  onSave: (p: Schemas['DatabasePatch']) => Promise<unknown>
}) {
  const styles = useStyles2(getStyles)
  return (
    <div className={styles.root}>
      {/* A kind without a catalog image (custom) names its image in params. */}
      {(catalogImage(cat, db.version) || db.image) && (
        <ImagePanel db={db} cat={cat} canEdit={canEdit} onSave={onSave} />
      )}
      <RuntimePanel db={db} canEdit={canEdit} onSave={onSave} />
    </div>
  )
}

function ImagePanel({
  db,
  cat,
  canEdit,
  onSave,
}: {
  db: Database
  cat: CatalogDatabase | undefined
  canEdit: boolean
  onSave: (p: Schemas['DatabasePatch']) => Promise<unknown>
}) {
  const { t } = useTranslation()
  const stored = db.image ?? ''
  const [value, setValue] = useState(stored)
  const [saving, setSaving] = useState(false)
  const trimmed = value.trim()
  const dirty = trimmed !== stored
  const invalid = trimmed !== '' && /\s/.test(trimmed)
  const save = async (image: string) => {
    setSaving(true)
    try {
      await onSave({ image })
      setValue(image)
      toast.success(image ? t('library.build.image.saved') : t('library.build.image.cleared'))
    } catch (e) {
      toast.error(e)
    } finally {
      setSaving(false)
    }
  }
  return (
    <Panel
      title={t('library.build.image.title')}
      description={t('library.build.image.hint')}
      actions={
        <Stack gap={1}>
          <Button
            size="sm"
            variant="secondary"
            icon="history"
            disabled={!canEdit || (!stored && !dirty) || saving}
            onClick={() => (stored ? void save('') : setValue(''))}
          >
            {t('library.build.image.reset')}
          </Button>
          <Button
            size="sm"
            disabled={!canEdit || !dirty || invalid || saving}
            onClick={() => void save(trimmed)}
          >
            {t('common.actions.save')}
          </Button>
        </Stack>
      }
    >
      <Field
        label={t('library.build.image.label')}
        description={t('library.build.image.catalog', {
          image: catalogImage(cat, db.version) || '—',
        })}
        invalid={invalid}
        error={invalid ? t('library.build.image.spaces') : undefined}
        noMargin
      >
        <Input
          value={value}
          placeholder={catalogImage(cat, db.version)}
          disabled={!canEdit}
          onChange={(e) => setValue(e.currentTarget.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && dirty && !invalid) void save(trimmed)
          }}
        />
      </Field>
    </Panel>
  )
}

function RuntimePanel({
  db,
  canEdit,
  onSave,
}: {
  db: Database
  canEdit: boolean
  onSave: (p: Schemas['DatabasePatch']) => Promise<unknown>
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const stored = useMemo(
    () => (isEmptyRuntime(db.runtime) ? {} : (db.runtime as Runtime)),
    [db.runtime]
  )
  const [mode, setMode] = useState<'form' | 'yaml'>('form')
  const [draft, setDraft] = useState<Runtime>(stored)
  const [text, setText] = useState('')
  const [formKey, setFormKey] = useState(0)
  const [saving, setSaving] = useState(false)
  const parsed = useMemo(() => parseRuntime(text, t), [text, t])
  const current = mode === 'form' ? draft : parsed.value
  const dirty = current !== undefined && JSON.stringify(current) !== JSON.stringify(stored)
  const hasValue = !!current && Object.keys(current).length > 0
  const { errors, checking } = useSchemaValidate(
    RUNTIME_SCHEMA,
    undefined,
    current,
    dirty && hasValue
  )
  const issues = dirty && hasValue ? Object.entries(errors) : []
  // In the form an issue sits under its field; only what the form can't show is listed.
  const listed = mode === 'form' ? issues.filter(([p]) => !formPath(p)) : issues
  const blocked = (mode === 'yaml' && !!parsed.error) || issues.length > 0
  const other = Object.keys(draft).filter((k) => !(FORM_KEYS as readonly string[]).includes(k))

  const switchMode = (next: 'form' | 'yaml') => {
    if (next === mode) return
    if (next === 'yaml') {
      setText(Object.keys(draft).length ? toYaml(draft) : '')
      setMode('yaml')
      return
    }
    if (!parsed.value) return
    setDraft(parsed.value)
    setFormKey((k) => k + 1)
    setMode('form')
  }
  const reset = () => {
    setDraft(stored)
    setText(Object.keys(stored).length ? toYaml(stored) : '')
    setFormKey((k) => k + 1)
  }
  const insert = (example: string) => {
    const base = parsed.value ?? {}
    setText(toYaml(mergeRuntime(base, parseDocument(example) as Runtime)))
  }
  const save = async () => {
    if (!current) return
    setSaving(true)
    try {
      await onSave({ runtime: current })
      setDraft(current)
      toast.success(t('library.build.runtime.saved'))
    } catch (e) {
      toast.error(e)
    } finally {
      setSaving(false)
    }
  }

  return (
    <Panel
      title={t('library.build.runtime.title')}
      description={t('library.build.runtime.hint')}
      actions={
        <Stack gap={1} alignItems="center">
          <RadioButtonGroup
            size="sm"
            options={[
              { label: t('library.build.runtime.modeForm'), value: 'form' },
              { label: 'YAML', value: 'yaml' },
            ]}
            value={mode}
            onChange={switchMode}
          />
          <Button size="sm" variant="secondary" disabled={!dirty || saving} onClick={reset}>
            {t('common.actions.reset')}
          </Button>
          <Button
            size="sm"
            disabled={!canEdit || !dirty || blocked || checking || saving}
            onClick={() => void save()}
          >
            {t('common.actions.save')}
          </Button>
        </Stack>
      }
    >
      {mode === 'form' ? (
        <>
          <RuntimeForm
            key={formKey}
            value={draft}
            onChange={setDraft}
            topology={db.topology_preview}
            readOnly={!canEdit}
            errors={Object.fromEntries(issues)}
          />
          {other.length > 0 && (
            <Text color="secondary" variant="bodySmall">
              {t('library.build.runtime.otherKeys', { keys: other.join(', ') })}
            </Text>
          )}
        </>
      ) : (
        <div className={styles.yaml}>
          <div>
            <CodeEditorLazy
              value={text}
              language="yaml"
              height={520}
              readOnly={!canEdit}
              onChange={setText}
              onBlur={setText}
            />
            {parsed.error && (
              <Text color="error">{t('library.build.runtime.parse', { error: parsed.error })}</Text>
            )}
          </div>
          <RuntimeHelp onInsert={canEdit ? insert : undefined} />
        </div>
      )}
      <div className={styles.issues}>
        {listed.length > 0 ? (
          listed.map(([path, message]) => (
            <Text key={path} color="error">
              <span className={styles.mono}>{path || '·'}</span> {message}
            </Text>
          ))
        ) : issues.length > 0 ? (
          <Text color="error">{t('library.build.runtime.fixAbove', { count: issues.length })}</Text>
        ) : (
          <Text color="secondary">
            {!hasValue
              ? t('library.build.runtime.empty')
              : checking
                ? t('schema.validating')
                : dirty
                  ? t('library.build.runtime.valid')
                  : t('library.build.runtime.applied')}
          </Text>
        )}
      </div>
    </Panel>
  )
}

function parseRuntime(text: string, t: (k: string) => string): { value?: Runtime; error?: string } {
  if (!text.trim()) return { value: {} }
  try {
    const v = parseDocument(text)
    if (!v || typeof v !== 'object' || Array.isArray(v))
      return { error: t('library.build.runtime.notMap') }
    return { value: v as Runtime }
  } catch (e) {
    return { error: e instanceof Error ? e.message : String(e) }
  }
}

/** An example added to what is already there: lists append, maps merge. */
function mergeRuntime(base: Runtime, add: Runtime): Runtime {
  const out: Runtime = { ...base }
  for (const [k, v] of Object.entries(add)) {
    const prev = out[k]
    if (Array.isArray(prev) && Array.isArray(v)) out[k] = [...prev, ...v]
    else if (prev && typeof prev === 'object' && v && typeof v === 'object')
      out[k] = { ...(prev as Runtime), ...(v as Runtime) }
    else out[k] = v
  }
  return out
}
