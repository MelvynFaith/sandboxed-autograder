import { useEffect, useRef } from 'react'

function FormFeedback({ error, success, announcementKey }) {
  const noticeRef = useRef(null)
  const message = error || success
  const isError = Boolean(error)

  useEffect(() => {
    if (!message) return

    const notice = noticeRef.current
    if (!notice) return

    notice.scrollIntoView?.({ block: 'nearest' })
    notice.focus()
  }, [announcementKey, isError, message])

  if (!message) return null

  return (
    <p
      className={`sticky top-0 z-10 mt-4 rounded-md border px-4 py-3 focus:outline focus:outline-2 focus:outline-offset-2 focus:outline-blue-700 ${
        isError
          ? 'border-danger/40 bg-danger-tint text-danger'
          : 'border-accent/40 bg-accent-tint text-link'
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
