import { toast } from '@app/Toaster'
import { useCallback } from 'react'
import { useTranslation } from 'react-i18next'

// Clipboard write with a toast, for «Копировать …» items of table row menus.
export function useCopy() {
  const { t } = useTranslation()
  return useCallback(
    (text: string) =>
      void navigator.clipboard
        .writeText(text)
        .then(() => toast.success(t('common.actions.copied')))
        .catch((e) => toast.error(e)),
    [t]
  )
}
