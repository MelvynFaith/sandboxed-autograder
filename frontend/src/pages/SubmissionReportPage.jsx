import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { getSubmissionReport } from '../api/submissionReport.js'
import { formatScore, verdictStyles } from '../components/gradingLabels.js'

// The report of one submission for lecturers and admins, opened from the recap (FR-11).
function SubmissionReportPage() {
  const { submissionId } = useParams()
  const navigate = useNavigate()
  const [report, setReport] = useState(null)
  const [isLoading, setIsLoading] = useState(true)
  const [loadError, setLoadError] = useState('')

  useEffect(() => {
    let isCurrent = true

    async function loadReport() {
      setIsLoading(true)
      setLoadError('')
      setReport(null)
      try {
        const result = await getSubmissionReport(submissionId)
        if (isCurrent) {
          setReport(result)
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

    loadReport()
    return () => {
      isCurrent = false
    }
  }, [submissionId])

  return (
    <main className="mx-auto max-w-4xl px-6 py-10">
      <div className="flex flex-wrap gap-3">
        <button
          className="rounded border border-slate-300 bg-white px-3 py-2 font-medium hover:bg-slate-100"
          onClick={() => navigate(-1)}
          type="button"
        >
          Kembali
        </button>
        <Link
          className="rounded border border-slate-300 bg-white px-3 py-2 font-medium hover:bg-slate-100"
          to={`/submissions/${encodeURIComponent(submissionId)}/code`}
        >
          Lihat kode
        </Link>
      </div>
      <h1 className="mt-4 text-2xl font-semibold">Laporan Submission</h1>
      {isLoading && <p className="mt-4 text-slate-600">Memuat laporan...</p>}
      {loadError && (
        <p className="mt-4 text-red-700" role="alert">
          Gagal memuat laporan: {loadError}
        </p>
      )}
      {report && (
        <section aria-label={`Laporan submission ${report.submissionId}`} className="mt-4">
          <p className="text-sm text-slate-700">
            Skor total: {formatScore(report.totalScore, report.maxScore)}
          </p>
          {report.results.length === 0 ? (
            <p className="mt-3 text-sm text-slate-600">Belum ada hasil test case.</p>
          ) : (
            <ul className="mt-3 space-y-3">
              {report.results.map((result, index) => (
                <li
                  className="rounded-md border border-slate-200 bg-white p-4"
                  key={result.testCaseId ?? `result-${index}`}
                >
                  <div className="flex flex-wrap items-center justify-between gap-3">
                    <h2 className="font-medium">
                      Test case {result.externalId ?? result.testCaseId ?? index + 1}
                      {result.hidden ? ' (tersembunyi)' : ''}
                    </h2>
                    <span
                      className={`inline-flex rounded-full px-3 py-1 text-sm font-semibold ${verdictStyles[result.verdict]}`}
                    >
                      {result.verdict}
                    </span>
                  </div>
                  <p className="mt-2 text-sm text-slate-700">
                    Skor {result.score}, {result.executionTimeMs} ms, {result.memoryKb} KB
                  </p>
                  {result.errorMessage && (
                    <p className="mt-2 text-sm text-red-800">
                      <span className="font-medium">Pesan error: </span>
                      {result.errorMessage}
                    </p>
                  )}
                  {result.actualOutput != null && (
                    <pre className="mt-2 overflow-x-auto rounded bg-slate-50 p-3 text-sm">
                      {result.actualOutput}
                    </pre>
                  )}
                </li>
              ))}
            </ul>
          )}
        </section>
      )}
    </main>
  )
}

export default SubmissionReportPage
