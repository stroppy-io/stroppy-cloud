import { Alert } from '@grafana/ui'
import { formError } from '@helpers/form'

// Renders `form.state.errorMap.onServer` (a string set by `applyServerErrors`) next to the submit button.
export function ServerErrorAlert({ error }: { error: unknown }) {
  const message = formError(error)
  if (!message) return null
  return <Alert severity="error" title={message} />
}
