import { useEffect, useRef } from 'react'

const focusableSelector = [
  'a[href]',
  'button:not([disabled])',
  'input:not([disabled])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(', ')

function FocusTrapDialog({ labelledBy, onDismiss, returnFocusRef, children }) {
  const dialogRef = useRef(null)
  const onDismissRef = useRef(onDismiss)

  useEffect(() => {
    onDismissRef.current = onDismiss
  }, [onDismiss])

  useEffect(() => {
    const previouslyFocused = returnFocusRef?.current ?? document.activeElement
    const dialog = dialogRef.current
    const initialFocus =
      dialog?.querySelector('[data-dialog-initial-focus]') ??
      dialog?.querySelector(focusableSelector) ??
      dialog

    initialFocus?.focus()

    function handleKeyDown(event) {
      if (event.key === 'Escape') {
        if (onDismissRef.current) {
          event.preventDefault()
          onDismissRef.current()
        }
        return
      }

      if (event.key !== 'Tab' || !dialog) {
        return
      }

      const focusable = [...dialog.querySelectorAll(focusableSelector)]
      if (focusable.length === 0) {
        event.preventDefault()
        dialog.focus()
        return
      }

      const first = focusable[0]
      const last = focusable[focusable.length - 1]
      if (!dialog.contains(document.activeElement)) {
        event.preventDefault()
        first.focus()
      } else if (event.shiftKey && document.activeElement === first) {
        event.preventDefault()
        last.focus()
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault()
        first.focus()
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => {
      window.removeEventListener('keydown', handleKeyDown)
      if (previouslyFocused instanceof HTMLElement && previouslyFocused.isConnected) {
        previouslyFocused.focus()
      }
    }
  }, [returnFocusRef])

  return (
    <div
      aria-labelledby={labelledBy}
      aria-modal="true"
      className="fixed inset-0 z-20 grid place-items-center bg-slate-950/40 p-6"
      ref={dialogRef}
      role="dialog"
      tabIndex={-1}
    >
      <div className="w-full max-w-md space-y-4 rounded-lg bg-white p-6">
        {children}
      </div>
    </div>
  )
}

export default FocusTrapDialog
