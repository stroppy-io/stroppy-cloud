import { createContext, useContext } from 'react'

// Filter popovers and row-action menus report their open state up to the table so the page can
// pause auto-refresh while the user is inside one (a refetch must never close an open overlay).
export interface OverlayRegistry {
  setOpen: (id: string, open: boolean) => void
}

export const OverlayContext = createContext<OverlayRegistry | null>(null)

export function useOverlayRegistry(): OverlayRegistry | null {
  return useContext(OverlayContext)
}
