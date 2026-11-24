import { Fragment, useCallback, useEffect, useState } from 'react'
import { Link, useOutletContext } from 'react-router-dom'
import { cancelSubmission, getSubmissionHistory } from '../../api/submissionHistory.js'
import { getSubmissionReport } from '../../api/submissionReport.js'
import { subscribeToSubmissionStatus } from '../../api/submissionStatus.js'
import StatusBadge from '../../components/StatusBadge.jsx'
import SubmissionReport from '../../components/SubmissionReport.jsx'
import {
  formatScore,
  submissionStatusLabels,
  submissionStatusStyles,
} from '../../components/submissionLabels.js'

const PAGE_SIZE = 20
const ACTIVE = new Set(['QUEUED', 'RUNNING'])
const shortDate = new Intl.DateTimeFormat('id-ID', {
  dateStyle: 'medium',
  timeStyle: 'short',
  timeZone: 'Asia/Jakarta',
})

// The student's own submissions for this assignment, newest first (FR-09), with the report of each
// opened in place (FR-10). FR-14 allows one active submission per assignment, so at most one row is
// followed live (FR-08); the list reloads when it finishes.
function MySubmissionsTab() {
  const { assignment } = useOutletContext()
  const [page, setPage] = useState(1)
  const [reloadCount, setReloadCount] = useState(0)
  const [result, setResult] = useState(null)
  const [loadError, setLoadError] = useState('')
  const [actionMessage, setActionMessage] = useState(null)
  const [openReport, setOpenReport] = useState(null)
  const [reports, setReports] = useState({})
  const [live, setLive] = useState(null)
  const reload = useCallback(() => setReloadCount((count) => count + 1), [])

  useEffect(() => {
    let isCurrent = true
    getSubmissionHistory({ page, pageSize: PAGE_SIZE, assignmentId: assignment.id })
      .then((next) => {
        if (isCurrent) {
          setResult(next)
          setLoadError('')
        }
      })
      .catch((error) => {
        if (isCurrent) setLoadError(error.message)
      })
    return () => {
      isCurrent = false
    }
  }, [assignment.id, page, reloadCount])

  const activeId = result?.items.find((submission) => ACTIVE.has(submission.status))?.id

  useEffect(() => {
    if (!activeId) {
      return undefined
    }
    return subscribeToSubmissionStatus(activeId, {
      onStatus: (status) => {
        setLive({ id: activeId, ...status })
        if (!ACTIVE.has(status.status)) {
          reload()
        }
      },
      onFallback: () => {},
      onError: () => {},
    })
  }, [activeId, reload])

  async function toggleReport(submissionId) {
    if (openReport === submissionId) {
      setOpenReport(null)
      return
    }
    setOpenReport(submissionId)
    if (reports[submissionId]?.report) {
      return
    }
    setReports((current) => ({ ...current, [submissionId]: { loading: true } }))
    try {
      const report = await getSubmissionReport(submissionId)
      setReports((current) => ({ ...current, [submissionId]: { report } }))
    } catch (error) {
      setReports((current) => ({ ...current, [submissionId]: { error: error.message } }))
    }
  }

  async function handleCancel(submissionId) {
    setActionMessage(null)
    try {
      await cancelSubmission(submissionId)
      setActionMessage({ type: 'success', text: 'Submission dibatalkan.' })
      reload()
    } catch (error) {
      setActionMessage({ type: 'failure', text: error.message })
    }
  }

  // The live status of the followed submission is newer than the list until the list reloads.
  function currentStatus(submission) {
    return live?.id === submission.id ? live.status : submission.status
  }

  function statusCell(submission) {
    const status = currentStatus(submission)
    return (
      <div className="flex flex-wrap items-center gap-2">
        <StatusBadge
          className={submissionStatusStyles[status]}
          label={submissionStatusLabels[status]}
          status={status}
        />
        {status === 'RUNNING' && live?.id === submission.id && live.total_tests > 0 && (
          <span className="text-xs text-slate-600">
            Test {live.current_test} dari {live.total_tests}
          </span>
        )}
        {submission.isLate && (
          <span className="rounded bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-900">
            Terlambat
          </span>
        )}
      </div>
    )
  }

  return (
    <section aria-labelledby="my-submissions-heading" className="mt-6">
      <h2 className="sr-only" id="my-submissions-heading">
        Submission saya
      </h2>
      {loadError && (
        <p className="text-red-700" role="alert">
          Gagal memuat submission: {loadError}
        </p>
      )}
      {actionMessage && (
        <p
          className={actionMessage.type === 'success' ? 'mb-3 text-blue-800' : 'mb-3 text-red-700'}
          role={actionMessage.type === 'success' ? 'status' : 'alert'}
        >
          {actionMessage.text}
        </p>
      )}
      {!loadError && !result && <p className="text-slate-600">Memuat submission...</p>}
      {result?.items.length === 0 && page === 1 && (
        <p className="text-slate-600">
          Belum ada submission untuk soal ini.{' '}
          <Link className="text-blue-700 hover:underline" to="../submit">
            Kirim kode
          </Link>
        </p>
      )}
      {result?.items.length > 0 && (
        // A narrow screen scrolls the table, never the page (NFR-11).
        <div className="overflow-x-auto">
          <table className="w-full border-collapse text-left text-sm">
            <caption className="sr-only">Submission saya untuk {assignment.title}</caption>
            <thead>
              <tr className="border-b border-slate-300 text-slate-600">
                <th className="py-2 pr-3 font-medium" scope="col">
                  Waktu (WIB)
                </th>
                <th className="py-2 pr-3 font-medium" scope="col">
                  Status
                </th>
                <th className="py-2 pr-3 font-medium" scope="col">
                  Skor
                </th>
                <th className="py-2 font-medium" scope="col">
                  <span className="sr-only">Aksi</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {result.items.map((submission) => {
                const report = reports[submission.id]
                const isOpen = openReport === submission.id
                return (
                  <Fragment key={submission.id}>
                    <tr className="border-b border-slate-200 align-middle">
                      <td className="py-3 pr-3">
                        <time dateTime={submission.submittedAt}>
                          {shortDate.format(new Date(submission.submittedAt))}
                        </time>
                      </td>
                      <td className="py-3 pr-3">{statusCell(submission)}</td>
                      <td className="py-3 pr-3 font-medium">
                        {formatScore(submission.totalScore, submission.maxScore)}
                      </td>
                      <td className="py-3">
                        <div className="flex flex-wrap items-center justify-end gap-x-3 gap-y-1 leading-6">
                          {submission.status === 'COMPLETED' && (
                            <button
                              aria-controls={`report-${submission.id}`}
                              aria-expanded={isOpen}
                              className="font-medium text-blue-700 hover:underline"
                              onClick={() => toggleReport(submission.id)}
                              type="button"
                            >
                              {isOpen ? 'Tutup laporan' : 'Laporan'}
                            </button>
                          )}
                          <Link
                            className="inline-flex min-h-11 items-center font-medium text-blue-700 hover:underline"
                            to={`/submissions/${encodeURIComponent(submission.id)}/code`}
                          >
                            Kode
                          </Link>
                          {currentStatus(submission) === 'QUEUED' && (
                            <button
                              className="font-medium text-red-700 hover:underline"
                              onClick={() => handleCancel(submission.id)}
                              type="button"
                            >
                              Batalkan
                            </button>
                          )}
                        </div>
                      </td>
                    </tr>
                    {isOpen && (
                      <tr className="border-b border-slate-200">
                        <td colSpan={4} id={`report-${submission.id}`}>
                          {report?.loading && <p className="py-3 text-slate-600">Memuat laporan...</p>}
                          {report?.error && (
                            <p className="py-3 text-red-700" role="alert">
                              Gagal memuat laporan: {report.error}
                            </p>
                          )}
                          {report?.report && <SubmissionReport report={report.report} />}
                        </td>
                      </tr>
                    )}
                  </Fragment>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
      {result && (page > 1 || result.hasNextPage) && (
        <nav
          aria-label="Halaman submission"
          className="mt-4 flex items-center justify-between gap-3"
        >
          <button
            className="rounded border border-slate-300 px-3 py-2 text-sm disabled:opacity-50"
            disabled={page <= 1}
            onClick={() => setPage((current) => current - 1)}
            type="button"
          >
            Sebelumnya
          </button>
          <span className="text-sm text-slate-600">Halaman {page}</span>
          <button
            className="rounded border border-slate-300 px-3 py-2 text-sm disabled:opacity-50"
            disabled={!result.hasNextPage}
            onClick={() => setPage((current) => current + 1)}
            type="button"
          >
            Berikutnya
          </button>
        </nav>
      )}
    </section>
  )
}

export default MySubmissionsTab
