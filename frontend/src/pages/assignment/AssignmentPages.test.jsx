import '@testing-library/jest-dom/vitest'
import { EditorView } from '@codemirror/view'
import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import App from '../../App.jsx'
import { subscribeToSubmissionStatus } from '../../api/submissionStatus.js'
import { submitSubmission } from '../../api/submissions.js'

vi.mock('../../api/submissions.js', async (importOriginal) => ({
  ...(await importOriginal()),
  submitSubmission: vi.fn(),
}))

vi.mock('../../api/submissionStatus.js', () => ({
  subscribeToSubmissionStatus: vi.fn((_submissionId, { onStatus }) => {
    onStatus({ status: 'RUNNING', current_test: 1, total_tests: 3 })
    return vi.fn()
  }),
}))

const testCases = [
  {
    id: 'testcase-1',
    assignment_id: 'assignment-1',
    external_id: 'tc_01',
    input: '3\n',
    expected_output: '9\n',
    weight: 1,
    is_hidden: false,
    match_mode: 'WHITESPACE_INSENSITIVE',
  },
  {
    id: 'testcase-2',
    assignment_id: 'assignment-1',
    external_id: 'tc_02',
    input: 'INPUT-RAHASIA',
    expected_output: 'OUTPUT-RAHASIA',
    weight: 1,
    is_hidden: true,
    match_mode: 'EXACT',
  },
  {
    id: 'testcase-3',
    assignment_id: 'assignment-1',
    external_id: 'tc_03',
    input: '',
    expected_output: '0\n',
    weight: 1,
    is_hidden: false,
    match_mode: 'EXACT',
  },
]

function setStudent() {
  localStorage.setItem('token', 'test-token')
  localStorage.setItem('refresh_token', 'test-refresh-token')
  localStorage.setItem(
    'user',
    JSON.stringify({ id: 'mock-student', nama: 'Mahasiswa', email: 'm@example.test', role: 'mahasiswa' }),
  )
}

function open(path, now = '2026-10-01T12:00:00+07:00') {
  vi.setSystemTime(new Date(now))
  window.history.pushState({}, '', path)
  render(<App />)
}

// The editor is loaded lazily, so wait for it before typing.
async function typeCode(text) {
  const view = EditorView.findFromDOM(
    (await screen.findByTestId('code-editor')).shadowRoot.querySelector('.cm-editor'),
  )
  act(() => view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: text } }))
}

// The pages load lazily in the app; importing them once here keeps a cold module transform on a
// busy machine from running past the default wait of findBy*.
beforeAll(async () => {
  await Promise.all([
    import('./AssignmentLayout.jsx'),
    import('./ProblemTab.jsx'),
    import('./SubmitTab.jsx'),
    import('./MySubmissionsTab.jsx'),
    import('../../components/CodeEditor.jsx'),
    import('../../components/AssignmentDescription.jsx'),
  ])
})

beforeEach(() => {
  localStorage.clear()
  vi.stubEnv('VITE_USE_MOCK', 'true')
  vi.useFakeTimers({ toFake: ['Date'] })
  setStudent()
  localStorage.setItem('sandboxed_autograder_mock_test_cases', JSON.stringify(testCases))
  submitSubmission.mockResolvedValue({
    id: 'submission-9',
    status: 'QUEUED',
    isLate: false,
    submittedAt: '2026-10-01T05:00:00Z',
  })
})

afterEach(() => {
  cleanup()
  localStorage.clear()
  vi.useRealTimers()
  vi.unstubAllEnvs()
  vi.clearAllMocks()
})

describe('problem page', () => {
  it('shows the title, both deadlines, the limits and the statement', async () => {
    open('/assignments/assignment-1')

    expect(await screen.findByRole('heading', { level: 1, name: 'Dasar Algoritma' })).toBeInTheDocument()
    expect(screen.getByText('Dibuka')).toBeInTheDocument()
    expect(screen.getByText('Deadline:').nextElementSibling).toHaveTextContent('12 Oktober 2026')
    expect(screen.getByText('Batas terlambat:').nextElementSibling).toHaveTextContent('13 Oktober 2026')
    expect((await screen.findByText('Batas waktu CPU per test')).nextElementSibling).toHaveTextContent('1 detik')
    expect(screen.getByText('Batas memori').nextElementSibling).toHaveTextContent('128 MB')
    expect(await screen.findByTestId('assignment-description')).toHaveTextContent('Buat program Python')
    expect(
      within(screen.getByRole('navigation', { name: 'Breadcrumb' })).getByRole('link', {
        name: 'Daftar Assignment',
      }),
    ).toHaveAttribute('href', '/assignments')
  })

  it('marks the current view in the assignment navigation', async () => {
    open('/assignments/assignment-1')

    const nav = await screen.findByRole('navigation', { name: 'Bagian assignment' })
    expect(within(nav).getByRole('link', { name: 'Soal' })).toHaveAttribute('aria-current', 'page')
    fireEvent.click(within(nav).getByRole('link', { name: 'Kirim kode' }))
    // The submit view loads lazily; the link is current once the navigation has finished.
    await waitFor(() =>
      expect(within(nav).getByRole('link', { name: 'Kirim kode' })).toHaveAttribute('aria-current', 'page'),
    )
    expect(within(nav).getByRole('link', { name: 'Soal' })).not.toHaveAttribute('aria-current')
    expect(window.location.pathname).toBe('/assignments/assignment-1/submit')
  })

  it('shows every visible test case as an example and never a hidden one', async () => {
    open('/assignments/assignment-1')

    const examples = await screen.findAllByRole('listitem', { name: /^Contoh/ })
    expect(examples).toHaveLength(2)
    expect(within(examples[0]).getByRole('heading', { name: 'Contoh 1 (tc_01)' })).toBeInTheDocument()
    expect(within(examples[0]).getByText('Perbedaan spasi dan baris baru diabaikan saat membandingkan.')).toBeInTheDocument()
    expect(within(examples[0]).getByRole('button', { name: 'Salin input contoh 1' })).toBeInTheDocument()
    expect(within(examples[1]).getByRole('heading', { name: 'Contoh 2 (tc_03)' })).toBeInTheDocument()
    expect(within(examples[1]).getByText('(kosong)')).toBeInTheDocument()
    expect(within(examples[1]).getByText(/termasuk spasi dan baris baru/)).toBeInTheDocument()
    expect(document.body).not.toHaveTextContent('RAHASIA')
  })

  it('says when the lecturer shows no examples', async () => {
    localStorage.setItem('sandboxed_autograder_mock_test_cases', JSON.stringify([testCases[1]]))
    open('/assignments/assignment-1')

    expect(await screen.findByText('Dosen tidak menampilkan contoh untuk soal ini.')).toBeInTheDocument()
  })

  it('shows an unknown or foreign assignment as not found', async () => {
    open('/assignments/assignment-404')

    expect(await screen.findByRole('alert')).toHaveTextContent('Assignment tidak ditemukan.')
  })

  it('keeps lecturers on their own pages', async () => {
    localStorage.setItem('user', JSON.stringify({ id: 'user-1', nama: 'D', email: 'd@example.test', role: 'dosen' }))
    open('/assignments/assignment-1')

    await screen.findByRole('heading', { name: 'Daftar Assignment' })
    expect(window.location.pathname).toBe('/assignments')
  })
})

describe('submit view', () => {
  it('sends code typed in the editor as main.py', async () => {
    open('/assignments/assignment-1/submit')

    expect(await screen.findByLabelText('Bahasa')).toHaveDisplayValue('Python 3 (standard library)')
    await typeCode('n = int(input())\nprint(n * n)\n')
    fireEvent.click(screen.getByRole('button', { name: 'Kirim' }))

    expect(await screen.findByRole('status')).toHaveTextContent('Kode terkirim dan sedang diantrekan untuk dinilai.')
    expect(submitSubmission).toHaveBeenCalledTimes(1)
    const [assignmentId, upload] = submitSubmission.mock.calls[0]
    expect(assignmentId).toBe('assignment-1')
    expect(upload.name).toBe('main.py')
    expect(await upload.text()).toBe('n = int(input())\nprint(n * n)\n')
    expect(screen.getByRole('region', { name: 'Status submission submission-9' })).toBeInTheDocument()
  })

  it('refuses an empty editor and code over 256 KB without calling the API', async () => {
    open('/assignments/assignment-1/submit')
    await screen.findByTestId('code-editor')

    fireEvent.click(screen.getByRole('button', { name: 'Kirim' }))
    expect(screen.getByRole('alert')).toHaveTextContent('Tulis kode Python sebelum mengirim.')

    // 128 Ki two-byte characters: under 256 Ki characters, over 256 KiB as UTF-8.
    await typeCode('é'.repeat(131073))
    fireEvent.click(screen.getByRole('button', { name: 'Kirim' }))
    expect(screen.getByRole('alert')).toHaveTextContent('Ukuran kode maksimal 256 KB.')
    expect(submitSubmission).not.toHaveBeenCalled()
  })

  it('sends a chosen .py file and refuses other files', async () => {
    open('/assignments/assignment-1/submit')
    fireEvent.click(await screen.findByRole('radio', { name: 'Unggah berkas .py' }))
    const input = screen.getByLabelText('Berkas kode Python')

    fireEvent.change(input, { target: { files: [new File(['x'], 'notes.txt')] } })
    fireEvent.click(screen.getByRole('button', { name: 'Kirim' }))
    expect(screen.getByRole('alert')).toHaveTextContent('Pilih berkas dengan ekstensi .py.')

    const file = new File(['print(9)'], 'solusi.py', { type: 'text/x-python' })
    fireEvent.change(input, { target: { files: [file] } })
    fireEvent.click(screen.getByRole('button', { name: 'Kirim' }))

    expect(await screen.findByRole('status')).toHaveTextContent('Kode terkirim')
    expect(submitSubmission).toHaveBeenCalledWith('assignment-1', file)
  })

  it('counts down after a rate-limit refusal', async () => {
    const refusal = new Error('Tunggu setidaknya 30 detik sebelum mengirim submission berikutnya.')
    refusal.retryAfter = 30
    submitSubmission.mockRejectedValueOnce(refusal)
    open('/assignments/assignment-1/submit')
    await screen.findByLabelText('Bahasa')

    await typeCode('print(1)\n')
    fireEvent.click(screen.getByRole('button', { name: 'Kirim' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Tunggu setidaknya 30 detik')
    expect(screen.getByRole('button', { name: 'Kirim lagi dalam 30 detik' })).toBeDisabled()
  })

  it('warns that a submission after the deadline is marked late', async () => {
    open('/assignments/assignment-1/submit', '2026-10-13T12:00:00+07:00')

    expect(await screen.findByText(/Deadline sudah lewat/)).toBeInTheDocument()
    expect(screen.getByText('Masa terlambat')).toBeInTheDocument()
  })

  it('offers no form once the assignment is closed', async () => {
    open('/assignments/assignment-1/submit', '2026-10-14T00:00:01+07:00')

    expect(await screen.findByText('Pengumpulan untuk assignment ini sudah ditutup.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Kirim' })).not.toBeInTheDocument()
    expect(screen.getByText('Ditutup')).toBeInTheDocument()
  })
})

describe('my submissions view', () => {
  it('lists the graded submission and opens its report in place', async () => {
    open('/assignments/assignment-1/submissions')

    const table = await screen.findByRole('table', { name: 'Submission saya untuk Dasar Algoritma' })
    const rows = within(table).getAllByRole('row').slice(1)
    expect(rows).toHaveLength(1)
    expect(within(rows[0]).getByText('Selesai')).toBeInTheDocument()
    expect(within(rows[0]).getByText('85 / 100')).toBeInTheDocument()
    expect(within(rows[0]).getByRole('link', { name: 'Kode' })).toHaveAttribute(
      'href',
      '/submissions/submission-1/code',
    )

    const toggle = within(rows[0]).getByRole('button', { name: 'Laporan' })
    fireEvent.click(toggle)
    expect(toggle).toHaveAttribute('aria-expanded', 'true')
    expect(await screen.findByRole('region', { name: 'Laporan submission submission-1' })).toBeInTheDocument()
    fireEvent.click(toggle)
    expect(screen.queryByRole('region', { name: 'Laporan submission submission-1' })).not.toBeInTheDocument()
  })

  it('shows only this assignment, follows the active submission live and marks late ones', async () => {
    open('/assignments/assignment-2/submissions')

    const table = await screen.findByRole('table', { name: 'Submission saya untuk Struktur Data' })
    const rows = within(table).getAllByRole('row').slice(1)
    expect(rows).toHaveLength(2)
    expect(await within(rows[0]).findByText('Sedang berjalan')).toBeInTheDocument()
    expect(within(rows[0]).getByText('Test 1 dari 3')).toBeInTheDocument()
    expect(within(rows[1]).getByText('Terlambat')).toBeInTheDocument()
    expect(screen.queryByText('85 / 100')).not.toBeInTheDocument()
  })

  it('cancels a queued submission and reloads the list', async () => {
    subscribeToSubmissionStatus.mockImplementation((_submissionId, { onStatus }) => {
      onStatus({ status: 'QUEUED', current_test: 0, total_tests: 0 })
      return vi.fn()
    })
    open('/assignments/assignment-2/submissions')

    const table = await screen.findByRole('table', { name: 'Submission saya untuk Struktur Data' })
    const first = within(table).getAllByRole('row')[1]
    fireEvent.click(within(first).getByRole('button', { name: 'Batalkan' }))

    expect(await screen.findByRole('status')).toHaveTextContent('Submission dibatalkan.')
    expect(await within(table).findByText('Dibatalkan')).toBeInTheDocument()
  })
})
