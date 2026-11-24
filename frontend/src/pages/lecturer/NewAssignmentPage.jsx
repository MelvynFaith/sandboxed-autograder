import { useEffect, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { getCourses } from '../../api/courses.js'
import FormFeedback from '../../components/FormFeedback.jsx'
import AssignmentForm from './AssignmentForm.jsx'
import { useFeedback } from './useFeedback.js'

// Creating an assignment is its own page: the course comes from the address, and after the save
// the lecturer lands on the test cases of the new assignment, which is the next thing to write.
function NewAssignmentPage() {
  const [searchParams] = useSearchParams()
  const courseId = searchParams.get('courseId') ?? ''
  const navigate = useNavigate()
  const [course, setCourse] = useState(null)
  const [isLoading, setIsLoading] = useState(true)
  const { error, success, revision, setError, setSuccess } = useFeedback()

  useEffect(() => {
    let isCurrent = true
    getCourses()
      .then((courses) => {
        if (isCurrent) setCourse(courses.find((entry) => String(entry.id) === courseId) ?? null)
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
    // setError only sets state; the courses are loaded once per course in the address.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [courseId])

  return (
    <main className="mx-auto max-w-3xl px-4 py-8 sm:px-6">
      <nav aria-label="Breadcrumb" className="text-sm">
        <ol className="flex flex-wrap items-center gap-2 text-muted">
          <li>
            <Link
              className="text-link hover:underline"
              to={`/lecturer/assignments${courseId ? `?courseId=${encodeURIComponent(courseId)}` : ''}`}
            >
              Kelola Assignment
            </Link>
          </li>
          <li aria-hidden="true">/</li>
          <li aria-current="page" className="text-ink">Assignment baru</li>
        </ol>
      </nav>
      <h1 className="mt-3 text-2xl font-semibold">Assignment baru</h1>
      {course && (
        <p className="mt-1 text-sm text-muted">
          Course: {course.code} — {course.name} ({course.term})
        </p>
      )}
      <div className="mt-6 space-y-6">
        <FormFeedback announcementKey={revision} error={error} success={success} />
        {isLoading ? (
          <p className="text-muted">Memuat Course...</p>
        ) : !course ? (
          <p className="text-muted">
            Course tidak ditemukan. Pilih Course di halaman Kelola Assignment.
          </p>
        ) : (
          <AssignmentForm
            courseId={course.id}
            onSaved={(saved) =>
              navigate(
                `/lecturer/assignments/${encodeURIComponent(saved.id)}/testcases?courseId=${encodeURIComponent(course.id)}`,
                { state: { notice: 'Assignment berhasil dibuat.' } },
              )
            }
            setError={setError}
            setSuccess={setSuccess}
          />
        )}
      </div>
    </main>
  )
}

export default NewAssignmentPage
