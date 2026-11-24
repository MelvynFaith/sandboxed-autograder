import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands'
import {
  bracketMatching,
  defaultHighlightStyle,
  indentOnInput,
  indentUnit,
  syntaxHighlighting,
} from '@codemirror/language'
import { python } from '@codemirror/lang-python'
import { EditorState } from '@codemirror/state'
import {
  drawSelection,
  EditorView,
  highlightActiveLine,
  highlightActiveLineGutter,
  keymap,
  lineNumbers,
} from '@codemirror/view'
import { useEffect, useRef } from 'react'

const EDITOR_HINT =
  'Tab menambah indentasi. Untuk keluar dari editor dengan keyboard, tekan Esc lalu Tab.'

const theme = EditorView.theme({
  '&': { fontSize: '0.875rem', backgroundColor: '#ffffff' },
  '&.cm-focused': { outline: '2px solid #1d4ed8', outlineOffset: '2px' },
  '.cm-scroller': {
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
    minHeight: '16rem',
    maxHeight: '32rem',
  },
  '.cm-gutters': { backgroundColor: '#f8fafc', color: '#475569', borderRight: '1px solid #e2e8f0' },
  '.cm-activeLine, .cm-activeLineGutter': { backgroundColor: '#eff6ff' },
})

// Styles of the label, the frame and the hint, which live in the shadow tree with the editor.
const frameCss = `
  .label { display: block; margin-bottom: 0.25rem; font-weight: 500; }
  .frame { overflow: hidden; border: 1px solid #cbd5e1; border-radius: 0.25rem; }
  .hint { margin: 0.25rem 0 0; font-size: 0.875rem; line-height: 1.25rem; color: #475569; }
`

// A Python editor (CodeMirror 6).
//
// It lives in a shadow root. CodeMirror injects its styles at run time: into a document it adds
// <style> elements, which the Content-Security-Policy of the site blocks (default-src 'self', no
// 'unsafe-inline'); into a shadow root it uses constructed style sheets, which the policy allows.
// The policy stays as strict as it is. The label and the hint are in the same shadow tree, so the
// text box keeps its accessible name and description (an ID reference cannot cross the boundary).
//
// Tab indents, because Python needs it; Esc and then Tab moves focus out of the editor, so the
// keyboard is never trapped (NFR-11, WCAG 2.1.2).
function CodeEditor({ label, onChange, value }) {
  const host = useRef(null)
  const view = useRef(null)
  const labelNode = useRef(null)
  const latestOnChange = useRef(onChange)

  useEffect(() => {
    latestOnChange.current = onChange
  }, [onChange])

  useEffect(() => {
    const shadow = host.current.shadowRoot ?? host.current.attachShadow({ mode: 'open' })
    const caption = document.createElement('span')
    caption.className = 'label'
    caption.id = 'editor-label'
    const frame = document.createElement('div')
    frame.className = 'frame'
    const hint = document.createElement('p')
    hint.className = 'hint'
    hint.id = 'editor-hint'
    hint.textContent = EDITOR_HINT
    shadow.replaceChildren(caption, frame, hint)
    labelNode.current = caption
    if (shadow.adoptedStyleSheets && typeof CSSStyleSheet === 'function') {
      const sheet = new CSSStyleSheet()
      sheet.replaceSync(frameCss)
      shadow.adoptedStyleSheets = [sheet]
    }

    const editor = new EditorView({
      parent: frame,
      root: shadow,
      state: EditorState.create({
        doc: value,
        extensions: [
          lineNumbers(),
          highlightActiveLineGutter(),
          highlightActiveLine(),
          history(),
          drawSelection(),
          indentOnInput(),
          bracketMatching(),
          indentUnit.of('    '),
          EditorState.tabSize.of(4),
          python(),
          syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
          keymap.of([...defaultKeymap, ...historyKeymap, indentWithTab]),
          EditorView.contentAttributes.of({
            'aria-labelledby': 'editor-label',
            'aria-describedby': 'editor-hint',
            'aria-multiline': 'true',
            spellcheck: 'false',
            autocapitalize: 'off',
            autocorrect: 'off',
          }),
          EditorView.updateListener.of((update) => {
            if (update.docChanged) {
              latestOnChange.current(update.state.doc.toString())
            }
          }),
          theme,
        ],
      }),
    })
    view.current = editor
    return () => {
      editor.destroy()
      shadow.replaceChildren()
      view.current = null
      labelNode.current = null
    }
    // The editor is created once; value and label changes are applied below.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    if (labelNode.current) {
      labelNode.current.textContent = label
    }
  }, [label])

  useEffect(() => {
    const editor = view.current
    if (editor && editor.state.doc.toString() !== value) {
      editor.dispatch({ changes: { from: 0, to: editor.state.doc.length, insert: value } })
    }
  }, [value])

  return <div data-testid="code-editor" ref={host} />
}

export default CodeEditor
