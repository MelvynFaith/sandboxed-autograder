import { useEffect, useRef, useState } from 'react'
import { getSubmissionHistory } from '../api/submissionHistory.js'
import { getSubmissionReport } from '../api/submissionReport.js'

const dateFormatter = new Intl.DateTimeFormat('id-ID', {
  dateStyle: 'long',
  timeStyle: 'short',
  timeZone: 'Asia/Jakarta',
})

const submissionStatusLabels = {
  queued: 'Antre',
  running: 'Sedang berjalan',
  completed: 'Selesai',
  error: 'Error',
  timeout: 'Waktu habis',
}

const submissionStatusStyles = {
  queued: 'bg-amber-100 text-amber-900',
  running: 'bg-blue-100 text-blue-900',
  completed: 'bg-green-100 text-green-900',
  error: 'bg-red-100 text-red-900',
  timeout: 'bg-red-100 text-red-900',
}

const verdictStyles = {
  PASSED: 'bg-green-100 text-green-900',
  FAILED: 'bg-red-100 text-red-900',
  TIMEOUT: 'bg-amber-100 text-amber-900',
  MEMORY_LIMIT_EXCEEDED: 'bg-purple-100 text-purple-900',
  RUNTIME_ERROR: 'bg-orange-100 text-orange-900',
  SYNTAX_ERROR: 'bg-pink-100 text-pink-900',
  SECURITY_VIOLATION: 'bg-slate-900 text-white',
}

function StatusBadge({ status, label, className }) {
  return (
    <span
      className={`inline-flex rounded-full px-3 py-1 text-sm font-semibold ${className}`}
      data-testid={verdictStyles[status] ? 'verdict-badge' : undefined}
    >
      {label ?? status}
    </span>
  )
}

function SubmissionReport({ report }) {
  return (
    <section aria-label={`Laporan submission ${report.submissionId}`} className="mt-5 space-y-4">
      <h3 className="text-lg font-semibold">Laporan per test case</h3>
      <p className="text-sm text-slate-700">Skor total: {report.totalScore}</p>
      {report.results.length === 0 ? (
        <p className="text-sm text-slate-600">Belum ada hasil test case.</p>
      ) : (
        <ul className="space-y-3">
          {report.results.map((result) => (
            <li
              className="rounded-md border border-slate-200 bg-slate-50 p-4"
              key={result.testCaseId}
            >
              <div className="flex flex-wrap items-center justify-between gap-3">
                <h4 className="font-medium">Test case {result.testCaseId}</h4>
                <StatusBadge
                  className={verdictStyles[result.verdict]}
                  status={result.verdict}
                />
              </div>
              <dl className="mt-3 grid gap-3 text-sm sm:grid-cols-3">
                <div>
                  <dt className="text-slate-600">Skor</dt>
                  <dd className="font-medium">{result.score}</dd>
                </div>
                <div>
                  <dt className="text-slate-600">Waktu eksekusi</dt>
                  <dd className="font-medium">{result.executionTimeMs} ms</dd>
                </div>
                <div>
                  <dt className="text-slate-600">Penggunaan memori</dt>
                  <dd className="font-medium">{result.memoryKb} KB</dd>
                </div>
              </dl>
              {!result.hidden && (
                <div className="mt-3 space-y-2 text-sm">
                  {result.errorMessage && (
                    <p className="text-red-800">
                      <span className="font-medium">Pesan error: </span>
                      {result.errorMessage}
                    </p>
                  )}
                  {result.actualOutput !== null && (
                    <div>
                      <h5 className="font-medium">Output aktual</h5>
                      <pre className="mt-1 overflow-x-auto rounded bg-white p-3 text-slate-800">
                        {result.actualOutput}
                      </pre>
                    </div>
                  )}
                </div>
              )}
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}

function HistoryPage() {
  const [submissions, setSubmissions] = useState([])
  const [isLoading, setIsLoading] = useState(true)
  const [loadError, setLoadError] = useState('')
  const [selectedId, setSelectedId] = useState(null)
  const [report, setReport] = useState(null)
  const [isReportLoading, setIsReportLoading] = useState(false)
  const [reportError, setReportError] = useState('')
  const reportRequestId = useRef(0)

  useEffect(() => {
    let isCurrent = true

    async function loadHistory() {
      try {
        const results = await getSubmissionHistory()
        if (isCurrent) {
          setSubmissions(results)
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

    loadHistory()
    return () => {
      isCurrent = false
    }
  }, [])

  async function toggleReport(submissionId) {
    if (selectedId === submissionId) {
      reportRequestId.current += 1
      setSelectedId(null)
      setReport(null)
      setReportError('')
      setIsReportLoading(false)
      return
    }

    const currentRequestId = reportRequestId.current + 1
    reportRequestId.current = currentRequestId
    setSelectedId(submissionId)
    setReport(null)
    setReportError('')
    setIsReportLoading(true)

    try {
      const result = await getSubmissionReport(submissionId)
      if (currentRequestId === reportRequestId.current) {
        setReport(result)
      }
    } catch (error) {
      if (currentRequestId === reportRequestId.current) {
        setReportError(error.message)
      }
    } finally {
      if (currentRequestId === reportRequestId.current) {
        setIsReportLoading(false)
      }
    }
  }

  return (
    <main className="mx-auto max-w-4xl px-6 py-10">
      <h1 className="text-2xl font-semibold">Riwayat Submission</h1>
      {isLoading && <p className="mt-4 text-slate-600">Memuat riwayat submission...</p>}
      {loadError && (
        <p className="mt-4 text-red-700" role="alert">
          Gagal memuat riwayat submission: {loadError}
        </p>
      )}
      {!isLoading && !loadError && submissions.length === 0 && (
        <p className="mt-4 text-slate-600">Belum ada submission.</p>
      )}
      <ul className="mt-6 space-y-4">
        {submissions.map((submission) => (
          <li
            className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm"
            key={submission.id}
          >
            <div className="flex flex-wrap items-start justify-between gap-4">
              <div>
                <h2 className="text-lg font-semibold">{submission.assignmentTitle}</h2>
                <p className="mt-2 text-sm text-slate-700">
                  Skor: {submission.totalScore}
                </p>
                <p className="mt-1 text-sm text-slate-600">
                  Dikumpulkan:{' '}
                  <time dateTime={submission.submittedAt}>
                    {dateFormatter.format(new Date(submission.submittedAt))}
                  </time>
                </p>
              </div>
              <StatusBadge
                className={
                  submissionStatusStyles[submission.status] ?? 'bg-slate-100 text-slate-800'
                }
                label={submissionStatusLabels[submission.status] ?? submission.status}
                status={submission.status}
              />
            </div>
            <button
              aria-expanded={selectedId === submission.id}
              className="mt-4 rounded bg-blue-700 px-4 py-2 font-medium text-white hover:bg-blue-800"
              onClick={() => toggleReport(submission.id)}
              type="button"
            >
              {selectedId === submission.id ? 'Tutup laporan' : 'Lihat laporan'}
            </button>
            {selectedId === submission.id && (
              <div>
                {isReportLoading && (
                  <p className="mt-4 text-slate-600">Memuat laporan...</p>
                )}
                {reportError && (
                  <p className="mt-4 text-red-700" role="alert">
                    Gagal memuat laporan: {reportError}
                  </p>
                )}
                {report && <SubmissionReport report={report} />}
              </div>
            )}
          </li>
        ))}
      </ul>
    </main>
  )
}

export default HistoryPage
