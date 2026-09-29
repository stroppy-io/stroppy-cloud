import { fieldErrorsFrom } from '@api/errors'
import { catalogMutations } from '@api/queries/catalog'
import { type Schema, type Values, validateLocal } from '@components/schema/engine'
import { useDebouncedCallback } from '@tanstack/react-pacer'
import { useEffect, useState } from 'react'

// Local constraints instantly + server `:validate` debounced (400 ms). Returns merged field errors.
export function useSchemaValidate(
  schemaId: string | undefined,
  schema: Schema | undefined,
  value: Values | undefined,
  enabled = true
) {
  const [serverErrors, setServerErrors] = useState<Record<string, string>>({})
  const [checking, setChecking] = useState(false)
  const run = useDebouncedCallback(
    async (v: Values) => {
      if (!schemaId) return
      setChecking(true)
      try {
        const res = await catalogMutations.validate(schemaId, v)
        setServerErrors(fieldErrorsFrom(res.result))
      } catch {
        // validation endpoint failure is not a field error
      } finally {
        setChecking(false)
      }
    },
    { wait: 400 }
  )
  const local = schema && value ? validateLocal(schema.fields, value) : {}
  const localOk = Object.keys(local).length === 0
  const serialized = JSON.stringify(value)
  // biome-ignore lint/correctness/useExhaustiveDependencies: re-run on serialized value only
  useEffect(() => {
    if (enabled && value && localOk) run(value)
  }, [serialized, enabled, localOk, run])
  return {
    errors: { ...serverErrors, ...local },
    checking,
    hasErrors: Object.keys({ ...serverErrors, ...local }).length > 0,
  }
}
