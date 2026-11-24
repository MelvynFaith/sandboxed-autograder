import { useEffect, useRef, useState } from 'react'
import { useLocation, useOutletContext } from 'react-router-dom'
import { getResourceLimit } from '../../api/lecturerAssignments.js'
import {
  createTestCase,
  deleteTestCase,
  exportTestCasesYaml,
  getTestCases,
  importTestCasesYaml,
  updateTestCase,
} from '../../api/testcases.js'
import ConfirmDeleteDialog from '../../components/ConfirmDeleteDialog.jsx'
import FormFeedback from '../../components/FormFeedback.jsx'
import { useFeedback } from './useFeedback.js'

const matchModes = ['EXACT', 'WHITESPACE_INSENSITIVE']
const MAX_YAML_SIZE = 1024 * 1024
const emptyTestCase = {
  external_id: '',
  input: '',
  expected_output: '',
  weight: '',
  is_hidden: false,
  match_mode: 'WHITESPACE_INSENSITIVE',
}

function TestCasesTab() {
  const { assignment, onSaved } = useOutletContext()
  const location = useLocation()
  const { error, success, revision, setError, setSuccess } = useFeedback(
    location.state?.notice ?? '',
  )
  const [testCases, setTestCases] = useState([])
  const [editingTestCaseId, setEditingTestCaseId] = useState(null)
  const [testCaseForm, setTestCaseForm] = useState(emptyTestCase)
  const [pendingDelete, setPendingDelete] = useState(null)
  const [yamlFile, setYamlFile] = useState(null)
  const [isImportingYaml, setIsImportingYaml] = useState(false)
  const [isExportingYaml, setIsExportingYaml] = useState(false)
  const yamlFileInput = useRef(null)
  const deleteTrigger = useRef(null)

  useEffect(() => {
    let isCurrent = true
    getTestCases(assignment.id)
      .then((results) => {
        if (isCurrent) setTestCases(results)
      })
      .catch((loadError) => {
        if (isCurrent) setError(loadError.message)
      })
    return () => {
      isCurrent = false
    }
    // setError only sets state; the test cases are loaded once per assignment.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [assignment.id])

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
        : await createTestCase(assignment.id, payload)
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
      const { blob, filename } = await exportTestCasesYaml(assignment.id)
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
    if (!yamlFile || !assignment) return

    setError('')
    setSuccess('')
    setIsImportingYaml(true)
    try {
      const result = await importTestCasesYaml(assignment.id, await yamlFile.text())
      setYamlFile(null)
      if (yamlFileInput.current) {
        yamlFileInput.current.value = ''
      }
      setSuccess(`Impor YAML selesai: ${result.created} dibuat, ${result.updated} diperbarui.`)
      try {
        const [updatedTestCases, updatedLimits] = await Promise.all([
          getTestCases(assignment.id),
          getResourceLimit(assignment.id),
        ])
        setTestCases(updatedTestCases)
        onSaved(assignment, updatedLimits)
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
    if (!pendingDelete) {
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

  function requestDelete(id, event) {
    deleteTrigger.current = event.currentTarget
    event.currentTarget.focus()
    setPendingDelete({ id })
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
    <>
      <FormFeedback announcementKey={revision} error={error} success={success} />
        <section aria-labelledby="testcases-title" className="space-y-5">
          <h2 className="text-xl font-semibold" id="testcases-title">
            Test case — {assignment.title}
          </h2>
          <div className="flex flex-wrap gap-3">
            <button
              className="rounded-md border border-line-strong px-4 py-2 font-medium disabled:opacity-60"
              disabled={isExportingYaml}
              onClick={exportTestCases}
              type="button"
            >
              {isExportingYaml ? 'Mengekspor YAML...' : 'Ekspor YAML'}
            </button>
          </div>
          <form
            className="space-y-3 rounded-lg border border-line bg-surface p-5"
            onSubmit={importTestCases}
          >
            <h3 className="text-lg font-semibold">Impor test case dari YAML</h3>
            <label className="block">
              <span className="mb-1 block font-medium">Berkas YAML (maksimal 1 MiB)</span>
              <input
                accept=".yaml,.yml,application/yaml,text/yaml"
                className="block w-full min-w-0 rounded-md border border-line-strong p-2"
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
              <p className="text-sm text-muted">Dipilih: {yamlFile.name}</p>
            )}
            <button
              className="rounded-md bg-accent px-4 py-2 font-medium text-white disabled:opacity-60"
              disabled={!yamlFile || isImportingYaml}
              type="submit"
            >
              {isImportingYaml ? 'Mengimpor YAML...' : 'Impor YAML'}
            </button>
          </form>
          <ul className="space-y-3">
            {testCases.map((testCase) => (
              <li className="rounded-lg border border-line bg-surface p-4" key={testCase.id}>
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <h3 className="font-semibold">{testCase.externalId}</h3>
                    <p className="text-sm">Bobot: {testCase.weight}; mode: {testCase.matchMode}</p>
                    <p className="text-sm">{testCase.hidden ? 'Tersembunyi' : 'Terlihat'}</p>
                  </div>
                  <div className="flex gap-2">
                    <button
                      className="rounded-md border border-line-strong px-3 py-2"
                      onClick={() => editTestCase(testCase)}
                      type="button"
                    >
                      Ubah
                    </button>
                    <button
                      className="rounded-md border border-danger/50 px-3 py-2 text-danger"
                      onClick={(event) => requestDelete(testCase.id, event)}
                      type="button"
                    >
                      Hapus
                    </button>
                  </div>
                </div>
              </li>
            ))}
          </ul>
          <form className="space-y-4 rounded-lg border border-line bg-surface p-5" onSubmit={saveTestCase}>
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
                    className="w-full rounded-md border border-line-strong p-2"
                    name={name}
                    onChange={(event) =>
                      setTestCaseForm((current) => ({ ...current, [name]: event.target.value }))
                    }
                    rows="3"
                    value={testCaseForm[name]}
                  />
                ) : (
                  <input
                    className="w-full rounded-md border border-line-strong p-2"
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
            <div>
              <label className="mb-1 block font-medium" htmlFor="test-case-match-mode">
                Mode pencocokan output
              </label>
              <select
                aria-describedby="match-mode-help"
                className="w-full rounded-md border border-line-strong p-2"
                id="test-case-match-mode"
                onChange={(event) =>
                  setTestCaseForm((current) => ({ ...current, match_mode: event.target.value }))
                }
                value={testCaseForm.match_mode}
              >
                {matchModes.map((mode) => <option key={mode}>{mode}</option>)}
              </select>
              <p className="mt-1 text-sm text-muted" id="match-mode-help">
                WHITESPACE_INSENSITIVE (bawaan) mengabaikan perbedaan spasi, tab, dan baris
                baru, sehingga baris baru di akhir keluaran print() tidak menggagalkan jawaban
                yang benar. EXACT membandingkan seluruh keluaran, termasuk spasi dan baris
                baru; pilih EXACT bila keduanya harus ikut dinilai, dan sertakan baris baru di
                keluaran yang diharapkan.
              </p>
            </div>
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
              <button className="rounded-md bg-accent px-4 py-2 font-medium text-white" type="submit">
                {editingTestCaseId ? 'Simpan test case' : 'Tambah test case'}
              </button>
              {editingTestCaseId && (
                <button
                  className="rounded-md border border-line-strong px-4 py-2"
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
      {pendingDelete && (
        <ConfirmDeleteDialog
          onConfirm={confirmDeleteTestCase}
          onDismiss={() => setPendingDelete(null)}
          returnFocusRef={deleteTrigger}
        />
      )}
    </>
  )
}

export default TestCasesTab
