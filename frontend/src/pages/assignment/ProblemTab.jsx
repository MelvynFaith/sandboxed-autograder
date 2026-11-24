import { useEffect, useState } from 'react'
import { Link, useOutletContext } from 'react-router-dom'
import { getTestCases } from '../../api/testcases.js'
import AssignmentDescription from '../../components/AssignmentDescription.jsx'
import CopyButton from '../../components/CopyButton.jsx'
import { describeLimits } from './assignmentInfo.js'

const matchModeNotes = {
  EXACT: 'Keluaran dibandingkan persis, termasuk spasi dan baris baru.',
  WHITESPACE_INSENSITIVE: 'Perbedaan spasi dan baris baru diabaikan saat membandingkan.',
}

function ExampleBox({ label, text, copyLabel }) {
  return (
    <div className="min-w-0">
      <div className="flex items-center justify-between gap-2 border-b border-line bg-inset px-3 py-1.5">
        <h4 className="text-sm font-semibold">{label}</h4>
        <CopyButton label={copyLabel} text={text} />
      </div>
      <pre className="max-h-64 overflow-auto bg-surface p-3 text-sm">
        {text === '' ? <span className="italic text-muted">(kosong)</span> : text}
      </pre>
    </div>
  )
}

// The problem statement: limits, the lecturer's text, then every test case the lecturer left
// visible, as examples (FR-03: hidden ones are never sent to a student).
function ProblemTab() {
  const { assignment, limits, windowState } = useOutletContext()
  const [examples, setExamples] = useState(null)
  const [loadError, setLoadError] = useState('')

  useEffect(() => {
    let isCurrent = true
    getTestCases(assignment.id)
      .then((testCases) => {
        if (isCurrent) {
          setExamples(testCases.filter((testCase) => !testCase.hidden))
        }
      })
      .catch((error) => {
        if (isCurrent) {
          setLoadError(error.message)
        }
      })
    return () => {
      isCurrent = false
    }
  }, [assignment.id])

  return (
    <div className="mt-6 space-y-8">
      <section aria-labelledby="limits-heading">
        <h2 className="sr-only" id="limits-heading">
          Batas eksekusi
        </h2>
        <dl className="grid grid-cols-1 gap-3 rounded-lg border border-line bg-surface p-4 text-sm sm:grid-cols-2 lg:grid-cols-4">
          {describeLimits(limits).map((item) => (
            <div key={item.label}>
              <dt className="text-muted">{item.label}</dt>
              <dd className="font-semibold">{item.value}</dd>
            </div>
          ))}
        </dl>
        <p className="mt-2 text-sm text-muted">
          Bahasa: Python 3, hanya standard library. Input dibaca dari stdin, keluaran ditulis ke
          stdout.
        </p>
      </section>

      <section aria-labelledby="statement-heading">
        <h2 className="text-xl font-semibold" id="statement-heading">
          Deskripsi soal
        </h2>
        <AssignmentDescription text={assignment.description} />
      </section>

      <section aria-labelledby="examples-heading">
        <h2 className="text-xl font-semibold" id="examples-heading">
          Contoh
        </h2>
        {loadError && (
          <p className="mt-3 text-danger" role="alert">
            Gagal memuat contoh: {loadError}
          </p>
        )}
        {!loadError && examples === null && <p className="mt-3 text-muted">Memuat contoh...</p>}
        {examples?.length === 0 && (
          <p className="mt-3 text-muted">Dosen tidak menampilkan contoh untuk soal ini.</p>
        )}
        {examples?.length > 0 && (
          <ol className="mt-3 space-y-4">
            {examples.map((example, index) => (
              <li
                aria-labelledby={`example-${example.id}`}
                className="overflow-hidden rounded-lg border border-line"
                key={example.id}
              >
                <h3
                  className="bg-inset px-3 py-2 text-sm font-semibold"
                  id={`example-${example.id}`}
                >
                  Contoh {index + 1} ({example.externalId})
                </h3>
                <div className="grid divide-y divide-line border-t border-line md:grid-cols-2 md:divide-x md:divide-y-0">
                  <ExampleBox
                    copyLabel={`Salin input contoh ${index + 1}`}
                    label="Input"
                    text={example.input}
                  />
                  <ExampleBox
                    copyLabel={`Salin output contoh ${index + 1}`}
                    label="Output"
                    text={example.expectedOutput}
                  />
                </div>
                <p className="border-t border-line bg-inset px-3 py-2 text-xs text-muted">
                  {matchModeNotes[example.matchMode]}
                </p>
              </li>
            ))}
          </ol>
        )}
      </section>

      {windowState !== 'closed' && (
        <p>
          <Link
            className="inline-block rounded-md bg-accent px-4 py-2 font-medium text-white hover:bg-accent-hover"
            to="submit"
          >
            Kirim kode untuk soal ini
          </Link>
        </p>
      )}
    </div>
  )
}

export default ProblemTab
