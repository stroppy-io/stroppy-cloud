import { atom } from "nanostores"

type Theme = "light" | "dark"

const stored = localStorage.getItem("stroppy-theme") as Theme | null

export const $theme = atom<Theme>(stored ?? "dark")

$theme.subscribe((theme) => {
  localStorage.setItem("stroppy-theme", theme)
  document.documentElement.classList.toggle("dark", theme === "dark")
})

// Apply on load
document.documentElement.classList.toggle("dark", $theme.get() === "dark")

export function toggleTheme() {
  $theme.set($theme.get() === "dark" ? "light" : "dark")
}
