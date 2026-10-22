import { useEffect, useState } from 'react'
import { getAssignments } from '../api/assignments.js'
import { getCourseAssignmentsPage, getCoursesPage } from '../api/courses.js'
import { MAX_FILE_SIZE, submitSubmission } from '../api/submissions.js'
import SubmissionStatus from '../components/SubmissionStatus.jsx'
import { useAuth } from '../context/useAuth.js'

const deadlineFormatter = new Intl.DateTimeFormat('id-ID', {
  dateStyle: 'long',
  timeStyle: 'short',
  timeZone: 'Asia/Jakarta',
})
const PAGE_SIZE = 10

function AssignmentsPage() {
  const { user } = useAuth()
  const [assignments, setAssignments] = useState([])
  const [courseGroups, setCourseGroups] = useState([])
  const [coursePage, setCoursePage] = useState({
    items: [],
    page: 1,
    page_size: PAGE_SIZE,
    has_next: false,
  })
  const [coursePageNumber, setCoursePageNumber] = useState(1)
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
        if (user.role !== 'mahasiswa') {
          const results = await getAssignments()
          if (isCurrent) setAssignments(results)
          return
        }

        const result = await getCoursesPage({
          page: coursePageNumber,
          pageSize: PAGE_SIZE,
        })
        const groups = await Promise.all(
          result.items.map(async (course) => {
            const assignmentPage = await getCourseAssignmentsPage(course.id, {
              page: 1,
              pageSize: PAGE_SIZE,
            })
            return {
              course,
              ...assignmentPage,
            }
          }),
        )
        if (isCurrent) {
          setCoursePage(result)
          setCourseGroups(groups)
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
  }, [coursePageNumber, user.role])

  async function changeAssignmentPage(courseId, page) {
    try {
      const result = await getCourseAssignmentsPage(courseId, {
        page,
        pageSize: PAGE_SIZE,
      })
      setCourseGroups((current) =>
        current.map((group) =>
          String(group.course.id) === String(courseId) ? { ...group, ...result } : group,
        ),
      )
    } catch (error) {
      setLoadError(error.message)
    }
  }

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
        [assignmentId]: { type: 'failure', text: 'Pilih berkas dengan ekstensi .py.' },
      }))
      return
    }

    if (file.size > MAX_FILE_SIZE) {
      setFiles((current) => ({ ...current, [assignmentId]: null }))
      setMessages((current) => ({
        ...current,
        [assignmentId]: { type: 'failure', text: 'Ukuran berkas maksimal 256 KB.' },
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
        [assignment.id]: { type: 'failure', text: 'Pilih berkas Python sebelum mengunggah.' },
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
        [assignment.id]: { type: 'failure', text: error.message },
      }))
    } finally {
      setSubmittingId(null)
    }
  }

  function renderAssignment(assignment) {
    return (
      <li
        className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm"
        key={assignment.id}
      >
        <h3 className="text-lg font-semibold">{assignment.title}</h3>
        <p className="mt-2 text-slate-700">{assignment.description}</p>
        <p className="mt-3 text-sm text-slate-600">
          Deadline:{' '}
          <time dateTime={assignment.deadlineAt}>
            {deadlineFormatter.format(new Date(assignment.deadlineAt))}
          </time>
        </p>
        {user.role === 'mahasiswa' && (
          <form
            className="mt-5 space-y-3"
            onSubmit={(event) => handleSubmit(event, assignment)}
          >
            <div>
              <label className="mb-1 block font-medium" htmlFor={`file-${assignment.id}`}>
                Kode Python untuk {assignment.title}
              </label>
              <input
                accept=".py"
                className="block w-full min-w-0 rounded border border-slate-300 bg-white p-2"
                id={`file-${assignment.id}`}
                onChange={(event) =>
                  handleFileChange(assignment.id, event.target.files?.[0] ?? null)
                }
                type="file"
              />
              <p className="mt-1 text-sm text-slate-600">Berkas .py, maksimal 256 KB.</p>
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
    )
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
      {!isLoading && !loadError && user.role === 'mahasiswa' && courseGroups.length === 0 && (
        <p className="mt-4 text-slate-600">Belum ada Course yang diikuti.</p>
      )}
      {!isLoading && !loadError && user.role !== 'mahasiswa' && assignments.length === 0 && (
        <p className="mt-4 text-slate-600">Belum ada assignment yang tersedia.</p>
      )}
      {user.role === 'mahasiswa' ? (
        <>
          {courseGroups.map((group) => (
            <section className="mt-6" key={group.course.id}>
              <h2 className="text-xl font-semibold">
                {group.course.code} — {group.course.name}
              </h2>
              {group.items.length === 0 ? (
                <p className="mt-3 text-slate-600">Belum ada assignment pada Course ini.</p>
              ) : (
                <ul className="mt-4 space-y-4">{group.items.map(renderAssignment)}</ul>
              )}
              <nav
                aria-label={`Halaman assignment ${group.course.name}`}
                className="mt-4 flex items-center justify-between gap-3"
              >
                <button
                  className="rounded border border-slate-300 px-3 py-2 text-sm disabled:opacity-50"
                  disabled={group.page <= 1}
                  onClick={() => changeAssignmentPage(group.course.id, group.page - 1)}
                  type="button"
                >
                  Sebelumnya
                </button>
                <span className="text-sm text-slate-600">Halaman {group.page}</span>
                <button
                  className="rounded border border-slate-300 px-3 py-2 text-sm disabled:opacity-50"
                  disabled={!group.has_next}
                  onClick={() => changeAssignmentPage(group.course.id, group.page + 1)}
                  type="button"
                >
                  Berikutnya
                </button>
              </nav>
            </section>
          ))}
          {courseGroups.length > 0 && (
            <nav aria-label="Halaman daftar Course" className="mt-8 flex items-center justify-between gap-3">
              <button
                className="rounded border border-slate-300 px-3 py-2 text-sm disabled:opacity-50"
                disabled={coursePage.page <= 1}
                onClick={() => setCoursePageNumber((page) => page - 1)}
                type="button"
              >
                Sebelumnya
              </button>
              <span className="text-sm text-slate-600">Halaman {coursePage.page}</span>
              <button
                className="rounded border border-slate-300 px-3 py-2 text-sm disabled:opacity-50"
                disabled={!coursePage.has_next}
                onClick={() => setCoursePageNumber((page) => page + 1)}
                type="button"
              >
                Berikutnya
              </button>
            </nav>
          )}
        </>
      ) : (
        <ul className="mt-6 space-y-4">{assignments.map(renderAssignment)}</ul>
      )}
    </main>
  )
}

export default AssignmentsPage
