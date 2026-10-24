import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import {
  createCourse,
  createEnrollment,
  deleteEnrollment,
  getCourseEnrollmentsPage,
  getCoursesPage,
} from '../api/courses.js'
import { useAuth } from '../context/useAuth.js'

const PAGE_SIZE = 10

function PageControls({ page, hasNext, onPageChange, label }) {
  return (
    <nav aria-label={label} className="mt-4 flex items-center justify-between gap-3">
      <button
        className="rounded border border-slate-300 px-3 py-2 text-sm font-medium hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-50"
        disabled={page <= 1}
        onClick={() => onPageChange(page - 1)}
        type="button"
      >
        Sebelumnya
      </button>
      <span className="text-sm text-slate-600">Halaman {page}</span>
      <button
        className="rounded border border-slate-300 px-3 py-2 text-sm font-medium hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-50"
        disabled={!hasNext}
        onClick={() => onPageChange(page + 1)}
        type="button"
      >
        Berikutnya
      </button>
    </nav>
  )
}

function CoursesPage() {
  const { user } = useAuth()
  const [searchParams, setSearchParams] = useSearchParams()
  const requestedCourseId = searchParams.get('courseId')
  const isAdmin = user.role === 'admin'
  const [coursesPage, setCoursesPage] = useState({
    items: [],
    page: 1,
    page_size: PAGE_SIZE,
    has_next: false,
  })
  const [coursePage, setCoursePage] = useState(1)
  const [selectedCourseId, setSelectedCourseId] = useState('')
  const [enrollmentsPage, setEnrollmentsPage] = useState({
    items: [],
    page: 1,
    page_size: PAGE_SIZE,
    has_next: false,
  })
  const [enrollmentPage, setEnrollmentPage] = useState(1)
  const [courseForm, setCourseForm] = useState({
    name: '',
    code: '',
    term: '',
    lecturerEmail: '',
  })
  const [enrollmentEmail, setEnrollmentEmail] = useState('')
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const selectedCourse = coursesPage.items.find(
    (course) => String(course.id) === selectedCourseId,
  )
  const activeCourseId = selectedCourse
    ? selectedCourseId
    : String(coursesPage.items[0]?.id ?? '')
  const activeCourse = coursesPage.items.find(
    (course) => String(course.id) === activeCourseId,
  )

  useEffect(() => {
    let isCurrent = true

    getCoursesPage({ page: coursePage, pageSize: PAGE_SIZE })
      .then((result) => {
        if (!isCurrent) return
        setCoursesPage(result)
        const requestedCourse = result.items.find(
          (course) => String(course.id) === requestedCourseId,
        )
        setSelectedCourseId(String(requestedCourse?.id ?? result.items[0]?.id ?? ''))
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
  }, [coursePage, requestedCourseId])

  useEffect(() => {
    if (!activeCourseId) return undefined
    let isCurrent = true
    getCourseEnrollmentsPage(activeCourseId, {
      page: enrollmentPage,
      pageSize: PAGE_SIZE,
    })
      .then((result) => {
        if (isCurrent) setEnrollmentsPage(result)
      })
      .catch((loadError) => {
        if (isCurrent) setError(loadError.message)
      })

    return () => {
      isCurrent = false
    }
  }, [activeCourseId, enrollmentPage])

  async function refreshEnrollments() {
    const result = await getCourseEnrollmentsPage(activeCourseId, {
      page: enrollmentPage,
      pageSize: PAGE_SIZE,
    })
    setEnrollmentsPage(result)
  }

  async function handleCourseSubmit(event) {
    event.preventDefault()
    setError('')
    setSuccess('')

    const course = {
      name: courseForm.name.trim(),
      code: courseForm.code.trim(),
      term: courseForm.term.trim(),
      lecturerEmail: courseForm.lecturerEmail.trim(),
    }
    if (!course.name || !course.code || !course.term) {
      setError('Nama Course, kode, dan term wajib diisi.')
      return
    }
    if (isAdmin && !course.lecturerEmail) {
      setError('Email dosen pengampu wajib diisi.')
      return
    }
    if (isAdmin && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(course.lecturerEmail)) {
      setError('Format email dosen pengampu tidak valid.')
      return
    }

    try {
      const created = await createCourse(course, user.role)
      setCourseForm({ name: '', code: '', term: '', lecturerEmail: '' })
      setCoursePage(1)
      setSelectedCourseId(String(created.id))
      setSearchParams({ courseId: String(created.id) })
      const result = await getCoursesPage({ page: 1, pageSize: PAGE_SIZE })
      setCoursesPage(result)
      setSuccess('Course berhasil dibuat.')
    } catch (createError) {
      setError(createError.message)
    }
  }

  async function handleEnrollmentSubmit(event) {
    event.preventDefault()
    setError('')
    setSuccess('')
    const email = enrollmentEmail.trim()
    if (!activeCourseId || !email) {
      setError('Pilih Course dan masukkan email mahasiswa.')
      return
    }

    try {
      await createEnrollment(activeCourseId, { email })
      setEnrollmentEmail('')
      await refreshEnrollments()
      setSuccess('Mahasiswa berhasil ditambahkan ke Course.')
    } catch (createError) {
      setError(createError.message)
    }
  }

  async function handleDeleteEnrollment(userId) {
    if (!activeCourseId) return

    setError('')
    setSuccess('')
    try {
      await deleteEnrollment(activeCourseId, userId)
      await refreshEnrollments()
      setSuccess('Enrollment berhasil dihapus.')
    } catch (deleteError) {
      setError(deleteError.message)
    }
  }

  return (
    <main className="mx-auto max-w-5xl px-4 py-8 sm:px-6 sm:py-10">
      <h1 className="text-2xl font-semibold">Course</h1>
      <nav aria-label="Alur dosen" className="mt-4 rounded-lg border border-slate-200 bg-white p-4">
        <ol className="flex flex-wrap gap-x-6 gap-y-2 text-sm">
          <li aria-current="step" className="font-semibold text-blue-800">
            1. Course &amp; enrollment
          </li>
          <li>
            <Link
              className="font-medium text-blue-700 underline"
              to={`/lecturer/assignments${activeCourseId ? `?courseId=${encodeURIComponent(activeCourseId)}` : ''}`}
            >
              2. Assignment &amp; test case
            </Link>
          </li>
          <li className="text-slate-600">3. Rekap nilai &amp; CSV</li>
        </ol>
      </nav>
      {error && (
        <p className="mt-4 rounded border border-red-200 bg-red-50 px-4 py-3 text-red-700" role="alert">
          {error}
        </p>
      )}
      {success && (
        <p className="mt-4 rounded border border-blue-200 bg-blue-50 px-4 py-3 text-blue-800" role="status">
          {success}
        </p>
      )}

      <section className="mt-6 rounded-lg border border-slate-200 bg-white p-4 shadow-sm sm:p-5">
        <h2 className="text-lg font-semibold">Buat Course baru</h2>
        <form className="mt-4 grid gap-4 md:grid-cols-3" onSubmit={handleCourseSubmit}>
          <label className="block text-sm font-medium text-slate-700">
            Nama
            <input
              className="mt-1 block w-full min-w-0 rounded border border-slate-300 bg-white p-2"
              onChange={(event) => setCourseForm((current) => ({ ...current, name: event.target.value }))}
              value={courseForm.name}
            />
          </label>
          <label className="block text-sm font-medium text-slate-700">
            Kode
            <input
              className="mt-1 block w-full min-w-0 rounded border border-slate-300 bg-white p-2"
              onChange={(event) => setCourseForm((current) => ({ ...current, code: event.target.value }))}
              value={courseForm.code}
            />
          </label>
          <label className="block text-sm font-medium text-slate-700">
            Term
            <input
              className="mt-1 block w-full min-w-0 rounded border border-slate-300 bg-white p-2"
              onChange={(event) => setCourseForm((current) => ({ ...current, term: event.target.value }))}
              value={courseForm.term}
            />
          </label>
          {isAdmin && (
            <label className="block text-sm font-medium text-slate-700">
              Email dosen pengampu
              <input
                className="mt-1 block w-full min-w-0 rounded border border-slate-300 bg-white p-2"
                autoComplete="email"
                onChange={(event) =>
                  setCourseForm((current) => ({ ...current, lecturerEmail: event.target.value }))
                }
                required
                type="email"
                value={courseForm.lecturerEmail}
              />
            </label>
          )}
          <div className="md:col-span-3">
            <button
              className="w-full rounded bg-blue-700 px-4 py-2 font-medium text-white hover:bg-blue-800 sm:w-auto"
              type="submit"
            >
              Simpan Course
            </button>
          </div>
        </form>
      </section>

      <section className="mt-6">
        <h2 className="text-xl font-semibold">Daftar Course</h2>
        {isLoading ? (
          <p className="mt-4 text-slate-600">Memuat Course...</p>
        ) : coursesPage.items.length === 0 ? (
          <p className="mt-4 text-slate-600">Belum ada Course yang dapat dikelola.</p>
        ) : (
          <>
            <ul className="mt-4 grid gap-3 sm:grid-cols-2">
              {coursesPage.items.map((course) => (
                <li key={course.id}>
                  <button
                    aria-pressed={activeCourseId === String(course.id)}
                    className={`w-full rounded-lg border p-4 text-left ${
                      activeCourseId === String(course.id)
                        ? 'border-blue-600 bg-blue-50'
                        : 'border-slate-200 bg-white hover:bg-slate-50'
                    }`}
                    onClick={() => {
                      setError('')
                      setSelectedCourseId(String(course.id))
                      setSearchParams({ courseId: String(course.id) })
                      setEnrollmentPage(1)
                    }}
                    type="button"
                  >
                    <span className="block font-medium">{course.code} — {course.name}</span>
                    <span className="mt-1 block text-sm text-slate-600">Term: {course.term}</span>
                  </button>
                </li>
              ))}
            </ul>
            <PageControls
              hasNext={coursesPage.has_next}
              label="Halaman daftar Course"
              onPageChange={(page) => {
                setError('')
                setIsLoading(true)
                setEnrollmentPage(1)
                setCoursePage(page)
              }}
              page={coursesPage.page}
            />
          </>
        )}
      </section>

      {activeCourse && (
        <section className="mt-8 rounded-lg border border-slate-200 bg-white p-4 shadow-sm sm:p-5">
          <h2 className="text-lg font-semibold">Enrollment untuk {activeCourse.name}</h2>
          <form className="mt-4 flex flex-col gap-3 sm:flex-row" onSubmit={handleEnrollmentSubmit}>
            <label className="sr-only" htmlFor="enrollment-email">Email mahasiswa</label>
            <input
              className="min-w-0 flex-1 rounded border border-slate-300 bg-white p-2"
              id="enrollment-email"
              onChange={(event) => setEnrollmentEmail(event.target.value)}
              placeholder="Email mahasiswa"
              type="email"
              value={enrollmentEmail}
            />
            <button
              className="rounded bg-blue-700 px-4 py-2 font-medium text-white hover:bg-blue-800"
              type="submit"
            >
              Tambah enrollment
            </button>
          </form>
          {enrollmentsPage.items.length === 0 ? (
            <p className="mt-4 text-slate-600">Belum ada mahasiswa yang terdaftar pada Course ini.</p>
          ) : (
            <ul className="mt-4 divide-y divide-slate-200">
              {enrollmentsPage.items.map((entry) => (
                <li
                  className="flex flex-col gap-3 py-3 sm:flex-row sm:items-center sm:justify-between"
                  key={entry.id}
                >
                  <div className="min-w-0">
                    <p className="break-words font-medium">{entry.name}</p>
                    <p className="break-all text-sm text-slate-600">{entry.email}</p>
                  </div>
                  <button
                    className="self-start rounded border border-red-300 px-3 py-2 text-sm text-red-700 hover:bg-red-50 sm:self-auto"
                    onClick={() => handleDeleteEnrollment(entry.userId)}
                    type="button"
                  >
                    Keluarkan mahasiswa
                  </button>
                </li>
              ))}
            </ul>
          )}
          <PageControls
            hasNext={enrollmentsPage.has_next}
            label="Halaman enrollment"
            onPageChange={setEnrollmentPage}
            page={enrollmentsPage.page}
          />
        </section>
      )}
    </main>
  )
}

export default CoursesPage
