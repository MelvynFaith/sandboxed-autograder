import { useEffect, useRef, useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import {
  downloadRecapCsv,
  getAssignmentRecap,
  regradeAssignment,
  regradeSubmission,
} from '../api/gradeRecap.js'
import {
  dateFormatter,
  formatScore,
  statusLabels,
  statusStyles,
} from '../components/gradingLabels.js'
import FocusTrapDialog from '../components/FocusTrapDialog.jsx'

const regradableStatuses = new Set(['COMPLETED', 'FAILED'])

function RegradeAllConfirmation({ isBusy, onConfirm, onDismiss, returnFocusRef }) {
  const titleId = 'regrade-all-confirmation-title'

  return (
    <FocusTrapDialog
      labelledBy={titleId}
      onDismiss={isBusy ? undefined : onDismiss}
      returnFocusRef={returnFocusRef}
    >
      <h2 className="text-lg font-semibold" id={titleId}>
        Konfirmasi penilaian ulang seluruh assignment
      </h2>
      <p className="text-sm text-amber-950">
        Nilai ulang semua submission yang sudah selesai pada assignment ini? Hasil lama akan
        diganti setelah penilaian ulang selesai.
      </p>
      <div className="flex flex-wrap gap-3">
        <button
          className="rounded bg-blue-700 px-4 py-2 font-medium text-white hover:bg-blue-800 disabled:opacity-60"
          disabled={isBusy}
          onClick={onConfirm}
          type="button"
        >
          {isBusy ? 'Menjadwalkan...' : 'Ya, nilai ulang semua'}
        </button>
        <button
          className="rounded border border-slate-300 bg-white px-4 py-2 font-medium disabled:opacity-60"
          disabled={isBusy}
          onClick={onDismiss}
          data-dialog-initial-focus
          type="button"
        >
          Batal
        </button>
      </div>
    </FocusTrapDialog>
  )
}

function RecapPage() {
  const { assignmentId } = useParams()
  const [searchParams] = useSearchParams()
  const courseId = searchParams.get('courseId')
  const [recap, setRecap] = useState(null)
  const [page, setPage] = useState(1)
  const [reloadKey, setReloadKey] = useState(0)
  const [isLoading, setIsLoading] = useState(true)
  const [loadError, setLoadError] = useState('')
  const [notice, setNotice] = useState(null)
  const [isDownloading, setIsDownloading] = useState(false)
  const [isConfirmingAll, setIsConfirmingAll] = useState(false)
  const [busyRegradeId, setBusyRegradeId] = useState(null)
  const [isRegradingAll, setIsRegradingAll] = useState(false)
  const regradeAllTrigger = useRef(null)

  useEffect(() => {
    let isCurrent = true

    async function loadRecap() {
      setIsLoading(true)
      setLoadError('')
      try {
        const result = await getAssignmentRecap(assignmentId, { page, pageSize: 50 })
        if (isCurrent) {
          setRecap(result)
        }
      } catch (error) {
        if (isCurrent) {
          setRecap(null)
          setLoadError(error.message)
        }
      } finally {
        if (isCurrent) {
          setIsLoading(false)
        }
      }
    }

    loadRecap()
    return () => {
      isCurrent = false
    }
  }, [assignmentId, page, reloadKey])

  async function downloadCsv() {
    setIsDownloading(true)
    setNotice(null)
    try {
      const filename = await downloadRecapCsv(assignmentId)
      setNotice({ type: 'success', text: `Rekap nilai diunduh sebagai ${filename}.` })
    } catch (error) {
      setNotice({ type: 'failure', text: error.message })
    } finally {
      setIsDownloading(false)
    }
  }

  async function regradeOne(row) {
    const submissionId = row.latestSubmission.id
    setBusyRegradeId(submissionId)
    setNotice(null)
    try {
      await regradeSubmission(submissionId)
      setNotice({
        type: 'success',
        text: `Submission ${row.name} dijadwalkan untuk dinilai ulang.`,
      })
    } catch (error) {
      setNotice({ type: 'failure', text: error.message })
    } finally {
      setBusyRegradeId(null)
      // Show the real state either way: it may have changed since the page was loaded.
      setReloadKey((current) => current + 1)
    }
  }

  async function regradeAll() {
    setIsRegradingAll(true)
    setNotice(null)
    try {
      const result = await regradeAssignment(assignmentId)
      setNotice({
        type: 'success',
        text: `${result.queued} submission dijadwalkan untuk dinilai ulang, ${result.skipped} dilewati.`,
      })
    } catch (error) {
      setNotice({ type: 'failure', text: error.message })
    } finally {
      setIsRegradingAll(false)
      setIsConfirmingAll(false)
      setReloadKey((current) => current + 1)
    }
  }

  return (
    <main className="mx-auto max-w-6xl px-6 py-10">
      <h1 className="text-2xl font-semibold">Rekap Nilai</h1>
      <nav aria-label="Alur dosen" className="mt-4 rounded-lg border border-slate-200 bg-white p-4">
        <ol className="flex flex-wrap gap-x-6 gap-y-2 text-sm">
          <li>
            <Link
              className="font-medium text-blue-700 underline"
              to={`/courses${courseId ? `?courseId=${encodeURIComponent(courseId)}` : ''}`}
            >
              1. Course &amp; enrollment
            </Link>
          </li>
          <li>
            <Link
              className="font-medium text-blue-700 underline"
              to={`/lecturer/assignments${courseId ? `?courseId=${encodeURIComponent(courseId)}` : ''}`}
            >
              2. Assignment &amp; test case
            </Link>
          </li>
          <li aria-current="step" className="font-semibold text-blue-800">
            3. Rekap nilai &amp; CSV
          </li>
        </ol>
      </nav>
      {recap && <p className="mt-1 text-slate-700">{recap.assignmentTitle}</p>}
      {isLoading && <p className="mt-4 text-slate-600">Memuat rekap nilai...</p>}
      {loadError && (
        <p className="mt-4 text-red-700" role="alert">
          Gagal memuat rekap nilai: {loadError}
        </p>
      )}
      {recap && (
        <div className="mt-5 flex flex-wrap gap-3">
          <button
            className="rounded bg-blue-700 px-4 py-2 font-medium text-white hover:bg-blue-800 disabled:opacity-60"
            disabled={isDownloading}
            onClick={downloadCsv}
            type="button"
          >
            {isDownloading ? 'Mengunduh...' : 'Unduh CSV'}
          </button>
          <button
            aria-hidden={isConfirmingAll}
            className="rounded border border-slate-300 bg-white px-4 py-2 font-medium hover:bg-slate-100"
            onClick={(event) => {
              setNotice(null)
              regradeAllTrigger.current = event.currentTarget
              event.currentTarget.focus()
              setIsConfirmingAll(true)
            }}
            type="button"
          >
            Nilai ulang semua
          </button>
        </div>
      )}
      {isConfirmingAll && (
        <RegradeAllConfirmation
          isBusy={isRegradingAll}
          onConfirm={regradeAll}
          onDismiss={() => setIsConfirmingAll(false)}
          returnFocusRef={regradeAllTrigger}
        />
      )}
      <div
        className="mt-3 min-h-5 text-sm"
        role={notice?.type === 'failure' ? 'alert' : 'status'}
      >
        {notice && (
          <p className={notice.type === 'success' ? 'text-blue-800' : 'text-red-700'}>
            {notice.text}
          </p>
        )}
      </div>
      {recap && recap.items.length === 0 && (
        <p className="mt-4 text-slate-600">Belum ada mahasiswa yang terdaftar.</p>
      )}
      {recap && recap.items.length > 0 && (
        <div className="mt-2 overflow-x-auto">
          <table className="w-full border-collapse text-left text-sm">
            <caption className="sr-only">Nilai submission terakhir setiap mahasiswa</caption>
            <thead>
              <tr className="border-b border-slate-300">
                <th className="py-2 pr-4" scope="col">Mahasiswa</th>
                <th className="py-2 pr-4" scope="col">Jumlah submission</th>
                <th className="py-2 pr-4" scope="col">Status</th>
                <th className="py-2 pr-4" scope="col">Skor</th>
                <th className="py-2 pr-4" scope="col">Dikumpulkan</th>
                <th className="py-2" scope="col">Aksi</th>
              </tr>
            </thead>
            <tbody>
              {recap.items.map((row) => {
                const latest = row.latestSubmission
                return (
                  <tr className="border-b border-slate-200 align-top" key={row.userId}>
                    <th className="py-3 pr-4 font-normal" scope="row">
                      <span className="block font-medium">{row.name}</span>
                      <span className="block text-slate-600">{row.email}</span>
                    </th>
                    <td className="py-3 pr-4">{row.submissionCount}</td>
                    {latest ? (
                      <>
                        <td className="py-3 pr-4">
                          <span
                            className={`inline-flex rounded-full px-3 py-1 font-semibold ${statusStyles[latest.status]}`}
                          >
                            {statusLabels[latest.status]}
                          </span>
                          <span className="mt-1 flex flex-wrap gap-2">
                            {latest.isLate && (
                              <span className="rounded bg-amber-100 px-2 py-0.5 font-medium text-amber-900">
                                Terlambat
                              </span>
                            )}
                            {latest.staleGrade && (
                              <span className="rounded bg-orange-100 px-2 py-0.5 font-medium text-orange-900">
                                Nilai usang
                              </span>
                            )}
                          </span>
                        </td>
                        <td className="py-3 pr-4">
                          {formatScore(latest.totalScore, latest.maxScore)}
                        </td>
                        <td className="py-3 pr-4">
                          <time dateTime={latest.submittedAt}>
                            {dateFormatter.format(new Date(latest.submittedAt))}
                          </time>
                        </td>
                        <td className="py-3">
                          <div className="flex flex-wrap gap-2">
                            <Link
                              aria-label={`Laporan ${row.name}`}
                              className="rounded border border-slate-300 px-3 py-1.5 font-medium hover:bg-slate-100"
                              to={`/submissions/${encodeURIComponent(latest.id)}/report`}
                            >
                              Laporan
                            </Link>
                            <Link
                              aria-label={`Kode ${row.name}`}
                              className="rounded border border-slate-300 px-3 py-1.5 font-medium hover:bg-slate-100"
                              to={`/submissions/${encodeURIComponent(latest.id)}/code`}
                            >
                              Kode
                            </Link>
                            {regradableStatuses.has(latest.status) && (
                              <button
                                aria-label={`Nilai ulang ${row.name}`}
                                className="rounded border border-slate-300 px-3 py-1.5 font-medium hover:bg-slate-100 disabled:opacity-60"
                                disabled={busyRegradeId === latest.id}
                                onClick={() => regradeOne(row)}
                                type="button"
                              >
                                {busyRegradeId === latest.id ? 'Menjadwalkan...' : 'Nilai ulang'}
                              </button>
                            )}
                          </div>
                        </td>
                      </>
                    ) : (
                      <td className="py-3 pr-4 text-slate-600" colSpan={4}>
                        Belum mengumpulkan
                      </td>
                    )}
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
      {recap && (page > 1 || recap.hasNextPage) && (
        <nav aria-label="Paginasi rekap nilai" className="mt-5 flex items-center gap-3">
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
            disabled={!recap.hasNextPage}
            onClick={() => setPage((current) => current + 1)}
            type="button"
          >
            Berikutnya
          </button>
        </nav>
      )}
    </main>
  )
}

export default RecapPage
