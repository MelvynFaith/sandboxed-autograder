import '@testing-library/jest-dom/vitest'
import { EditorView } from '@codemirror/view'
import { cleanup, render, screen, within } from '@testing-library/react'
import { useState } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import CodeEditor from './CodeEditor.jsx'

afterEach(cleanup)

// The editor lives in a shadow root (see CodeEditor.jsx), so queries go through it.
function shadow() {
  return screen.getByTestId('code-editor').shadowRoot
}

function editorView() {
  return EditorView.findFromDOM(shadow().querySelector('.cm-editor'))
}

describe('CodeEditor', () => {
  it('is a labelled, described multi-line text box', () => {
    render(<CodeEditor label="Kode Python" onChange={() => {}} value="" />)

    const box = within(shadow()).getByRole('textbox', { name: 'Kode Python' })
    expect(box).toHaveAttribute('aria-multiline', 'true')
    expect(box).toHaveAccessibleDescription(
      'Tab menambah indentasi. Untuk keluar dari editor dengan keyboard, tekan Esc lalu Tab.',
    )
  })

  it('reports every edit as the whole text', () => {
    const onChange = vi.fn()
    render(<CodeEditor label="Kode Python" onChange={onChange} value="" />)

    editorView().dispatch({ changes: { from: 0, insert: 'print(1)\n' } })

    expect(onChange).toHaveBeenLastCalledWith('print(1)\n')
  })

  it('shows a value set from outside', () => {
    const onChange = vi.fn()
    const { rerender } = render(<CodeEditor label="Kode Python" onChange={onChange} value="" />)

    rerender(<CodeEditor label="Kode Python" onChange={onChange} value={'x = 1\n'} />)

    expect(editorView().state.doc.toString()).toBe('x = 1\n')
    expect(onChange).toHaveBeenCalledWith('x = 1\n')
  })

  it('shows its label inside the shadow tree and follows label changes', () => {
    const { rerender } = render(<CodeEditor label="Kode Python" onChange={() => {}} value="" />)
    expect(shadow().querySelector('#editor-label')).toHaveTextContent('Kode Python')

    rerender(<CodeEditor label="Kode jawaban" onChange={() => {}} value="" />)

    expect(within(shadow()).getByRole('textbox', { name: 'Kode jawaban' })).toBeInTheDocument()
  })

  it('keeps its text across parent re-renders', () => {
    function Parent() {
      const [code, setCode] = useState('a = 1')
      return <CodeEditor label="Kode Python" onChange={setCode} value={code} />
    }
    render(<Parent />)

    editorView().dispatch({ changes: { from: 5, insert: '\nb = 2' } })

    expect(editorView().state.doc.toString()).toBe('a = 1\nb = 2')
  })
})
