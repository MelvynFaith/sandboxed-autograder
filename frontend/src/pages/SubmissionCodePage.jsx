import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { getSubmissionCode } from '../api/submissionCode.js'

function SubmissionCodePage() {
  const { submissionId } = useParams()
  const navigate = useNavigate()
  const [submission, setSubmission] = useState(null)
  const [isLoading, setIsLoading] = useState(true)
  const [loadError, setLoadError] = useState('')

  useEffect(() => {
    let isCurrent = true

    async function loadCode() {
      setIsLoading(true)
      setLoadError('')
      setSubmission(null)
      try {
        const result = await getSubmissionCode(submissionId)
        if (isCurrent) {
          setSubmission(result)
        }
      } catch (error) {
        if (isCurrent) {
          setLoadError(error.message)
        }
      } finally {
        if (isCurrent) {
          setIsLoading(false)
        }
      }
    }

    loadCode()
    return () => {
      isCurrent = false
    }
  }, [submissionId])

  // A trailing newline ends the last line; it does not start another one.
  const lines = submission ? submission.code.replace(/\n$/, '').split('\n') : []

  return (
    <main className="mx-auto max-w-4xl px-6 py-10">
      <button
        className="rounded border border-slate-300 bg-white px-3 py-2 font-medium hover:bg-slate-100"
        onClick={() => navigate(-1)}
        type="button"
      >
        Kembali
      </button>
      <h1 className="mt-4 text-2xl font-semibold">Kode Submission</h1>
      {isLoading && <p className="mt-4 text-slate-600">Memuat kode submission...</p>}
      {loadError && (
        <p className="mt-4 text-red-700" role="alert">
          {loadError}
        </p>
      )}
      {submission && (
        <section aria-label={`Kode submission ${submission.submissionId}`} className="mt-4">
          <p className="text-sm text-slate-700">
            Berkas: <span className="font-mono">{submission.originalFilename}</span>
          </p>
          {/* A scrollable region needs to be focusable so keyboard users can scroll it. */}
          <div
            aria-label={`Isi berkas ${submission.originalFilename}, ${lines.length} baris`}
            className="mt-3 flex overflow-x-auto rounded-md border border-slate-300 bg-white font-mono text-sm"
            role="region"
            tabIndex={0}
          >
            {/* Line numbers are decoration and stay out of the selection and the a11y tree. */}
            <div
              aria-hidden="true"
              className="select-none border-r border-slate-200 bg-slate-100 px-3 py-3 text-right text-slate-600"
              data-testid="line-numbers"
            >
              {lines.map((_, index) => (
                <div key={index}>{index + 1}</div>
              ))}
            </div>
            <pre className="flex-1 px-3 py-3">
              <code>{lines.join('\n')}</code>
            </pre>
          </div>
        </section>
      )}
    </main>
  )
}

export default SubmissionCodePage
