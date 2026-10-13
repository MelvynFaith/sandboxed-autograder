import { useState } from 'react'
import { createLecturerAssignment } from '../api/lecturerAssignments.js'
import { createTestCase } from '../api/testcases.js'

const resourceLimitFields = [
  { name: 'cpu_time_ms', label: 'Batas waktu CPU (ms)' },
  { name: 'memory_mb', label: 'Batas memori (MB)' },
  { name: 'max_proses', label: 'Jumlah proses maksimum' },
  { name: 'timeout_s', label: 'Waktu maksimum (detik)' },
]

const initialResourceLimits = {
  cpu_time_ms: '',
  memory_mb: '',
  max_proses: '',
  timeout_s: '',
}

function LecturerAssignmentsPage() {
  const [assignment, setAssignment] = useState(null)
  const [assignmentError, setAssignmentError] = useState('')
  const [assignmentMessage, setAssignmentMessage] = useState('')
  const [isCreatingAssignment, setIsCreatingAssignment] = useState(false)
  const [resourceLimits, setResourceLimits] = useState(initialResourceLimits)
  const [testCaseError, setTestCaseError] = useState('')
  const [testCaseMessage, setTestCaseMessage] = useState('')
  const [isCreatingTestCase, setIsCreatingTestCase] = useState(false)

  async function handleCreateAssignment(event) {
    event.preventDefault()
    setAssignmentError('')
    setAssignmentMessage('')

    const limits = Object.fromEntries(
      Object.entries(resourceLimits).map(([name, value]) => [name, Number(value)]),
    )
    if (Object.values(limits).some((value) => !Number.isFinite(value) || value <= 0)) {
      setAssignmentError('Semua batas sumber daya harus berupa angka positif.')
      return
    }

    const formData = new FormData(event.currentTarget)
    const title = formData.get('judul').trim()
    const description = formData.get('deskripsi').trim()
    const deadline = formData.get('deadline')
    if (!title || !description || !deadline || !Number.isFinite(Date.parse(deadline))) {
      setAssignmentError('Judul, deskripsi, dan deadline wajib diisi dengan benar.')
      return
    }

    const data = {
      judul: title,
      deskripsi: description,
      deadline: new Date(deadline).toISOString(),
      resource_limit: limits,
    }

    setIsCreatingAssignment(true)
    try {
      const createdAssignment = await createLecturerAssignment(data)
      setAssignment(createdAssignment)
      setAssignmentMessage('Assignment berhasil dibuat. Tambahkan test case.')
    } catch (error) {
      setAssignmentError(error.message)
    } finally {
      setIsCreatingAssignment(false)
    }
  }

  async function handleCreateTestCase(event) {
    event.preventDefault()
    const form = event.currentTarget
    setTestCaseError('')
    setTestCaseMessage('')

    const formData = new FormData(event.currentTarget)
    const score = Number(formData.get('bobot'))
    if (!Number.isFinite(score) || score <= 0) {
      setTestCaseError('Bobot harus berupa angka positif.')
      return
    }

    const testCase = {
      input: formData.get('input'),
      expected_output: formData.get('expected_output'),
      bobot: score,
      is_hidden: formData.get('is_hidden') === 'on',
    }

    setIsCreatingTestCase(true)
    try {
      await createTestCase(assignment.id, testCase)
      setTestCaseMessage('Test case berhasil ditambahkan.')
      form.reset()
    } catch (error) {
      setTestCaseError(error.message)
    } finally {
      setIsCreatingTestCase(false)
    }
  }

  return (
    <main className="mx-auto max-w-4xl space-y-8 px-6 py-10">
      <h1 className="text-2xl font-semibold">Kelola Assignment</h1>

      {!assignment && (
        <form
          className="space-y-5 rounded-lg border border-slate-200 bg-white p-5 shadow-sm"
          noValidate
          onSubmit={handleCreateAssignment}
        >
          <h2 className="text-lg font-semibold">Buat assignment</h2>
          <div>
            <label className="mb-1 block font-medium" htmlFor="assignment-title">
              Judul
            </label>
            <input
              className="w-full rounded border border-slate-300 p-2"
              id="assignment-title"
              name="judul"
              required
              type="text"
            />
          </div>
          <div>
            <label className="mb-1 block font-medium" htmlFor="assignment-description">
              Deskripsi
            </label>
            <textarea
              className="w-full rounded border border-slate-300 p-2"
              id="assignment-description"
              name="deskripsi"
              required
              rows="3"
            />
          </div>
          <div>
            <label className="mb-1 block font-medium" htmlFor="assignment-deadline">
              Deadline
            </label>
            <input
              className="w-full rounded border border-slate-300 p-2"
              id="assignment-deadline"
              name="deadline"
              required
              type="datetime-local"
            />
          </div>
          <fieldset className="space-y-3">
            <legend className="font-semibold">Batas sumber daya</legend>
            {resourceLimitFields.map(({ name, label }) => (
              <div key={name}>
                <label className="mb-1 block font-medium" htmlFor={`resource-${name}`}>
                  {label}
                </label>
                <input
                  className="w-full rounded border border-slate-300 p-2"
                  id={`resource-${name}`}
                  min="0.01"
                  name={name}
                  onChange={(event) =>
                    setResourceLimits((current) => ({
                      ...current,
                      [name]: event.target.value,
                    }))
                  }
                  required
                  step="any"
                  type="number"
                  value={resourceLimits[name]}
                />
              </div>
            ))}
          </fieldset>
          {assignmentError && (
            <p className="text-red-700" role="alert">
              Gagal membuat assignment: {assignmentError}
            </p>
          )}
          <button
            className="rounded bg-blue-700 px-4 py-2 font-medium text-white hover:bg-blue-800 disabled:opacity-60"
            disabled={isCreatingAssignment}
            type="submit"
          >
            {isCreatingAssignment ? 'Membuat...' : 'Buat assignment'}
          </button>
        </form>
      )}

      {assignment && (
        <section aria-labelledby="created-assignment-title">
          <h2 className="text-lg font-semibold" id="created-assignment-title">
            {assignment.judul}
          </h2>
          <p className="mt-2 text-slate-700">{assignment.deskripsi}</p>
          <p className="mt-3 text-blue-800" role="status">
            {assignmentMessage}
          </p>

          <form
            className="mt-6 space-y-5 rounded-lg border border-slate-200 bg-white p-5 shadow-sm"
            noValidate
            onSubmit={handleCreateTestCase}
          >
            <h3 className="text-lg font-semibold">Tambah test case</h3>
            <div>
              <label className="mb-1 block font-medium" htmlFor="testcase-input">
                Input
              </label>
              <textarea
                className="w-full rounded border border-slate-300 p-2"
                id="testcase-input"
                name="input"
                rows="3"
              />
            </div>
            <div>
              <label className="mb-1 block font-medium" htmlFor="testcase-expected-output">
                Keluaran yang diharapkan
              </label>
              <textarea
                className="w-full rounded border border-slate-300 p-2"
                id="testcase-expected-output"
                name="expected_output"
                rows="3"
              />
            </div>
            <div>
              <label className="mb-1 block font-medium" htmlFor="testcase-score">
                Bobot
              </label>
              <input
                className="w-full rounded border border-slate-300 p-2"
                id="testcase-score"
                min="0.01"
                name="bobot"
                required
                step="any"
                type="number"
              />
            </div>
            <label className="flex items-center gap-2">
              <input name="is_hidden" type="checkbox" />
              Test case tersembunyi
            </label>
            {testCaseError && (
              <p className="text-red-700" role="alert">
                Gagal menambahkan test case: {testCaseError}
              </p>
            )}
            {testCaseMessage && (
              <p className="text-blue-800" role="status">
                {testCaseMessage}
              </p>
            )}
            <button
              className="rounded bg-blue-700 px-4 py-2 font-medium text-white hover:bg-blue-800 disabled:opacity-60"
              disabled={isCreatingTestCase}
              type="submit"
            >
              {isCreatingTestCase ? 'Menambahkan...' : 'Tambah test case'}
            </button>
          </form>
        </section>
      )}
    </main>
  )
}

export default LecturerAssignmentsPage
