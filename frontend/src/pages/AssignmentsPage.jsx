import { useEffect, useState } from 'react'
import { getAssignments } from '../api/assignments.js'
import { MAX_FILE_SIZE, submitSubmission } from '../api/submissions.js'
import SubmissionStatus from '../components/SubmissionStatus.jsx'
import { useAuth } from '../context/useAuth.js'

const deadlineFormatter = new Intl.DateTimeFormat('id-ID', {
  dateStyle: 'long',
  timeStyle: 'short',
  timeZone: 'Asia/Jakarta',
})

function AssignmentsPage() {
  const { token, user } = useAuth()
  const [assignments, setAssignments] = useState([])
  const [isLoading, setIsLoading] = useState(true)
  const [loadError, setLoadError] = useState('')
  const [files, setFiles] = useState({})
  const [messages, setMessages] = useState({})
  const [submissions, setSubmissions] = useState({})
  const [submittingId, setSubmittingId] = useState(null)

  useEffect(() => {
    let isCurrent = true

    async function loadAssignments() {
      try {
        const results = await getAssignments()
        if (isCurrent) {
          setAssignments(results)
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

    loadAssignments()
    return () => {
      isCurrent = false
    }
  }, [])

  function handleFileChange(assignmentId, file) {
    setMessages((current) => ({ ...current, [assignmentId]: null }))

    if (!file) {
      setFiles((current) => ({ ...current, [assignmentId]: null }))
      return
    }

    if (!file.name.toLowerCase().endsWith('.py')) {
      setFiles((current) => ({ ...current, [assignmentId]: null }))
      setMessages((current) => ({
        ...current,
        [assignmentId]: { type: 'error', text: 'Pilih berkas dengan ekstensi .py.' },
      }))
      return
    }

    if (file.size > MAX_FILE_SIZE) {
      setFiles((current) => ({ ...current, [assignmentId]: null }))
      setMessages((current) => ({
        ...current,
        [assignmentId]: { type: 'error', text: 'Ukuran berkas maksimal 1 MiB.' },
      }))
      return
    }

    setFiles((current) => ({ ...current, [assignmentId]: file }))
  }

  async function handleSubmit(event, assignment) {
    event.preventDefault()
    const form = event.currentTarget
    const file = files[assignment.id]

    if (!file) {
      setMessages((current) => ({
        ...current,
        [assignment.id]: { type: 'error', text: 'Pilih berkas Python sebelum mengunggah.' },
      }))
      return
    }

    setSubmittingId(assignment.id)
    setMessages((current) => ({ ...current, [assignment.id]: null }))

    try {
      const submission = await submitSubmission(assignment.id, file)
      setFiles((current) => ({ ...current, [assignment.id]: null }))
      form.reset()
      setSubmissions((current) => ({
        ...current,
        [assignment.id]: submission.id,
      }))
      setMessages((current) => ({
        ...current,
        [assignment.id]: {
          type: 'success',
          text: 'Kode berhasil diunggah.',
        },
      }))
    } catch (error) {
      setMessages((current) => ({
        ...current,
        [assignment.id]: { type: 'error', text: error.message },
      }))
    } finally {
      setSubmittingId(null)
    }
  }

  return (
    <main className="mx-auto max-w-4xl px-6 py-10">
      <h1 className="text-2xl font-semibold">Daftar Assignment</h1>
      {isLoading && <p className="mt-4 text-slate-600">Memuat assignment...</p>}
      {loadError && (
        <p className="mt-4 text-red-700" role="alert">
          Gagal memuat assignment: {loadError}
        </p>
      )}
      {!isLoading && !loadError && assignments.length === 0 && (
        <p className="mt-4 text-slate-600">Belum ada assignment yang tersedia.</p>
      )}
      <ul className="mt-6 space-y-4">
        {assignments.map((assignment) => (
          <li
            className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm"
            key={assignment.id}
          >
            <h2 className="text-lg font-semibold">{assignment.judul}</h2>
            <p className="mt-2 text-slate-700">{assignment.deskripsi}</p>
            <p className="mt-3 text-sm text-slate-600">
              Deadline:{' '}
              <time dateTime={assignment.deadline}>
                {deadlineFormatter.format(new Date(assignment.deadline))}
              </time>
            </p>
            {user.role === 'mahasiswa' && (
              <form
                className="mt-5 space-y-3"
                onSubmit={(event) => handleSubmit(event, assignment)}
              >
                <div>
                  <label className="mb-1 block font-medium" htmlFor={`file-${assignment.id}`}>
                    Kode Python untuk {assignment.judul}
                  </label>
                  <input
                    accept=".py"
                    className="block w-full rounded border border-slate-300 bg-white p-2"
                    id={`file-${assignment.id}`}
                    onChange={(event) =>
                      handleFileChange(assignment.id, event.target.files?.[0] ?? null)
                    }
                    type="file"
                  />
                  <p className="mt-1 text-sm text-slate-600">Berkas .py, maksimal 1 MiB.</p>
                </div>
                {messages[assignment.id] && (
                  <p
                    className={
                      messages[assignment.id].type === 'success'
                        ? 'text-sm text-blue-800'
                        : 'text-sm text-red-700'
                    }
                    role={messages[assignment.id].type === 'success' ? 'status' : 'alert'}
                  >
                    {messages[assignment.id].text}
                  </p>
                )}
                {submissions[assignment.id] !== undefined && (
                  <SubmissionStatus
                    key={submissions[assignment.id]}
                    submissionId={submissions[assignment.id]}
                    token={token}
                  />
                )}
                <button
                  className="rounded bg-blue-700 px-4 py-2 font-medium text-white hover:bg-blue-800 disabled:opacity-60"
                  disabled={!files[assignment.id] || submittingId === assignment.id}
                  type="submit"
                >
                  {submittingId === assignment.id ? 'Mengunggah...' : 'Upload kode Python'}
                </button>
              </form>
            )}
          </li>
        ))}
      </ul>
    </main>
  )
}

export default AssignmentsPage
