// Monaco must come from our bundle, not the jsdelivr CDN that @monaco-editor/react defaults to,
// and its language workers must be real Vite workers. Imported once by CodeEditorLazy.
import { loader } from '@monaco-editor/react'
import * as monaco from 'monaco-editor'
import editorWorker from 'monaco-editor/esm/vs/editor/editor.worker?worker'
import jsonWorker from 'monaco-editor/esm/vs/language/json/json.worker?worker'

let configured = false
export function setupMonaco(): void {
  if (configured) return
  configured = true
  window.MonacoEnvironment = {
    getWorker: (_id: string, label: string) =>
      label === 'json' ? new jsonWorker() : new editorWorker(),
  }
  loader.config({ monaco })
}
