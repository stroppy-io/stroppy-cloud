import type { Schemas } from '@api/types'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import {
  Button,
  Combobox,
  Field,
  IconButton,
  Input,
  RadioButtonGroup,
  Stack,
  Text,
  TextArea,
  useStyles2,
} from '@grafana/ui'
import { type ComponentProps, createContext, useContext, useState } from 'react'
import { useTranslation } from 'react-i18next'

// The form side of test.runtime@1: the three things people change about a
// deployment — a container we make, a step on the machine before it starts,
// a container of their own beside it. Anything else in the overlay stays as
// it is and is edited as YAML.

export type Runtime = Record<string, unknown>
type Obj = Record<string, unknown>
type Topology = Schemas['TopologyPreview'] | undefined

/** Keys the form edits; the rest of an overlay is kept untouched. */
export const FORM_KEYS = ['containers', 'host_prep', 'additional_containers'] as const

const getStyles = (theme: GrafanaTheme2) => ({
  section: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1.5) }),
  sectionHead: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(0.5) }),
  card: css({
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    padding: theme.spacing(1.5, 2),
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(1.5),
    background: theme.colors.background.secondary,
  }),
  cardHead: css({ display: 'flex', alignItems: 'flex-end', gap: theme.spacing(2) }),
  grow: css({ flex: 1, minWidth: 0 }),
  grid: css({
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fit, minmax(260px, 1fr))',
    gap: theme.spacing(1.5, 2),
  }),
  mono: css({ fontFamily: theme.typography.fontFamilyMonospace }),
  pair: css({
    display: 'grid',
    gridTemplateColumns: 'minmax(0, 1fr) minmax(0, 2fr) auto',
    gap: theme.spacing(1),
    alignItems: 'center',
  }),
})

// ---------- what a container override can point at ----------

interface Target {
  key: string
  role?: string
  kind: string
  label: string
}

const RUNNER = 'runner'

function kindOfRole(role: string): string | undefined {
  if (role === RUNNER) return undefined
  if (role.includes('proxy') || role.includes('bouncer')) return 'proxy'
  if (role.includes('etcd') || role.includes('coordinator')) return 'coordinator'
  return 'database'
}

function targetsOf(topology: Topology, t: (k: string, o?: Obj) => string): Target[] {
  const out: Target[] = []
  for (const n of topology?.nodes ?? []) {
    const kind = kindOfRole(n.role)
    if (!kind || out.some((x) => x.role === n.role)) continue
    out.push({
      key: `${n.role}|${kind}`,
      role: n.role,
      kind,
      label: t(`library.build.target.${kind}`, { role: n.role, engine: n.engine ?? '' }),
    })
  }
  out.push({ key: '|exporter', kind: 'exporter', label: t('library.build.target.exporters') })
  return out
}

function targetKey(o: Obj): string {
  return `${(o.role as string) ?? ''}|${(o.kind as string) ?? ''}`
}

/** Machine names the compiler gives: `<role>-<n>`. */
function machinesOf(topology: Topology): { name: string; role: string }[] {
  return (topology?.nodes ?? []).flatMap((n) =>
    Array.from({ length: Math.max(1, n.count) }, (_, i) => ({
      name: `${n.role}-${i + 1}`,
      role: n.role,
    }))
  )
}

function rolesOf(topology: Topology): string[] {
  return [...new Set((topology?.nodes ?? []).map((n) => n.role))]
}

// ---------- helpers over plain objects ----------

const list = (v: unknown): Obj[] => (Array.isArray(v) ? (v as Obj[]) : [])
const obj = (v: unknown): Obj => (v && typeof v === 'object' && !Array.isArray(v) ? (v as Obj) : {})

/** Drops empty values so an untouched field stays out of the overlay. */
function clean(o: Obj): Obj {
  const out: Obj = {}
  for (const [k, v] of Object.entries(o)) {
    if (v === undefined || v === '') continue
    if (Array.isArray(v) && v.length === 0) continue
    if (v && typeof v === 'object' && !Array.isArray(v) && Object.keys(v).length === 0) continue
    out[k] = v
  }
  return out
}

function setList(rt: Runtime, key: string, items: Obj[]): Runtime {
  const next = { ...rt }
  if (items.length) next[key] = items
  else delete next[key]
  return next
}

// ---------- errors by path ----------

/** Server issues by overlay path ("containers[0].set.image" → message). */
const Issues = createContext<Record<string, string>>({})

/** The first issue at this path or inside it. */
function useIssue(path: string): string | undefined {
  const issues = useContext(Issues)
  for (const [p, m] of Object.entries(issues))
    if (p === path || p.startsWith(`${path}.`) || p.startsWith(`${path}[`)) return m
  return undefined
}

/** Grafana Field that shows the server issue of its overlay path. */
function PathField({
  path,
  children,
  ...rest
}: {
  path: string
  children: ComponentProps<typeof Field>['children']
  label?: string
  description?: string
}) {
  const issue = useIssue(path)
  return (
    <Field {...rest} invalid={!!issue} error={issue} noMargin>
      {children}
    </Field>
  )
}

/** Paths the form shows next to a field; the rest is listed under the form. */
export function formPath(path: string): boolean {
  return /^(containers|host_prep|additional_containers)\[\d+\]/.test(path)
}

// ---------- the form ----------

export function RuntimeForm({
  value,
  onChange,
  topology,
  readOnly,
  errors,
}: {
  value: Runtime
  onChange: (v: Runtime) => void
  topology: Topology
  readOnly: boolean
  errors: Record<string, string>
}) {
  const { t } = useTranslation()
  const containers = list(value.containers)
  const steps = list(value.host_prep)
  const extra = list(value.additional_containers)
  const update = (key: string, items: Obj[]) => onChange(setList(value, key, items))
  const targets = targetsOf(topology, t)
  return (
    <Issues.Provider value={errors}>
      <Stack direction="column" gap={4}>
        <Section
          title={t('library.build.containers.title')}
          hint={t('library.build.containers.hint')}
          addLabel={t('library.build.containers.add')}
          readOnly={readOnly}
          onAdd={() =>
            update('containers', [
              ...containers,
              { role: targets[0]?.role, kind: targets[0]?.kind ?? 'database', set: {} },
            ])
          }
        >
          {containers.map((c, i) => (
            <ContainerOverrideCard
              key={i}
              path={`containers[${i}]`}
              value={c}
              targets={targets}
              readOnly={readOnly}
              onChange={(next) =>
                update(
                  'containers',
                  containers.map((x, j) => (j === i ? next : x))
                )
              }
              onRemove={() =>
                update(
                  'containers',
                  containers.filter((_, j) => j !== i)
                )
              }
            />
          ))}
        </Section>

        <Section
          title={t('library.build.steps.title')}
          hint={t('library.build.steps.hint')}
          addLabel={t('library.build.steps.add')}
          readOnly={readOnly}
          onAdd={() =>
            update('host_prep', [
              ...steps,
              { role: rolesOf(topology)[0] ?? 'db', kind: 'sysctl', content: '' },
            ])
          }
        >
          {steps.map((s, i) => (
            <HostStepCard
              key={i}
              path={`host_prep[${i}]`}
              value={s}
              roles={rolesOf(topology)}
              readOnly={readOnly}
              onChange={(next) =>
                update(
                  'host_prep',
                  steps.map((x, j) => (j === i ? next : x))
                )
              }
              onRemove={() =>
                update(
                  'host_prep',
                  steps.filter((_, j) => j !== i)
                )
              }
            />
          ))}
        </Section>

        <Section
          title={t('library.build.extra.title')}
          hint={t('library.build.extra.hint')}
          addLabel={t('library.build.extra.add')}
          readOnly={readOnly}
          onAdd={() => {
            const m = machinesOf(topology)[0]
            update('additional_containers', [
              ...extra,
              {
                name: '',
                role: m?.role ?? 'db',
                machine: m?.name ?? 'db-1',
                image: '',
                kind: 'exporter',
              },
            ])
          }}
        >
          {extra.map((c, i) => (
            <ExtraContainerCard
              key={i}
              path={`additional_containers[${i}]`}
              value={c}
              machines={machinesOf(topology)}
              readOnly={readOnly}
              onChange={(next) =>
                update(
                  'additional_containers',
                  extra.map((x, j) => (j === i ? next : x))
                )
              }
              onRemove={() =>
                update(
                  'additional_containers',
                  extra.filter((_, j) => j !== i)
                )
              }
            />
          ))}
        </Section>
      </Stack>
    </Issues.Provider>
  )
}

function Section({
  title,
  hint,
  addLabel,
  readOnly,
  onAdd,
  children,
}: {
  title: string
  hint: string
  addLabel: string
  readOnly: boolean
  onAdd: () => void
  children: React.ReactNode
}) {
  const styles = useStyles2(getStyles)
  return (
    <section className={styles.section}>
      <div className={styles.sectionHead}>
        <Text element="h4" weight="medium">
          {title}
        </Text>
        <Text color="secondary" variant="bodySmall">
          {hint}
        </Text>
      </div>
      {children}
      {!readOnly && (
        <div>
          <Button size="sm" variant="secondary" icon="plus" onClick={onAdd}>
            {addLabel}
          </Button>
        </div>
      )}
    </section>
  )
}

function CardHead({
  children,
  onRemove,
  readOnly,
}: {
  children: React.ReactNode
  onRemove: () => void
  readOnly: boolean
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  return (
    <div className={styles.cardHead}>
      <div className={styles.grow}>{children}</div>
      {!readOnly && (
        <IconButton name="trash-alt" tooltip={t('common.actions.delete')} onClick={onRemove} />
      )}
    </div>
  )
}

// ---------- a container we make ----------

function ContainerOverrideCard({
  path,
  value,
  targets,
  readOnly,
  onChange,
  onRemove,
}: {
  path: string
  value: Obj
  targets: Target[]
  readOnly: boolean
  onChange: (v: Obj) => void
  onRemove: () => void
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const set = obj(value.set)
  const patch = (fields: Obj) => onChange({ ...value, set: clean({ ...set, ...fields }) })
  const key = targetKey(value)
  const options = targets.some((x) => x.key === key)
    ? targets
    : [...targets, { key, role: value.role as string, kind: value.kind as string, label: key }]
  return (
    <div className={styles.card}>
      <CardHead onRemove={onRemove} readOnly={readOnly}>
        <PathField path={path} label={t('library.build.containers.target')}>
          <Combobox
            options={options.map((o) => ({ label: o.label, value: o.key }))}
            value={key}
            disabled={readOnly}
            onChange={(o) => {
              const tg = options.find((x) => x.key === o.value)
              if (!tg) return
              const next: Obj = { ...value, kind: tg.kind }
              if (tg.role) next.role = tg.role
              else delete next.role
              onChange(next)
            }}
          />
        </PathField>
      </CardHead>
      <div className={styles.grid}>
        <PathField
          path={`${path}.set.image`}
          label={t('library.build.containers.image')}
          description={t('library.build.containers.imageHint')}
        >
          <Input
            value={(set.image as string) ?? ''}
            placeholder={t('library.build.containers.imagePlaceholder')}
            disabled={readOnly}
            onChange={(e) => patch({ image: e.currentTarget.value.trim() })}
          />
        </PathField>
        <PathField
          path={`${path}.set.cmd`}
          label={t('library.build.containers.cmd')}
          description={t('library.build.containers.cmdHint')}
        >
          <ArgsInput
            value={list(set.cmd) as unknown as string[]}
            readOnly={readOnly}
            placeholder={t('library.build.containers.imagePlaceholder')}
            onChange={(cmd) => patch({ cmd })}
          />
        </PathField>
      </div>
      <PathField
        path={`${path}.set.env`}
        label={t('library.build.env.label')}
        description={t('library.build.env.hint')}
      >
        <KeyValueEditor
          value={obj(set.env) as Record<string, string>}
          readOnly={readOnly}
          onChange={(env) => patch({ env })}
        />
      </PathField>
      <PathField
        path={`${path}.set.files`}
        label={t('library.build.files.label')}
        description={t('library.build.files.hint')}
      >
        <FilesEditor
          value={list(set.files)}
          readOnly={readOnly}
          onChange={(files) => patch({ files })}
        />
      </PathField>
    </div>
  )
}

// ---------- a step on the machine ----------

const STEP_PLACEHOLDER: Record<string, string> = {
  sysctl: 'vm.swappiness = 1\nnet.core.somaxconn = 4096',
  script: '#!/bin/sh\nset -e\necho never > /sys/kernel/mm/transparent_hugepage/enabled',
  disks: '',
}

function HostStepCard({
  path,
  value,
  roles,
  readOnly,
  onChange,
  onRemove,
}: {
  path: string
  value: Obj
  roles: string[]
  readOnly: boolean
  onChange: (v: Obj) => void
  onRemove: () => void
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const kind = (value.kind as string) ?? 'sysctl'
  const kinds = ['sysctl', 'script', ...(kind === 'disks' ? ['disks'] : [])]
  return (
    <div className={styles.card}>
      <CardHead onRemove={onRemove} readOnly={readOnly}>
        <Stack gap={2} wrap="wrap" alignItems="flex-end">
          <PathField path={`${path}.role`} label={t('library.build.steps.role')}>
            <Combobox
              width={24}
              options={roles.map((r) => ({ label: r, value: r }))}
              value={(value.role as string) ?? null}
              disabled={readOnly}
              onChange={(o) => onChange({ ...value, role: o.value })}
            />
          </PathField>
          <PathField path={`${path}.kind`} label={t('library.build.steps.kind')}>
            <RadioButtonGroup
              options={kinds.map((k) => ({ label: t(`library.build.steps.kinds.${k}`), value: k }))}
              value={kind}
              disabled={readOnly}
              onChange={(k) => onChange({ ...value, kind: k })}
            />
          </PathField>
        </Stack>
      </CardHead>
      <PathField
        path={`${path}.content`}
        description={t(`library.build.steps.contentHint.${kind}`)}
      >
        <TextArea
          className={styles.mono}
          rows={5}
          value={(value.content as string) ?? ''}
          placeholder={STEP_PLACEHOLDER[kind]}
          disabled={readOnly}
          onChange={(e) => onChange({ ...value, content: e.currentTarget.value })}
        />
      </PathField>
    </div>
  )
}

// ---------- a container of their own ----------

function ExtraContainerCard({
  path,
  value,
  machines,
  readOnly,
  onChange,
  onRemove,
}: {
  path: string
  value: Obj
  machines: { name: string; role: string }[]
  readOnly: boolean
  onChange: (v: Obj) => void
  onRemove: () => void
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const patch = (fields: Obj) => onChange(clean({ ...value, ...fields }))
  const ports = list(value.ports).map((p) => Number(p.container))
  return (
    <div className={styles.card}>
      <CardHead onRemove={onRemove} readOnly={readOnly}>
        <div className={styles.grid}>
          <PathField path={`${path}.name`} label={t('library.build.extra.name')}>
            <Input
              value={(value.name as string) ?? ''}
              placeholder="my-exporter"
              disabled={readOnly}
              onChange={(e) => patch({ name: e.currentTarget.value.trim() })}
            />
          </PathField>
          <PathField path={`${path}.machine`} label={t('library.build.extra.machine')}>
            <Combobox
              options={machines.map((m) => ({ label: m.name, value: m.name, description: m.role }))}
              value={(value.machine as string) ?? null}
              disabled={readOnly}
              onChange={(o) =>
                patch({ machine: o.value, role: machines.find((m) => m.name === o.value)?.role })
              }
            />
          </PathField>
        </div>
      </CardHead>
      <div className={styles.grid}>
        <PathField path={`${path}.image`} label={t('library.build.extra.image')}>
          <Input
            value={(value.image as string) ?? ''}
            placeholder="prom/blackbox-exporter:v0.25.0"
            disabled={readOnly}
            onChange={(e) => patch({ image: e.currentTarget.value.trim() })}
          />
        </PathField>
        <PathField
          path={`${path}.ports`}
          label={t('library.build.extra.ports')}
          description={t('library.build.extra.portsHint')}
        >
          <PortsInput value={ports} readOnly={readOnly} onChange={(p) => patch({ ports: p })} />
        </PathField>
        <PathField
          path={`${path}.cmd`}
          label={t('library.build.containers.cmd')}
          description={t('library.build.extra.cmdHint')}
        >
          <ArgsInput
            value={list(value.cmd) as unknown as string[]}
            readOnly={readOnly}
            placeholder={'--config.file=/etc/blackbox.yml'}
            onChange={(cmd) => patch({ cmd })}
          />
        </PathField>
      </div>
      <PathField path={`${path}.env`} label={t('library.build.env.label')}>
        <KeyValueEditor
          value={obj(value.env) as Record<string, string>}
          readOnly={readOnly}
          onChange={(env) => patch({ env })}
        />
      </PathField>
    </div>
  )
}

// ---------- small editors ----------

/** Ports as typed text: "9115, 9116" keeps its comma while the user types. */
function PortsInput({
  value,
  readOnly,
  onChange,
}: {
  value: number[]
  readOnly: boolean
  onChange: (v: Obj[]) => void
}) {
  const [text, setText] = useState(value.join(', '))
  return (
    <Input
      value={text}
      placeholder="9115"
      disabled={readOnly}
      onChange={(e) => {
        setText(e.currentTarget.value)
        onChange(
          e.currentTarget.value
            .split(/[\s,]+/)
            .map(Number)
            .filter((n) => Number.isInteger(n) && n > 0)
            .map((n) => ({ container: n, host: n }))
        )
      }}
    />
  )
}

/** One argument per line: an argument may hold spaces, a line break never. */
function ArgsInput({
  value,
  placeholder,
  readOnly,
  onChange,
}: {
  value: string[]
  placeholder: string
  readOnly: boolean
  onChange: (v: string[]) => void
}) {
  const styles = useStyles2(getStyles)
  const [text, setText] = useState(value.join('\n'))
  return (
    <TextArea
      className={styles.mono}
      rows={3}
      value={text}
      placeholder={placeholder}
      disabled={readOnly}
      onChange={(e) => {
        setText(e.currentTarget.value)
        onChange(e.currentTarget.value.split('\n').filter((l) => l.trim() !== ''))
      }}
    />
  )
}

function KeyValueEditor({
  value,
  readOnly,
  onChange,
}: {
  value: Record<string, string>
  readOnly: boolean
  onChange: (v: Record<string, string>) => void
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const [rows, setRows] = useState<[string, string][]>(() => Object.entries(value))
  const emit = (next: [string, string][]) => {
    setRows(next)
    const out: Record<string, string> = {}
    for (const [k, v] of next) if (k.trim()) out[k.trim()] = v
    onChange(out)
  }
  return (
    <Stack direction="column" gap={1}>
      {rows.map(([k, v], i) => (
        <div key={i} className={styles.pair}>
          <Input
            className={styles.mono}
            value={k}
            placeholder="KEY"
            disabled={readOnly}
            onChange={(e) =>
              emit(rows.map((r, j) => (j === i ? [e.currentTarget.value, r[1]] : r)))
            }
          />
          <Input
            className={styles.mono}
            value={v}
            placeholder="value"
            disabled={readOnly}
            onChange={(e) =>
              emit(rows.map((r, j) => (j === i ? [r[0], e.currentTarget.value] : r)))
            }
          />
          {!readOnly && (
            <IconButton
              name="times"
              tooltip={t('common.actions.delete')}
              onClick={() => emit(rows.filter((_, j) => j !== i))}
            />
          )}
        </div>
      ))}
      {!readOnly && (
        <div>
          <Button
            size="sm"
            variant="secondary"
            fill="text"
            icon="plus"
            onClick={() => setRows([...rows, ['', '']])}
          >
            {t('library.build.env.add')}
          </Button>
        </div>
      )}
    </Stack>
  )
}

function FilesEditor({
  value,
  readOnly,
  onChange,
}: {
  value: Obj[]
  readOnly: boolean
  onChange: (v: Obj[]) => void
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const edit = (i: number, fields: Obj) =>
    onChange(value.map((f, j) => (j === i ? { ...f, ...fields } : f)))
  return (
    <Stack direction="column" gap={1}>
      {value.map((f, i) => (
        <Stack key={i} direction="column" gap={0.5}>
          <div className={styles.pair}>
            <Input
              className={styles.mono}
              value={(f.path as string) ?? ''}
              placeholder="/etc/postgresql/conf.d/90-mine.conf"
              disabled={readOnly}
              onChange={(e) => edit(i, { path: e.currentTarget.value.trim() })}
            />
            <Input
              className={styles.mono}
              value={(f.mode as string) ?? ''}
              placeholder="0644"
              disabled={readOnly}
              onChange={(e) => edit(i, { mode: e.currentTarget.value.trim() || undefined })}
            />
            {!readOnly && (
              <IconButton
                name="times"
                tooltip={t('common.actions.delete')}
                onClick={() => onChange(value.filter((_, j) => j !== i))}
              />
            )}
          </div>
          <TextArea
            className={styles.mono}
            rows={4}
            value={(f.content as string) ?? ''}
            placeholder={t('library.build.files.contentPlaceholder')}
            disabled={readOnly}
            onChange={(e) => edit(i, { content: e.currentTarget.value })}
          />
        </Stack>
      ))}
      {!readOnly && (
        <div>
          <Button
            size="sm"
            variant="secondary"
            fill="text"
            icon="plus"
            onClick={() => onChange([...value, { path: '', content: '' }])}
          >
            {t('library.build.files.add')}
          </Button>
        </div>
      )}
    </Stack>
  )
}
