import { TagsInput } from '@grafana/ui'

// `key=value` tags editor over a Record<string,string>; plain tags are stored with an empty value.
export function TagsEditor({
  value,
  onChange,
  disabled,
  placeholder,
}: {
  value: Record<string, string> | undefined
  onChange: (v: Record<string, string>) => void
  disabled?: boolean
  placeholder?: string
}) {
  const tags = Object.entries(value ?? {}).map(([k, v]) => (v ? `${k}=${v}` : k))
  return (
    <TagsInput
      tags={tags}
      disabled={disabled}
      placeholder={placeholder}
      onChange={(next) => {
        const out: Record<string, string> = {}
        for (const t of next) {
          const [k, ...rest] = t.split('=')
          if (k.trim()) out[k.trim()] = rest.join('=').trim()
        }
        onChange(out)
      }}
    />
  )
}

export function TagsView({ value }: { value: Record<string, string> | undefined }) {
  const entries = Object.entries(value ?? {})
  if (!entries.length) return null
  return <span>{entries.map(([k, v]) => (v ? `${k}=${v}` : k)).join(', ')}</span>
}
