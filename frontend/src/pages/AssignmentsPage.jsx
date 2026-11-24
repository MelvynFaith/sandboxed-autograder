import { lazy, Suspense, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getAssignments } from '../api/assignments.js'
import { getCourseAssignmentsPage, getCoursesPage } from '../api/courses.js'
import { getSubmissionHistory } from '../api/submissionHistory.js'
import ScoreMeter from '../components/ScoreMeter.jsx'
import StatusBadge from '../components/StatusBadge.jsx'
import {
  formatScore,
  submissionStatusLabels,
  submissionStatusStyles,
} from '../components/submissionLabels.js'
import { useAuth } from '../context/useAuth.js'
import {
  submissionWindow,
  useNow,
  windowLabels,
  windowStyles,
} from './assignment/assignmentInfo.js'

const deadlineFormatter = new Intl.DateTimeFormat('id-ID', {
  dateStyle: 'medium',
  timeStyle: 'short',
  timeZone: 'Asia/Jakarta',
})
const PAGE_SIZE = 10
// Only the staff list renders statements; the Markdown renderer loads when it is needed.
const AssignmentDescription = lazy(() => import('../components/AssignmentDescription.jsx'))

// The student's latest submission for one assignment: one small request per row (FR-09).
function LatestSubmission({ assignment }) {
  const [state, setState] = useState({ loading: true })

  useEffect(() => {
    let isCurrent = true
    getSubmissionHistory({ page: 1, pageSize: 1, assignmentId: assignment.id })
      .then((result) => {
        if (isCurrent) setState({ latest: result.items[0] ?? null })
      })
      .catch(() => {
        if (isCurrent) setState({ failed: true })
      })
    return () => {
      isCurrent = false
    }
  }, [assignment.id])

  if (state.loading) {
    return <span className="text-muted">Memuat...</span>
  }
  if (state.failed) {
    return <span className="text-muted">Status tidak tersedia</span>
  }
  if (!state.latest) {
    return <span className="text-muted">Belum mengumpulkan</span>
  }
  const { status, totalScore, maxScore } = state.latest
  return (
    <span className="flex flex-wrap items-center gap-2">
      <StatusBadge
        className={submissionStatusStyles[status]}
        label={submissionStatusLabels[status]}
        status={status}
      />
      {status === 'COMPLETED' && (
        <span className="inline-flex items-center gap-2 font-medium tabular-nums">
          {formatScore(totalScore, maxScore)}
          <ScoreMeter max={maxScore} total={totalScore} />
        </span>
      )}
    </span>
  )
}

function Pager({ label, page, hasNext, onChange }) {
  return (
    <nav aria-label={label} className="mt-4 flex items-center justify-between gap-3">
      <button
        className="rounded-md border border-line-strong px-3 py-2 text-sm disabled:opacity-50"
        disabled={page <= 1}
        onClick={() => onChange(page - 1)}
        type="button"
      >
        Sebelumnya
      </button>
      <span className="text-sm text-muted">Halaman {page}</span>
      <button
        className="rounded-md border border-line-strong px-3 py-2 text-sm disabled:opacity-50"
        disabled={!hasNext}
        onClick={() => onChange(page + 1)}
        type="button"
      >
        Berikutnya
      </button>
    </nav>
  )
}

// Students: every Course they follow, each with a table of its assignments (title, deadlines,
// latest submission). The statement, the submit form and the submissions live on the page of each
// assignment. Lecturers keep the plain list of the assignments they can see.
function AssignmentsPage() {
  const { user } = useAuth()
  const now = useNow()
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

  function renderAssignmentTable(group) {
    // A narrow screen scrolls the table, never the page (NFR-11).
    return (
      <div className="mt-3 overflow-x-auto">
        <table className="w-full border-collapse text-left text-sm">
          <caption className="sr-only">Assignment pada {group.course.name}</caption>
          <thead>
            <tr className="border-b border-line-strong text-muted">
              <th className="py-2 pr-3 font-medium" scope="col">
                Assignment
              </th>
              <th className="py-2 pr-3 font-medium" scope="col">
                Deadline (WIB)
              </th>
              <th className="hidden py-2 pr-3 font-medium md:table-cell" scope="col">
                Batas terlambat (WIB)
              </th>
              <th className="py-2 font-medium" scope="col">
                Submission terakhir
              </th>
            </tr>
          </thead>
          <tbody>
            {group.items.map((assignment) => {
              const windowState = submissionWindow(assignment, now)
              return (
                <tr className="border-b border-line align-middle" key={assignment.id}>
                  <td className="py-3 pr-3">
                    <Link
                      className="inline-flex min-h-11 items-center font-medium break-words text-link hover:underline"
                      to={`/assignments/${encodeURIComponent(assignment.id)}`}
                    >
                      {assignment.title}
                    </Link>
                  </td>
                  <td className="py-3 pr-3">
                    <time dateTime={assignment.deadlineAt}>
                      {deadlineFormatter.format(new Date(assignment.deadlineAt))}
                    </time>
                    {windowState !== 'open' && (
                      <span
                        className={`mt-1 block w-fit rounded-md px-2 py-0.5 text-xs font-medium ${windowStyles[windowState]}`}
                      >
                        {windowLabels[windowState]}
                      </span>
                    )}
                  </td>
                  <td className="hidden py-3 pr-3 md:table-cell">
                    <time dateTime={assignment.lateUntilAt}>
                      {deadlineFormatter.format(new Date(assignment.lateUntilAt))}
                    </time>
                  </td>
                  <td className="py-3">
                    <LatestSubmission assignment={assignment} />
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    )
  }

  return (
    <main className="mx-auto max-w-6xl px-4 py-8 sm:px-6">
      <h1 className="text-2xl font-semibold">Daftar Assignment</h1>
      {isLoading && <p className="mt-4 text-muted">Memuat assignment...</p>}
      {loadError && (
        <p className="mt-4 text-danger" role="alert">
          Gagal memuat assignment: {loadError}
        </p>
      )}
      {!isLoading && !loadError && user.role === 'mahasiswa' && courseGroups.length === 0 && (
        <p className="mt-4 text-muted">Belum ada Course yang diikuti.</p>
      )}
      {!isLoading && !loadError && user.role !== 'mahasiswa' && assignments.length === 0 && (
        <p className="mt-4 text-muted">Belum ada assignment yang tersedia.</p>
      )}
      {user.role === 'mahasiswa' ? (
        <>
          {courseGroups.map((group) => (
            <section
              aria-labelledby={`course-${group.course.id}`}
              className="mt-6 rounded-lg border border-line bg-surface p-4 sm:p-5"
              key={group.course.id}
            >
              <h2 className="text-xl font-semibold" id={`course-${group.course.id}`}>
                {group.course.code} — {group.course.name}
              </h2>
              {group.items.length === 0 ? (
                <p className="mt-3 text-muted">Belum ada assignment pada Course ini.</p>
              ) : (
                renderAssignmentTable(group)
              )}
              {(group.page > 1 || group.has_next) && (
                <Pager
                  hasNext={group.has_next}
                  label={`Halaman assignment ${group.course.name}`}
                  onChange={(page) => changeAssignmentPage(group.course.id, page)}
                  page={group.page}
                />
              )}
            </section>
          ))}
          {courseGroups.length > 0 && (coursePage.page > 1 || coursePage.has_next) && (
            <Pager
              hasNext={coursePage.has_next}
              label="Halaman daftar Course"
              onChange={setCoursePageNumber}
              page={coursePage.page}
            />
          )}
        </>
      ) : (
        <ul className="mt-6 space-y-4">
          {assignments.map((assignment) => (
            <li
              className="rounded-lg border border-line bg-surface p-5"
              key={assignment.id}
            >
              <h2 className="text-lg font-semibold">{assignment.title}</h2>
              <Suspense fallback={<p className="mt-3 text-muted">{assignment.description}</p>}>
                <AssignmentDescription text={assignment.description} />
              </Suspense>
              <p className="mt-3 text-sm text-muted">
                Deadline:{' '}
                <time dateTime={assignment.deadlineAt}>
                  {deadlineFormatter.format(new Date(assignment.deadlineAt))} WIB
                </time>
              </p>
            </li>
          ))}
        </ul>
      )}
    </main>
  )
}

export default AssignmentsPage
