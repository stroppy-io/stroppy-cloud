import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Button, Text, useStyles2 } from '@grafana/ui'
import { useTranslation } from 'react-i18next'

// What can go into the run tweaks, key by key: what it changes, and a piece
// of YAML that works as written. The examples follow test.runtime@1.

const EXAMPLES: { key: string; yaml: string }[] = [
  {
    key: 'containers',
    yaml: `containers:
  - role: db
    kind: database
    set:
      image: ghcr.io/acme/postgres:17-patched
      env:
        TZ: UTC
      files:
        - path: /etc/postgresql/conf.d/90-mine.conf
          content: |
            jit = off`,
  },
  {
    key: 'host_prep',
    yaml: `host_prep:
  - role: db
    kind: sysctl
    content: |
      vm.swappiness = 1
  - role: db
    kind: script
    content: |
      echo never > /sys/kernel/mm/transparent_hugepage/enabled`,
  },
  {
    key: 'additional_containers',
    yaml: `additional_containers:
  - name: blackbox
    role: db
    machine: db-1
    kind: exporter
    image: prom/blackbox-exporter:v0.25.0
    ports:
      - { container: 9115, host: 9115 }`,
  },
  {
    key: 'machines',
    yaml: `machines:
  db-1:
    cpu: 8
    memory_gb: 32`,
  },
  {
    key: 'scrapes',
    yaml: `scrapes:
  - role: db
    job: blackbox
    url: http://127.0.0.1:9115/metrics`,
  },
]

const getStyles = (theme: GrafanaTheme2) => ({
  root: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(2) }),
  item: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(0.5) }),
  head: css({ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }),
  key: css({ fontFamily: theme.typography.fontFamilyMonospace, fontWeight: 600 }),
  code: css({
    margin: 0,
    padding: theme.spacing(1),
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.canvas,
    border: `1px solid ${theme.colors.border.weak}`,
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
    whiteSpace: 'pre',
    overflowX: 'auto',
  }),
})

export function RuntimeHelp({ onInsert }: { onInsert?: (yaml: string) => void }) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  return (
    <div className={styles.root}>
      <Text color="secondary" variant="bodySmall">
        {t('library.build.help.intro')}
      </Text>
      {EXAMPLES.map((ex) => (
        <div key={ex.key} className={styles.item}>
          <div className={styles.head}>
            <span className={styles.key}>{ex.key}</span>
            {onInsert && (
              <Button
                size="sm"
                variant="secondary"
                fill="text"
                icon="plus"
                onClick={() => onInsert(ex.yaml)}
              >
                {t('library.build.help.insert')}
              </Button>
            )}
          </div>
          <Text color="secondary" variant="bodySmall">
            {t(`library.build.help.keys.${ex.key}`)}
          </Text>
          <pre className={styles.code}>{ex.yaml}</pre>
        </div>
      ))}
      <Text color="secondary" variant="bodySmall">
        {t('library.build.help.merge')}
      </Text>
    </div>
  )
}
