import { type ExportFormat, publicMutations, type ShareSnapshot } from '@api/queries/results'
import { toast } from '@app/Toaster'
import { useThemeMode } from '@app/theme-context'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import {
  Badge,
  Button,
  ButtonGroup,
  Dropdown,
  Icon,
  IconButton,
  Menu,
  Text,
  useStyles2,
} from '@grafana/ui'
import { formatDateTime } from '@helpers/time'
import { type Lang, setLang } from '@lib/i18n'
import { useMutation } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

const getStyles = (theme: GrafanaTheme2) => ({
  header: css({
    position: 'sticky',
    top: 0,
    zIndex: theme.zIndex.navbarFixed,
    background: theme.colors.background.primary,
    borderBottom: `1px solid ${theme.colors.border.weak}`,
    '@media print': { position: 'static', borderBottom: 0 },
  }),
  inner: css({
    maxWidth: 1100,
    margin: '0 auto',
    padding: theme.spacing(1, 3),
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(2),
    flexWrap: 'wrap',
    [theme.breakpoints.down('md')]: { padding: theme.spacing(1, 2) },
  }),
  brand: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    fontWeight: theme.typography.fontWeightMedium,
    color: 'inherit',
    textDecoration: 'none',
    whiteSpace: 'nowrap',
    '&:hover': { color: theme.colors.text.link },
  }),
  logo: css({
    width: 24,
    height: 24,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.primary.main,
    color: theme.colors.primary.contrastText,
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
  }),
  meta: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    flexWrap: 'wrap',
    minWidth: 0,
  }),
  actions: css({
    marginLeft: 'auto',
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    '@media print': { display: 'none' },
  }),
})

const EXT: Record<ExportFormat, string> = { md: 'md', json: 'json', csv: 'csv', pdf: 'md' }

function download(name: string, text: string, mime: string) {
  const blob = new Blob([text], { type: mime })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = name
  document.body.appendChild(a)
  a.click()
  a.remove()
  window.setTimeout(() => URL.revokeObjectURL(url), 1000)
}

export function PublicHeader({ token, snapshot }: { token: string; snapshot?: ShareSnapshot }) {
  const { t, i18n } = useTranslation()
  const styles = useStyles2(getStyles)
  const { mode, setMode } = useThemeMode()
  const exp = useMutation({
    mutationFn: (format: ExportFormat) => publicMutations.export(token, format),
    onSuccess: ({ text, mime }, format) => {
      const base = (snapshot?.title ?? snapshot?.run?.name ?? snapshot?.suite_run?.name ?? token)
        .replace(/[^\w\d-]+/g, '-')
        .replace(/^-+|-+$/g, '')
        .toLowerCase()
      download(`${base || 'report'}.${EXT[format]}`, text, mime)
      toast.success(t('public.export.done'), {
        description: format === 'pdf' ? t('public.export.pdfNote') : undefined,
      })
    },
    onError: (e: Error) => toast.error(e, { title: t('public.export.failed') }),
  })
  const other: Lang = i18n.language.startsWith('ru') ? 'en' : 'ru'
  return (
    <header className={styles.header}>
      <div className={styles.inner}>
        <a href="/" className={styles.brand}>
          <span className={styles.logo}>
            <Icon name="database" size="sm" />
          </span>
          {t('common.appName')}
        </a>
        <div className={styles.meta}>
          <Badge text={t('public.sharedReport')} color="blue" icon="share-alt" />
          {snapshot && (
            <>
              <Text color="secondary" variant="bodySmall">
                {t(`public.kind.${snapshot.kind}`)} · {t(`public.scope.${snapshot.scope}`)}
              </Text>
              <Text color="secondary" variant="bodySmall">
                · {t('public.captured', { when: formatDateTime(snapshot.captured_at) })}
              </Text>
              {snapshot.tenant_name && (
                <Text color="secondary" variant="bodySmall">
                  · {t('public.by', { tenant: snapshot.tenant_name })}
                </Text>
              )}
            </>
          )}
        </div>
        <div className={styles.actions}>
          {snapshot && (
            <ButtonGroup>
              <Button
                size="sm"
                variant="secondary"
                icon="download-alt"
                disabled={exp.isPending}
                onClick={() => exp.mutate('md')}
              >
                {t('public.export.md')}
              </Button>
              <Dropdown
                overlay={
                  <Menu>
                    <Menu.Item
                      label={t('public.export.json')}
                      icon="brackets-curly"
                      onClick={() => exp.mutate('json')}
                    />
                    <Menu.Item
                      label={t('public.export.csv')}
                      icon="table"
                      onClick={() => exp.mutate('csv')}
                    />
                    <Menu.Item
                      label={t('public.export.pdf')}
                      icon="file-alt"
                      description={t('public.export.pdfNote')}
                      onClick={() => exp.mutate('pdf')}
                    />
                  </Menu>
                }
              >
                <Button
                  size="sm"
                  variant="secondary"
                  icon="angle-down"
                  aria-label={t('public.export.title')}
                />
              </Dropdown>
            </ButtonGroup>
          )}
          <IconButton
            name="document-info"
            size="lg"
            tooltip={t('public.print')}
            onClick={() => window.print()}
          />
          <IconButton
            name={mode === 'dark' ? 'lightbulb-alt' : 'adjust-circle'}
            size="lg"
            tooltip={mode === 'dark' ? t('common.theme.light') : t('common.theme.dark')}
            onClick={() => setMode(mode === 'dark' ? 'light' : 'dark')}
          />
          <Button size="sm" variant="secondary" fill="text" onClick={() => setLang(other)}>
            {other.toUpperCase()}
          </Button>
        </div>
      </div>
    </header>
  )
}
