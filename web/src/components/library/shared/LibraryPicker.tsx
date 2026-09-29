import { type LibraryKind, libraryByKind } from '@api/queries/library'
import { Combobox, type ComboboxOption } from '@grafana/ui'
import { useTenant } from '@hooks/useTenant'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

// Combobox over library records of one kind (server-side search).
export function LibraryPicker({
  kind,
  value,
  onChange,
  id,
  width = 40,
  excludeId,
  disabled,
  autoFocus,
}: {
  kind: LibraryKind
  value: string | undefined | null
  onChange: (id: string | null, option?: ComboboxOption<string>) => void
  id?: string
  width?: number
  excludeId?: string
  disabled?: boolean
  autoFocus?: boolean
}) {
  const { t } = useTranslation()
  const { slug } = useTenant()
  const all = useQuery(libraryByKind[kind].options(slug))
  const toOption = (e: {
    id: string
    name: string
    description?: string
    kind?: string
    version?: string
    protocol?: string
    stroppy_version?: string
  }): ComboboxOption<string> => ({
    label: e.name,
    value: e.id,
    description:
      e.description ??
      (e.kind
        ? `${e.kind} ${e.version ?? ''}`
        : e.protocol
          ? `${e.protocol} · stroppy ${e.stroppy_version ?? ''}`
          : undefined),
  })
  const options = (all.data?.data ?? []).filter((e) => e.id !== excludeId).map(toOption)
  return (
    <Combobox
      id={id}
      width={width}
      isClearable
      disabled={disabled}
      autoFocus={autoFocus}
      loading={all.isPending}
      placeholder={t(`library.picker.${kind}`)}
      noOptionsMessage={t('library.picker.none')}
      options={options}
      value={value ?? null}
      onChange={(o) => onChange(o?.value ?? null, o ?? undefined)}
    />
  )
}
