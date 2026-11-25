import { Suspense, useEffect, useState } from 'react'
import { Link, NavLink, Outlet, useParams } from 'react-router-dom'
import { getAssignment } from '../../api/assignments.js'
import { getResourceLimit } from '../../api/lecturerAssignments.js'
import { dateFormatter } from '../../components/submissionLabels.js'

const tabs = [
  { to: '.', label: 'Soal dan pengaturan', end: true },
  { to: 'testcases', label: 'Test case' },
  { to: 'recap', label: 'Rekap nilai' },
]

function tabClass({ isActive }) {
  return `-mb-px inline-flex min-h-11 items-center border-b-2 px-3 font-medium ${
    isActive
      ? 'border-accent font-semibold text-ink'
      : 'border-transparent text-muted hover:border-line-strong hover:text-ink'
  }`
}

// One assignment on the lecturer's side, laid out like the student's problem page: title and
// deadlines on top, then the statement and settings, the test cases and the recap as linked views
// with an address each. The courseId in the links is the course of the assignment, so the list
// opens on the same course again.
function LecturerAssignmentLayout() {
  const { assignmentId } = useParams()
  // The loaded data carries the id it belongs to, so a change of id shows the loading state
  // without resetting anything inside the effect.
  const [loaded, setLoaded] = useState({ id: null })

  useEffect(() => {
    let isCurrent = true
    Promise.all([getAssignment(assignmentId), getResourceLimit(assignmentId)])
      .then(([assignment, limits]) => {
        if (isCurrent) setLoaded({ id: assignmentId, assignment, limits })
      })
      .catch((error) => {
        if (isCurrent) setLoaded({ id: assignmentId, error: error.message })
      })
    return () => {
      isCurrent = false
    }
  }, [assignmentId])

  const current = loaded.id === assignmentId ? loaded : {}
  const { assignment = null, limits = null } = current
  const loadError = current.error ?? ''

  function handleSaved(saved, savedLimits) {
    setLoaded({ id: assignmentId, assignment: saved, limits: savedLimits })
  }

  return (
    <main className="mx-auto max-w-6xl px-4 py-8 sm:px-6">
      <nav aria-label="Breadcrumb" className="text-sm">
        <ol className="flex flex-wrap items-center gap-2 text-muted">
          <li>
            <Link
              className="text-link hover:underline"
              to={`/lecturer/assignments${
                assignment ? `?courseId=${encodeURIComponent(assignment.courseId)}` : ''
              }`}
            >
              Kelola Assignment
            </Link>
          </li>
          <li aria-hidden="true">/</li>
          <li aria-current="page" className="min-w-0 truncate text-ink">
            {assignment?.title ?? 'Assignment'}
          </li>
        </ol>
      </nav>
      {loadError && (
        <p className="mt-6 text-danger" role="alert">
          Gagal memuat assignment: {loadError}
        </p>
      )}
      {!loadError && !assignment && <p className="mt-6 text-muted">Memuat assignment...</p>}
      {assignment && (
        <>
          <header className="mt-3">
            <h1 className="text-2xl font-semibold break-words">{assignment.title}</h1>
            <dl className="mt-3 grid gap-x-6 gap-y-1 text-sm text-ink sm:grid-cols-2">
              <div className="flex flex-wrap gap-x-2">
                <dt>Deadline:</dt>
                <dd className="font-medium">
                  <time dateTime={assignment.deadlineAt}>
                    {dateFormatter.format(new Date(assignment.deadlineAt))} WIB
                  </time>
                </dd>
              </div>
              <div className="flex flex-wrap gap-x-2">
                <dt>Batas terlambat:</dt>
                <dd className="font-medium">
                  <time dateTime={assignment.lateUntilAt}>
                    {dateFormatter.format(new Date(assignment.lateUntilAt))} WIB
                  </time>
                </dd>
              </div>
            </dl>
          </header>
          <nav
            aria-label="Bagian assignment"
            className="mt-6 flex flex-wrap gap-1 border-b border-line"
          >
            {tabs.map((tab) => (
              <NavLink
                className={tabClass}
                end={tab.end}
                key={tab.label}
                to={{ pathname: tab.to, search: `?courseId=${encodeURIComponent(assignment.courseId)}` }}
              >
                {tab.label}
              </NavLink>
            ))}
          </nav>
          <div className="mt-6">
            <Suspense fallback={<p className="text-muted">Memuat...</p>}>
              <Outlet context={{ assignment, limits, onSaved: handleSaved }} />
            </Suspense>
          </div>
        </>
      )}
    </main>
  )
}

export default LecturerAssignmentLayout
