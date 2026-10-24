import { useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { getAssignments } from '../api/assignments.js'
import { getCourses } from '../api/courses.js'
import {
  createLecturerAssignment,
  deleteLecturerAssignment,
  getResourceLimit,
  updateLecturerAssignment,
  updateResourceLimit,
} from '../api/lecturerAssignments.js'
import {
  createTestCase,
  deleteTestCase,
  exportTestCasesYaml,
  getTestCases,
  importTestCasesYaml,
  updateTestCase,
} from '../api/testcases.js'

const resourceLimitFields = [
  { name: 'cpu_time_ms', label: 'Batas waktu CPU (ms)' },
  { name: 'memory_mb', label: 'Batas memori (MB)' },
  { name: 'max_processes', label: 'Jumlah proses maksimum' },
  { name: 'timeout_s', label: 'Waktu maksimum (detik)' },
  { name: 'max_output_bytes', label: 'Output maksimum (byte)' },
  { name: 'tmpfs_size_mb', label: 'Ukuran tmpfs (MB)' },
]

const emptyLimits = Object.fromEntries(resourceLimitFields.map(({ name }) => [name, '']))
const emptyAssignment = { title: '', description: '', deadline_at: '', late_until_at: '' }
const matchModes = ['EXACT', 'WHITESPACE_INSENSITIVE']
const MAX_YAML_SIZE = 1024 * 1024
const emptyTestCase = {
  external_id: '',
  input: '',
  expected_output: '',
  weight: '',
  is_hidden: false,
  match_mode: 'EXACT',
}

function toLocalDateTime(value) {
  if (!value) {
    return ''
  }
  const date = new Date(value)
  return new Date(date.getTime() - date.getTimezoneOffset() * 60_000)
    .toISOString()
    .slice(0, 16)
}

function validateResourceLimits(limits) {
  if (Object.values(limits).some(
    (value) => !Number.isInteger(value) || value <= 0 || value > 2_147_483_647,
  )) {
    return 'Semua batas sumber daya harus berupa bilangan bulat positif.'
  }
  if (limits.tmpfs_size_mb + 16 > limits.memory_mb) {
    return 'Ukuran tmpfs ditambah 16 MB tidak boleh melebihi batas memori.'
  }
  if (2 * limits.max_output_bytes >= limits.tmpfs_size_mb * 1024 ** 2) {
    return 'Dua kali batas output harus lebih kecil dari ukuran tmpfs dalam byte.'
  }
  return null
}

function LecturerAssignmentsPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const requestedCourseId = searchParams.get('courseId')
  const [courses, setCourses] = useState([])
  const [courseId, setCourseId] = useState(() => requestedCourseId ?? '')
  const [assignments, setAssignments] = useState([])
  const [activeAssignment, setActiveAssignment] = useState(null)
  const [testCases, setTestCases] = useState([])
  const [editingTestCaseId, setEditingTestCaseId] = useState(null)
  const [assignmentForm, setAssignmentForm] = useState(emptyAssignment)
  const [resourceLimits, setResourceLimits] = useState(emptyLimits)
  const [testCaseForm, setTestCaseForm] = useState(emptyTestCase)
  const [isEditingAssignment, setIsEditingAssignment] = useState(false)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [pendingDelete, setPendingDelete] = useState(null)
  const [yamlFile, setYamlFile] = useState(null)
  const [isImportingYaml, setIsImportingYaml] = useState(false)
  const [isExportingYaml, setIsExportingYaml] = useState(false)
  const yamlFileInput = useRef(null)
  const confirmDeleteButton = useRef(null)
  const deleteTrigger = useRef(null)

  useEffect(() => {
    if (!pendingDelete) {
      return undefined
    }
    confirmDeleteButton.current?.focus()

    function handleDialogKeyDown(event) {
      if (event.key === 'Escape') {
        event.preventDefault()
        setPendingDelete(null)
        deleteTrigger.current?.focus()
      }
    }

    window.addEventListener('keydown', handleDialogKeyDown)
    return () => window.removeEventListener('keydown', handleDialogKeyDown)
  }, [pendingDelete])

  useEffect(() => {
    let current = true
    getCourses()
      .then((results) => {
        if (current) {
          setCourses(results)
          const requestedCourse = results.find(
            (course) => String(course.id) === requestedCourseId,
          )
          setCourseId(String(requestedCourse?.id ?? results[0]?.id ?? ''))
        }
      })
      .catch((loadError) => {
        if (current) {
          setError(loadError.message)
        }
      })
      .finally(() => {
        if (current) {
          setIsLoading(false)
        }
      })
    return () => {
      current = false
    }
  }, [requestedCourseId])

  useEffect(() => {
    let current = true
    if (!courseId) {
      return undefined
    }
    getAssignments({ courseId })
      .then((results) => {
        if (current) {
          setAssignments(results)
        }
      })
      .catch((loadError) => {
        if (current) {
          setError(loadError.message)
        }
      })
    return () => {
      current = false
    }
  }, [courseId])

  async function selectAssignment(assignment) {
    setActiveAssignment(assignment)
    setIsEditingAssignment(false)
    setAssignmentForm({
      title: assignment.title,
      description: assignment.description,
      deadline_at: toLocalDateTime(assignment.deadlineAt),
      late_until_at: toLocalDateTime(assignment.lateUntilAt),
    })
    setError('')
    try {
      const [results, limits] = await Promise.all([
        getTestCases(assignment.id),
        getResourceLimit(assignment.id),
      ])
      setTestCases(results)
      setResourceLimits(
        Object.fromEntries(resourceLimitFields.map(({ name }) => [name, String(limits[name])])),
      )
    } catch (loadError) {
      setError(loadError.message)
      setTestCases([])
    }
  }

  function handleAssignmentFormChange(event) {
    setAssignmentForm((current) => ({ ...current, [event.target.name]: event.target.value }))
  }

  async function saveAssignment(event) {
    event.preventDefault()
    setError('')
    setSuccess('')
    const limits = Object.fromEntries(
      Object.entries(resourceLimits).map(([name, value]) => [name, Number(value)]),
    )
    const resourceLimitError = validateResourceLimits(limits)
    if (resourceLimitError) {
      setError(resourceLimitError)
      return
    }

    const deadlineValid = Number.isFinite(Date.parse(assignmentForm.deadline_at))
    const lateUntilValid = Number.isFinite(Date.parse(assignmentForm.late_until_at))
    if (
      !assignmentForm.title.trim() ||
      !assignmentForm.description.trim() ||
      !deadlineValid ||
      !lateUntilValid
    ) {
      setError('Judul, deskripsi, deadline, dan batas akhir keterlambatan wajib diisi.')
      return
    }
    if (Date.parse(assignmentForm.late_until_at) < Date.parse(assignmentForm.deadline_at)) {
      setError('Batas akhir keterlambatan harus sama dengan atau setelah deadline.')
      return
    }
    const assignmentPayload = {
      title: assignmentForm.title.trim(),
      description: assignmentForm.description.trim(),
      deadline_at: new Date(assignmentForm.deadline_at).toISOString(),
      late_until_at: new Date(assignmentForm.late_until_at).toISOString(),
    }

    try {
      // A new assignment is created together with its limits; an existing one updates them
      // through the resource-limit endpoint after its own fields are saved.
      const savedAssignment = isEditingAssignment
        ? await updateLecturerAssignment(activeAssignment.id, assignmentPayload)
        : await createLecturerAssignment(courseId, assignmentPayload, limits)
      setAssignments((current) => {
        const remaining = current.filter((entry) => entry.id !== savedAssignment.id)
        return [...remaining, savedAssignment]
      })
      setActiveAssignment(savedAssignment)
      if (isEditingAssignment) {
        await updateResourceLimit(savedAssignment.id, limits)
      }
      setIsEditingAssignment(false)
      setSuccess(isEditingAssignment ? 'Assignment berhasil diperbarui.' : 'Assignment berhasil dibuat.')
      if (!isEditingAssignment) {
        setTestCases([])
        setTestCaseForm(emptyTestCase)
      }
    } catch (saveError) {
      setError(saveError.message)
    }
  }

  async function confirmDeleteAssignment() {
    if (!pendingDelete || pendingDelete.type !== 'assignment') {
      return
    }
    try {
      await deleteLecturerAssignment(pendingDelete.id)
      setAssignments((current) => current.filter((entry) => entry.id !== pendingDelete.id))
      if (activeAssignment?.id === pendingDelete.id) {
        setActiveAssignment(null)
        setTestCases([])
      }
      setSuccess('Assignment berhasil dihapus.')
    } catch (deleteError) {
      setError(deleteError.message)
    } finally {
      setPendingDelete(null)
    }
  }

  async function saveTestCase(event) {
    event.preventDefault()
    setError('')
    setSuccess('')
    const weight = Number(testCaseForm.weight)
    if (!Number.isInteger(weight) || weight <= 0 || weight > 2_147_483_647) {
      setError('Bobot harus berupa bilangan bulat positif.')
      return
    }
    const payload = { ...testCaseForm, external_id: testCaseForm.external_id.trim(), weight }
    try {
      const savedTestCase = editingTestCaseId
        ? await updateTestCase(editingTestCaseId, payload)
        : await createTestCase(activeAssignment.id, payload)
      setTestCases((current) => {
        const remaining = current.filter((entry) => entry.id !== savedTestCase.id)
        return [...remaining, savedTestCase]
      })
      setEditingTestCaseId(null)
      setTestCaseForm(emptyTestCase)
      setSuccess(editingTestCaseId ? 'Test case berhasil diperbarui.' : 'Test case berhasil ditambahkan.')
    } catch (saveError) {
      setError(saveError.message)
    }
  }

  async function exportTestCases() {
    setError('')
    setSuccess('')
    setIsExportingYaml(true)
    try {
      const { blob, filename } = await exportTestCasesYaml(activeAssignment.id)
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = filename
      document.body.appendChild(link)
      link.click()
      link.remove()
      window.setTimeout(() => URL.revokeObjectURL(url), 0)
      setSuccess('Test case berhasil diekspor sebagai YAML.')
    } catch (exportError) {
      setError(exportError.message)
    } finally {
      setIsExportingYaml(false)
    }
  }

  async function importTestCases(event) {
    event.preventDefault()
    if (!yamlFile || !activeAssignment) return

    setError('')
    setSuccess('')
    setIsImportingYaml(true)
    try {
      const result = await importTestCasesYaml(activeAssignment.id, await yamlFile.text())
      setYamlFile(null)
      if (yamlFileInput.current) {
        yamlFileInput.current.value = ''
      }
      setSuccess(`Impor YAML selesai: ${result.created} dibuat, ${result.updated} diperbarui.`)
      try {
        const [updatedTestCases, updatedLimits] = await Promise.all([
          getTestCases(activeAssignment.id),
          getResourceLimit(activeAssignment.id),
        ])
        setTestCases(updatedTestCases)
        setResourceLimits(
          Object.fromEntries(
            resourceLimitFields.map(({ name }) => [name, String(updatedLimits[name])]),
          ),
        )
      } catch (refreshError) {
        setError(`YAML berhasil diimpor, tetapi data terbaru gagal dimuat: ${refreshError.message}`)
      }
    } catch (importError) {
      setError(importError.message)
    } finally {
      setIsImportingYaml(false)
    }
  }

  async function confirmDeleteTestCase() {
    if (!pendingDelete || pendingDelete.type !== 'testcase') {
      return
    }

    try {
      await deleteTestCase(pendingDelete.id)
      setTestCases((current) => current.filter((entry) => entry.id !== pendingDelete.id))
      setSuccess('Test case berhasil dihapus.')
    } catch (deleteError) {
      setError(deleteError.message)
    } finally {
      setPendingDelete(null)
    }
  }

  function requestDelete(type, id, event) {
    deleteTrigger.current = event.currentTarget
    setPendingDelete({ type, id })
  }

  function cancelDelete() {
    setPendingDelete(null)
    deleteTrigger.current?.focus()
  }

  function editTestCase(testCase) {
    setEditingTestCaseId(testCase.id)
    setTestCaseForm({
      external_id: testCase.externalId,
      input: testCase.input,
      expected_output: testCase.expectedOutput,
      weight: String(testCase.weight),
      is_hidden: testCase.hidden,
      match_mode: testCase.matchMode,
    })
  }

  return (
    <main className="mx-auto max-w-5xl space-y-8 px-6 py-10">
      <h1 className="text-2xl font-semibold">Kelola Assignment</h1>
      <nav aria-label="Alur dosen" className="rounded-lg border border-slate-200 bg-white p-4">
        <ol className="flex flex-wrap gap-x-6 gap-y-2 text-sm">
          <li>
            <Link
              className="font-medium text-blue-700 underline"
              to={`/courses${courseId ? `?courseId=${encodeURIComponent(courseId)}` : ''}`}
            >
              1. Course &amp; enrollment
            </Link>
          </li>
          <li aria-current="step" className="font-semibold text-blue-800">
            2. Assignment &amp; test case
          </li>
          <li className="text-slate-600">3. Rekap nilai &amp; CSV (pilih assignment)</li>
        </ol>
      </nav>
      {error && <p className="text-red-700" role="alert">{error}</p>}
      {success && <p className="text-blue-800" role="status">{success}</p>}
      {isLoading ? (
        <p className="text-slate-600">Memuat Course...</p>
      ) : courses.length === 0 ? (
        <p className="text-slate-600">Belum ada Course yang dapat dikelola.</p>
      ) : (
        <>
          <div>
            <label className="mb-1 block font-medium" htmlFor="course-select">Course</label>
            <select
              className="w-full rounded border border-slate-300 p-2"
              id="course-select"
              onChange={(event) => {
                setCourseId(event.target.value)
                setSearchParams({ courseId: event.target.value })
                setActiveAssignment(null)
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

          <section aria-labelledby="assignments-title" className="space-y-3">
            <h2 className="text-xl font-semibold" id="assignments-title">Assignment pada Course</h2>
            <ul className="space-y-3">
              {assignments.map((assignment) => (
                <li
                  className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-slate-200 bg-white p-4"
                  key={assignment.id}
                >
                  <span className="font-medium">{assignment.title}</span>
                  <div className="flex flex-wrap gap-2">
                    <button
                      className="rounded border border-slate-300 px-3 py-2"
                      onClick={() => selectAssignment(assignment)}
                      type="button"
                    >
                      Kelola
                    </button>
                    <Link
                      className="rounded border border-slate-300 px-3 py-2 font-medium text-blue-700 hover:bg-slate-100"
                      to={`/lecturer/assignments/${encodeURIComponent(assignment.id)}/recap?courseId=${encodeURIComponent(courseId)}`}
                    >
                      Rekap nilai &amp; CSV
                    </Link>
                    <button
                      className="rounded border border-red-300 px-3 py-2 text-red-800"
                      onClick={(event) => requestDelete('assignment', assignment.id, event)}
                      type="button"
                    >
                      Hapus
                    </button>
                  </div>
                </li>
              ))}
            </ul>
          </section>

          <form
            className="space-y-5 rounded-lg border border-slate-200 bg-white p-5 shadow-sm"
            onSubmit={saveAssignment}
          >
            <h2 className="text-lg font-semibold">
              {isEditingAssignment ? 'Ubah assignment' : 'Buat assignment'}
            </h2>
            <label className="block">
              <span className="mb-1 block font-medium">Judul</span>
              <input
                className="w-full rounded border border-slate-300 p-2"
                name="title"
                onChange={handleAssignmentFormChange}
                required
                value={assignmentForm.title}
              />
            </label>
            <label className="block">
              <span className="mb-1 block font-medium">Deskripsi</span>
              <textarea
                className="w-full rounded border border-slate-300 p-2"
                name="description"
                onChange={handleAssignmentFormChange}
                required
                rows="3"
                value={assignmentForm.description}
              />
            </label>
            <div className="grid gap-4 sm:grid-cols-2">
              {[
                { name: 'deadline_at', label: 'Deadline' },
                { name: 'late_until_at', label: 'Batas akhir keterlambatan' },
              ].map(({ name, label }) => (
                <label className="block" key={name}>
                  <span className="mb-1 block font-medium">{label}</span>
                  <input
                    className="w-full rounded border border-slate-300 p-2"
                    name={name}
                    onChange={handleAssignmentFormChange}
                    required
                    type="datetime-local"
                    value={assignmentForm[name]}
                  />
                </label>
              ))}
            </div>
            <fieldset className="grid gap-4 sm:grid-cols-2">
              <legend className="mb-3 font-semibold">Batas sumber daya</legend>
              {resourceLimitFields.map(({ name, label }) => (
                <label className="block" key={name}>
                  <span className="mb-1 block font-medium">{label}</span>
                  <input
                    className="w-full rounded border border-slate-300 p-2"
                    min="1"
                    name={name}
                    onChange={(event) =>
                      setResourceLimits((current) => ({ ...current, [name]: event.target.value }))
                    }
                    required
                    step="1"
                    type="number"
                    value={resourceLimits[name]}
                  />
                </label>
              ))}
            </fieldset>
            <div className="flex flex-wrap gap-3">
              <button className="rounded bg-blue-700 px-4 py-2 font-medium text-white" type="submit">
                {isEditingAssignment ? 'Simpan perubahan' : 'Buat assignment'}
              </button>
              {activeAssignment && !isEditingAssignment && (
                <button
                  className="rounded border border-slate-300 px-4 py-2"
                  onClick={() => setIsEditingAssignment(true)}
                  type="button"
                >
                  Ubah assignment terpilih
                </button>
              )}
            </div>
          </form>

          {activeAssignment && (
            <section aria-labelledby="testcases-title" className="space-y-5">
              <h2 className="text-xl font-semibold" id="testcases-title">
                Test case — {activeAssignment.title}
              </h2>
              <div className="flex flex-wrap gap-3">
                <button
                  className="rounded border border-slate-300 px-4 py-2 font-medium disabled:opacity-60"
                  disabled={isExportingYaml}
                  onClick={exportTestCases}
                  type="button"
                >
                  {isExportingYaml ? 'Mengekspor YAML...' : 'Ekspor YAML'}
                </button>
              </div>
              <form
                className="space-y-3 rounded-lg border border-slate-200 bg-white p-5"
                onSubmit={importTestCases}
              >
                <h3 className="text-lg font-semibold">Impor test case dari YAML</h3>
                <label className="block">
                  <span className="mb-1 block font-medium">Berkas YAML (maksimal 1 MiB)</span>
                  <input
                    accept=".yaml,.yml,application/yaml,text/yaml"
                    className="block w-full min-w-0 rounded border border-slate-300 p-2"
                    onChange={(event) => {
                      const selectedFile = event.target.files?.[0] ?? null
                      setError('')
                      setSuccess('')
                      if (selectedFile && selectedFile.size > MAX_YAML_SIZE) {
                        event.target.value = ''
                        setYamlFile(null)
                        setError('Berkas YAML maksimal 1 MiB.')
                        return
                      }
                      setYamlFile(selectedFile)
                    }}
                    ref={yamlFileInput}
                    type="file"
                  />
                </label>
                {yamlFile && (
                  <p className="text-sm text-slate-600">Dipilih: {yamlFile.name}</p>
                )}
                <button
                  className="rounded bg-blue-700 px-4 py-2 font-medium text-white disabled:opacity-60"
                  disabled={!yamlFile || isImportingYaml}
                  type="submit"
                >
                  {isImportingYaml ? 'Mengimpor YAML...' : 'Impor YAML'}
                </button>
              </form>
              <ul className="space-y-3">
                {testCases.map((testCase) => (
                  <li className="rounded-lg border border-slate-200 bg-white p-4" key={testCase.id}>
                    <div className="flex flex-wrap items-start justify-between gap-3">
                      <div>
                        <h3 className="font-semibold">{testCase.externalId}</h3>
                        <p className="text-sm">Bobot: {testCase.weight}; mode: {testCase.matchMode}</p>
                        <p className="text-sm">{testCase.hidden ? 'Tersembunyi' : 'Terlihat'}</p>
                      </div>
                      <div className="flex gap-2">
                        <button
                          className="rounded border border-slate-300 px-3 py-2"
                          onClick={() => editTestCase(testCase)}
                          type="button"
                        >
                          Ubah
                        </button>
                        <button
                          className="rounded border border-red-300 px-3 py-2 text-red-800"
                          onClick={(event) => requestDelete('testcase', testCase.id, event)}
                          type="button"
                        >
                          Hapus
                        </button>
                      </div>
                    </div>
                  </li>
                ))}
              </ul>
              <form className="space-y-4 rounded-lg border border-slate-200 bg-white p-5" onSubmit={saveTestCase}>
                <h3 className="text-lg font-semibold">
                  {editingTestCaseId ? 'Ubah test case' : 'Tambah test case'}
                </h3>
                {[
                  { name: 'external_id', label: 'ID eksternal (opsional)' },
                  { name: 'input', label: 'Input', multiline: true },
                  { name: 'expected_output', label: 'Keluaran yang diharapkan', multiline: true },
                  { name: 'weight', label: 'Bobot', type: 'number' },
                ].map(({ name, label, multiline, type }) => (
                  <label className="block" key={name}>
                    <span className="mb-1 block font-medium">{label}</span>
                    {multiline ? (
                      <textarea
                        className="w-full rounded border border-slate-300 p-2"
                        name={name}
                        onChange={(event) =>
                          setTestCaseForm((current) => ({ ...current, [name]: event.target.value }))
                        }
                        rows="3"
                        value={testCaseForm[name]}
                      />
                    ) : (
                      <input
                        className="w-full rounded border border-slate-300 p-2"
                        min={type === 'number' ? '1' : undefined}
                        name={name}
                        onChange={(event) =>
                          setTestCaseForm((current) => ({ ...current, [name]: event.target.value }))
                        }
                        required={name === 'weight'}
                        step={type === 'number' ? '1' : undefined}
                        type={type ?? 'text'}
                        value={testCaseForm[name]}
                      />
                    )}
                  </label>
                ))}
                <label className="block">
                  <span className="mb-1 block font-medium">Mode pencocokan output</span>
                  <select
                    className="w-full rounded border border-slate-300 p-2"
                    onChange={(event) =>
                      setTestCaseForm((current) => ({ ...current, match_mode: event.target.value }))
                    }
                    value={testCaseForm.match_mode}
                  >
                    {matchModes.map((mode) => <option key={mode}>{mode}</option>)}
                  </select>
                </label>
                <label className="flex items-center gap-2">
                  <input
                    checked={testCaseForm.is_hidden}
                    onChange={(event) =>
                      setTestCaseForm((current) => ({ ...current, is_hidden: event.target.checked }))
                    }
                    type="checkbox"
                  />
                  Test case tersembunyi
                </label>
                <div className="flex flex-wrap gap-3">
                  <button className="rounded bg-blue-700 px-4 py-2 font-medium text-white" type="submit">
                    {editingTestCaseId ? 'Simpan test case' : 'Tambah test case'}
                  </button>
                  {editingTestCaseId && (
                    <button
                      className="rounded border border-slate-300 px-4 py-2"
                      onClick={() => {
                        setEditingTestCaseId(null)
                        setTestCaseForm(emptyTestCase)
                      }}
                      type="button"
                    >
                      Batal ubah
                    </button>
                  )}
                </div>
              </form>
            </section>
          )}
        </>
      )}

      {pendingDelete && (
        <div
          aria-labelledby="confirm-delete-title"
          aria-modal="true"
          className="fixed inset-0 z-10 grid place-items-center bg-slate-950/40 p-6"
          role="dialog"
        >
          <div className="w-full max-w-md space-y-4 rounded-lg bg-white p-6">
            <h2 className="text-lg font-semibold" id="confirm-delete-title">
              Konfirmasi penghapusan
            </h2>
            <p>Data yang dihapus tidak dapat dipulihkan.</p>
            <div className="flex gap-3">
              <button
                className="rounded bg-red-700 px-4 py-2 text-white"
                onClick={pendingDelete.type === 'assignment'
                  ? confirmDeleteAssignment
                  : confirmDeleteTestCase}
                type="button"
                ref={confirmDeleteButton}
              >
                Hapus
              </button>
              <button
                className="rounded border border-slate-300 px-4 py-2"
                onClick={cancelDelete}
                type="button"
              >
                Batal
              </button>
            </div>
          </div>
        </div>
      )}
    </main>
  )
}

export default LecturerAssignmentsPage
