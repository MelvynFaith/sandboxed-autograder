import { useEffect, useRef } from 'react'

function FormFeedback({ error, success }) {
  const noticeRef = useRef(null)
  const message = error || success
  const isError = Boolean(error)

  useEffect(() => {
    if (!message) return

    const notice = noticeRef.current
    if (!notice) return

    notice.scrollIntoView?.({ block: 'nearest' })
    notice.focus()
  }, [isError, message])

  if (!message) return null

  return (
    <p
      className={`sticky top-0 z-10 mt-4 rounded border px-4 py-3 shadow-sm focus:outline focus:outline-2 focus:outline-offset-2 focus:outline-blue-700 ${
        isError
          ? 'border-red-200 bg-red-50 text-red-700'
          : 'border-blue-200 bg-blue-50 text-blue-800'
      }`}
      ref={noticeRef}
      role={isError ? 'alert' : 'status'}
      tabIndex={-1}
    >
      {message}
    </p>
  )
}

export default FormFeedback
