import { useEffect, useId, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { cancelSubmission, getSubmissionHistory } from '../api/submissionHistory.js'
import { getSubmissionReport } from '../api/submissionReport.js'
import StatusBadge from '../components/StatusBadge.jsx'
import SubmissionReport from '../components/SubmissionReport.jsx'
import FocusTrapDialog from '../components/FocusTrapDialog.jsx'
import {
  dateFormatter,
  formatScore,
  submissionStatusLabels,
  submissionStatusStyles,
} from '../components/submissionLabels.js'

function CancelConfirmation({ assignmentTitle, isCancelling, onCancel, onDismiss, returnFocusRef }) {
  const titleId = useId()

  return (
    <FocusTrapDialog
      labelledBy={titleId}
      onDismiss={isCancelling ? undefined : onDismiss}
      returnFocusRef={returnFocusRef}
    >
      <h2 className="text-lg font-semibold" id={titleId}>
        Konfirmasi pembatalan submission {assignmentTitle}
      </h2>
      <p className="text-sm text-amber-950">
        Batalkan submission ini? Submission yang dibatalkan tidak akan dinilai.
      </p>
      <div className="flex flex-wrap gap-3">
        <button
          className="rounded bg-red-700 px-4 py-2 font-medium text-white hover:bg-red-800 disabled:opacity-60"
          disabled={isCancelling}
          onClick={onCancel}
          type="button"
        >
          {isCancelling ? 'Membatalkan...' : 'Ya, batalkan'}
        </button>
        <button
          className="rounded border border-slate-300 bg-white px-4 py-2 font-medium disabled:opacity-60"
          disabled={isCancelling}
          onClick={onDismiss}
          data-dialog-initial-focus
          type="button"
        >
          Tidak
        </button>
      </div>
    </FocusTrapDialog>
  )
}

function HistoryPage() {
  const [submissions, setSubmissions] = useState([])
  const [page, setPage] = useState(1)
  const [reloadKey, setReloadKey] = useState(0)
  const [hasNextPage, setHasNextPage] = useState(false)
  const [isLoading, setIsLoading] = useState(true)
  const [loadError, setLoadError] = useState('')
  const [selectedId, setSelectedId] = useState(null)
  const [report, setReport] = useState(null)
  const [isReportLoading, setIsReportLoading] = useState(false)
  const [reportError, setReportError] = useState('')
  const [confirmingId, setConfirmingId] = useState(null)
  const [cancellingId, setCancellingId] = useState(null)
  const [cancelNotice, setCancelNotice] = useState(null)
  const reportRequestId = useRef(0)
  const cancelTrigger = useRef(null)

  useEffect(() => {
    let isCurrent = true

    async function loadHistory() {
      setIsLoading(true)
      setLoadError('')
      try {
        const result = await getSubmissionHistory({ page, pageSize: 50 })
        if (isCurrent) {
          setSubmissions(result.items)
          setHasNextPage(result.hasNextPage)
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
  }, [page, reloadKey])

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

  async function confirmCancel(submissionId) {
    setCancellingId(submissionId)
    setCancelNotice(null)
    try {
      const cancelled = await cancelSubmission(submissionId)
      setSubmissions((current) =>
        current.map((entry) => (entry.id === submissionId ? { ...entry, status: cancelled.status } : entry)),
      )
      setCancelNotice({ id: submissionId, type: 'success', text: 'Submission berhasil dibatalkan.' })
    } catch (error) {
      setCancelNotice({ id: submissionId, type: 'failure', text: error.message })
      // The submission may have started or disappeared in the meantime; show the real state.
      setReloadKey((current) => current + 1)
    } finally {
      setCancellingId(null)
      setConfirmingId(null)
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
      {!isLoading && !loadError && (
        <nav aria-label="Paginasi riwayat submission" className="mt-5 flex items-center gap-3">
          <button
            className="rounded border border-slate-300 px-3 py-2 disabled:opacity-50"
            disabled={page === 1}
            onClick={() => setPage((current) => Math.max(1, current - 1))}
            type="button"
          >
            Sebelumnya
          </button>
          <span aria-current="page">Halaman {page}</span>
          <button
            className="rounded border border-slate-300 px-3 py-2 disabled:opacity-50"
            disabled={!hasNextPage}
            onClick={() => setPage((current) => current + 1)}
            type="button"
          >
            Berikutnya
          </button>
        </nav>
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
                  Skor: {formatScore(submission.totalScore, submission.maxScore)}
                </p>
                {submission.isLate && (
                  <p className="mt-1 text-sm font-medium text-amber-800">Terlambat</p>
                )}
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
            <div className="mt-4 flex flex-wrap gap-3">
              <button
                aria-expanded={selectedId === submission.id}
                className="rounded bg-blue-700 px-4 py-2 font-medium text-white hover:bg-blue-800"
                onClick={() => toggleReport(submission.id)}
                type="button"
              >
                {selectedId === submission.id ? 'Tutup laporan' : 'Lihat laporan'}
              </button>
              {/* FR-20: the student may read their own submitted code. */}
              <Link
                aria-label={`Lihat kode ${submission.assignmentTitle}`}
                className="rounded border border-slate-300 px-4 py-2 font-medium hover:bg-slate-100"
                to={`/submissions/${encodeURIComponent(submission.id)}/code`}
              >
                Lihat kode
              </Link>
              {submission.status === 'QUEUED' && (
                <button
                  aria-label={`Batalkan submission ${submission.assignmentTitle}`}
                  aria-hidden={confirmingId === submission.id}
                  className="rounded border border-red-300 px-4 py-2 font-medium text-red-800 hover:bg-red-50"
                  onClick={(event) => {
                    setCancelNotice(null)
                    cancelTrigger.current = event.currentTarget
                    event.currentTarget.focus()
                    setConfirmingId(submission.id)
                  }}
                  type="button"
                >
                  Batalkan
                </button>
              )}
            </div>
            {confirmingId === submission.id && (
              <CancelConfirmation
                assignmentTitle={submission.assignmentTitle}
                isCancelling={cancellingId === submission.id}
                onCancel={() => confirmCancel(submission.id)}
                onDismiss={() => setConfirmingId(null)}
                returnFocusRef={cancelTrigger}
              />
            )}
            {cancelNotice?.id === submission.id && (
              <p
                className={
                  cancelNotice.type === 'success'
                    ? 'mt-3 text-sm text-blue-800'
                    : 'mt-3 text-sm text-red-700'
                }
                role={cancelNotice.type === 'success' ? 'status' : 'alert'}
              >
                {cancelNotice.text}
              </p>
            )}
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
