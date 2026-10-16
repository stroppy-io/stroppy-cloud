import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Button, SecretInput, Text, useStyles2 } from '@grafana/ui'
import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { Widget } from './SchemaForm'

const getStyles = (theme: GrafanaTheme2) => ({
  row: css({ display: 'flex', gap: theme.spacing(1), alignItems: 'center', flexWrap: 'wrap' }),
  hidden: css({ display: 'none' }),
})

// A secret that usually comes as a file (a cloud key): paste it or pick the file, which is read in
// the browser and never shown.
function SecretFileInput({
  id,
  value,
  accept,
  disabled,
  onChange,
}: {
  id: string
  value: string
  accept?: string
  disabled?: boolean
  onChange: (v: string) => void
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const input = useRef<HTMLInputElement>(null)
  const [fileName, setFileName] = useState<string>()
  const [failed, setFailed] = useState(false)
  const handleFile = async (file: File | undefined) => {
    if (!file) return
    try {
      onChange((await file.text()).trim())
      setFileName(file.name)
      setFailed(false)
    } catch {
      setFailed(true)
    }
  }
  return (
    <div className={styles.row}>
      <SecretInput
        id={id}
        value={value}
        isConfigured={false}
        disabled={disabled}
        onReset={() => {
          onChange('')
          setFileName(undefined)
        }}
        onChange={(e) => {
          onChange(e.currentTarget.value)
          setFileName(undefined)
        }}
        width={30}
      />
      <Button
        variant="secondary"
        icon="upload"
        disabled={disabled}
        onClick={() => input.current?.click()}
      >
        {t('schema.fromFile')}
      </Button>
      <input
        ref={input}
        className={styles.hidden}
        type="file"
        accept={accept}
        onChange={(e) => {
          void handleFile(e.currentTarget.files?.[0])
          e.currentTarget.value = ''
        }}
      />
      {fileName && (
        <Text variant="bodySmall" color="secondary">
          {t('schema.fileLoaded', { name: fileName })}
        </Text>
      )}
      {failed && (
        <Text variant="bodySmall" color="error">
          {t('schema.fileUnreadable')}
        </Text>
      )}
    </div>
  )
}

// secretFileWidget renders a schema string field as SecretFileInput.
export function secretFileWidget(accept?: string): Widget {
  return ({ path, value, onChange, readOnly }) => (
    <SecretFileInput
      id={path}
      value={(value as string) ?? ''}
      accept={accept}
      disabled={readOnly}
      onChange={onChange}
    />
  )
}
