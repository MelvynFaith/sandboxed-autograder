import { Suspense, useEffect, useState } from 'react'
import { Link, NavLink, Outlet, useParams } from 'react-router-dom'
import { getAssignment } from '../../api/assignments.js'
import { getResourceLimit } from '../../api/lecturerAssignments.js'
import { dateFormatter } from '../../components/submissionLabels.js'
import { submissionWindow, useNow, windowLabels, windowStyles } from './assignmentInfo.js'

const tabs = [
  { to: '.', label: 'Soal', end: true },
  { to: 'submit', label: 'Kirim kode' },
  { to: 'submissions', label: 'Submission saya' },
]

function tabClass({ isActive }) {
  return `-mb-px inline-flex min-h-11 items-center border-b-2 px-3 font-medium ${
    isActive
      ? 'border-blue-700 text-blue-800'
      : 'border-transparent text-slate-700 hover:border-slate-300 hover:text-slate-900'
  }`
}

// One assignment, laid out like a problem on Codeforces: the title and the deadlines on top, then
// the statement, the submit form and the student's own submissions as three linked views. Links
// rather than ARIA tabs, so each view has its own address and the Back button works.
function AssignmentLayout() {
  const { assignmentId } = useParams()
  const now = useNow()
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

  const windowState = assignment ? submissionWindow(assignment, now) : null

  return (
    <main className="mx-auto max-w-5xl px-4 py-8 sm:px-6">
      <nav aria-label="Breadcrumb" className="text-sm">
        <ol className="flex flex-wrap items-center gap-2 text-slate-600">
          <li>
            <Link className="text-blue-700 hover:underline" to="/assignments">
              Daftar Assignment
            </Link>
          </li>
          <li aria-hidden="true">/</li>
          <li aria-current="page" className="min-w-0 truncate text-slate-900">
            {assignment?.title ?? 'Assignment'}
          </li>
        </ol>
      </nav>
      {loadError && (
        <p className="mt-6 text-red-700" role="alert">
          Gagal memuat assignment: {loadError}
        </p>
      )}
      {!loadError && !assignment && <p className="mt-6 text-slate-600">Memuat assignment...</p>}
      {assignment && (
        <>
          <header className="mt-3">
            <div className="flex flex-wrap items-center gap-3">
              <h1 className="text-2xl font-semibold break-words">{assignment.title}</h1>
              <span
                className={`rounded-full px-3 py-1 text-sm font-semibold ${windowStyles[windowState]}`}
              >
                {windowLabels[windowState]}
              </span>
            </div>
            <dl className="mt-3 grid gap-x-6 gap-y-1 text-sm text-slate-700 sm:grid-cols-2">
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
            className="mt-6 flex flex-wrap gap-1 border-b border-slate-200"
          >
            {tabs.map((tab) => (
              <NavLink className={tabClass} end={tab.end} key={tab.label} to={tab.to}>
                {tab.label}
              </NavLink>
            ))}
          </nav>
          <Suspense fallback={<p className="mt-6 text-slate-600">Memuat...</p>}>
            <Outlet context={{ assignment, limits, windowState }} />
          </Suspense>
        </>
      )}
    </main>
  )
}

export default AssignmentLayout
