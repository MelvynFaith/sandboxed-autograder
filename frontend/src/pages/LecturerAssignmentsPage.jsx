import { useEffect, useRef, useState } from 'react'
import { Link, useLocation, useSearchParams } from 'react-router-dom'
import { getAssignments } from '../api/assignments.js'
import { getCourses } from '../api/courses.js'
import { deleteLecturerAssignment } from '../api/lecturerAssignments.js'
import ConfirmDeleteDialog from '../components/ConfirmDeleteDialog.jsx'
import FormFeedback from '../components/FormFeedback.jsx'
import { dateFormatter } from '../components/submissionLabels.js'
import { useFeedback } from './lecturer/useFeedback.js'

// The lecturer's list of assignments of one course. Each row leads to the assignment's own pages
// (statement and settings, test cases, recap); creating one has its own page.
function LecturerAssignmentsPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const location = useLocation()
  const initialCourseId = useRef(searchParams.get('courseId'))
  const [courses, setCourses] = useState([])
  const [courseId, setCourseId] = useState(() => initialCourseId.current ?? '')
  const [assignments, setAssignments] = useState([])
  const [isLoading, setIsLoading] = useState(true)
  const [pendingDelete, setPendingDelete] = useState(null)
  const deleteTrigger = useRef(null)
  const { error, success, revision, setError, setSuccess } = useFeedback(
    location.state?.notice ?? '',
  )

  useEffect(() => {
    let isCurrent = true
    getCourses()
      .then((results) => {
        if (!isCurrent) return
        setCourses(results)
        const requested = results.find((course) => String(course.id) === initialCourseId.current)
        setCourseId(String(requested?.id ?? results[0]?.id ?? ''))
      })
      .catch((loadError) => {
        if (isCurrent) setError(loadError.message)
      })
      .finally(() => {
        if (isCurrent) setIsLoading(false)
      })
    return () => {
      isCurrent = false
    }
    // setError only sets state; the courses are loaded once.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    let isCurrent = true
    if (!courseId) {
      return undefined
    }
    getAssignments({ courseId })
      .then((results) => {
        if (isCurrent) setAssignments(results)
      })
      .catch((loadError) => {
        if (isCurrent) setError(loadError.message)
      })
    return () => {
      isCurrent = false
    }
    // setError only sets state; the list is loaded once per course.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [courseId])

  async function confirmDelete() {
    try {
      await deleteLecturerAssignment(pendingDelete.id)
      setAssignments((current) => current.filter((entry) => entry.id !== pendingDelete.id))
      setSuccess('Assignment berhasil dihapus.')
    } catch (deleteError) {
      setError(deleteError.message)
    } finally {
      setPendingDelete(null)
    }
  }

  const courseQuery = courseId ? `?courseId=${encodeURIComponent(courseId)}` : ''

  return (
    <main className="mx-auto max-w-6xl space-y-6 px-4 py-8 sm:px-6 sm:py-10">
      <h1 className="text-2xl font-semibold">Kelola Assignment</h1>
      <FormFeedback announcementKey={revision} error={error} success={success} />
      {isLoading ? (
        <p className="text-muted">Memuat Course...</p>
      ) : courses.length === 0 ? (
        <p className="text-muted">Belum ada Course yang dapat dikelola.</p>
      ) : (
        <>
          <div className="flex flex-wrap items-end justify-between gap-4">
            <div className="min-w-0 flex-1 sm:max-w-md">
              <label className="mb-1 block font-medium" htmlFor="course-select">Course</label>
              <select
                className="w-full rounded-md border border-line-strong p-2"
                id="course-select"
                onChange={(event) => {
                  setCourseId(event.target.value)
                  setSearchParams({ courseId: event.target.value })
                  setAssignments([])
                }}
                value={courseId}
              >
                {courses.map((course) => (
                  <option key={course.id} value={course.id}>
                    {course.code} — {course.name} ({course.term})
                  </option>
                ))}
              </select>
            </div>
            <Link
              className="rounded-md bg-accent px-4 py-2 font-medium text-white hover:bg-accent-hover"
              to={`/lecturer/assignments/new${courseQuery}`}
            >
              Buat assignment
            </Link>
          </div>

          <section aria-labelledby="assignments-title" className="space-y-3">
            <h2 className="text-xl font-semibold" id="assignments-title">Assignment pada Course</h2>
            {assignments.length === 0 ? (
              <p className="text-muted">Course ini belum punya assignment.</p>
            ) : (
              <div className="overflow-x-auto rounded-lg border border-line bg-surface">
                <table className="w-full border-collapse text-left text-sm">
                  <caption className="sr-only">Assignment pada Course yang dipilih</caption>
                  <thead>
                    <tr className="border-b border-line-strong">
                      <th className="px-4 py-2" scope="col">Judul</th>
                      <th className="px-4 py-2" scope="col">Deadline</th>
                      <th className="px-4 py-2" scope="col">Batas terlambat</th>
                      <th className="px-4 py-2" scope="col">Aksi</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-line">
                    {assignments.map((assignment) => (
                      <tr className="align-top" key={assignment.id}>
                        <th className="px-4 py-3 font-medium" scope="row">{assignment.title}</th>
                        <td className="px-4 py-3">
                          <time dateTime={assignment.deadlineAt}>
                            {dateFormatter.format(new Date(assignment.deadlineAt))}
                          </time>
                        </td>
                        <td className="px-4 py-3">
                          <time dateTime={assignment.lateUntilAt}>
                            {dateFormatter.format(new Date(assignment.lateUntilAt))}
                          </time>
                        </td>
                        <td className="px-4 py-3">
                          <div className="flex flex-wrap gap-2">
                            <Link
                              aria-label={`Kelola ${assignment.title}`}
                              className="rounded-md border border-line-strong px-3 py-1.5 font-medium hover:bg-raised"
                              to={`/lecturer/assignments/${encodeURIComponent(assignment.id)}${courseQuery}`}
                            >
                              Kelola
                            </Link>
                            <Link
                              aria-label={`Rekap nilai ${assignment.title}`}
                              className="rounded-md border border-line-strong px-3 py-1.5 font-medium text-link hover:bg-raised"
                              to={`/lecturer/assignments/${encodeURIComponent(assignment.id)}/recap${courseQuery}`}
                            >
                              Rekap nilai &amp; CSV
                            </Link>
                            <button
                              aria-label={`Hapus ${assignment.title}`}
                              className="rounded-md border border-danger/50 px-3 py-1.5 font-medium text-danger"
                              onClick={(event) => {
                                deleteTrigger.current = event.currentTarget
                                event.currentTarget.focus()
                                setPendingDelete({ id: assignment.id })
                              }}
                              type="button"
                            >
                              Hapus
                            </button>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </section>
        </>
      )}
      {pendingDelete && (
        <ConfirmDeleteDialog
          onConfirm={confirmDelete}
          onDismiss={() => setPendingDelete(null)}
          returnFocusRef={deleteTrigger}
        />
      )}
    </main>
  )
}

export default LecturerAssignmentsPage
