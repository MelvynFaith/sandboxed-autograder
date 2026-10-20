import StatusBadge from './StatusBadge.jsx'
import { formatScore, verdictStyles } from './submissionLabels.js'

function resultTitle(result, index) {
  if (result.hidden) {
    return `Test case tersembunyi ${index + 1}`
  }
  return `Test case ${result.externalId ?? result.testCaseId}`
}

// A hidden result shows only status, score, time and memory: the API omits the rest for students
// and the adapter drops it for everyone (FR-03).
function SubmissionReport({ report }) {
  return (
    <section aria-label={`Laporan submission ${report.submissionId}`} className="mt-5 space-y-4">
      <h3 className="text-lg font-semibold">Laporan per test case</h3>
      <p className="text-sm text-slate-700">
        Skor total: {formatScore(report.totalScore, report.maxScore)}
      </p>
      {report.results.length === 0 ? (
        <p className="text-sm text-slate-600">Belum ada hasil test case.</p>
      ) : (
        <ul className="space-y-3">
          {report.results.map((result, index) => (
            <li
              className="rounded-md border border-slate-200 bg-slate-50 p-4"
              key={result.testCaseId ?? `hidden-${index}`}
            >
              <div className="flex flex-wrap items-center justify-between gap-3">
                <h4 className="font-medium">{resultTitle(result, index)}</h4>
                <StatusBadge
                  className={verdictStyles[result.verdict]}
                  status={result.verdict}
                />
              </div>
              <dl className="mt-3 grid gap-3 text-sm sm:grid-cols-3">
                <div>
                  <dt className="text-slate-600">Skor</dt>
                  <dd className="font-medium">{result.score}</dd>
                </div>
                <div>
                  <dt className="text-slate-600">Waktu eksekusi</dt>
                  <dd className="font-medium">{result.executionTimeMs} ms</dd>
                </div>
                <div>
                  <dt className="text-slate-600">Penggunaan memori</dt>
                  <dd className="font-medium">{result.memoryKb} KB</dd>
                </div>
              </dl>
              {!result.hidden && (
                <div className="mt-3 space-y-2 text-sm">
                  {result.errorMessage && (
                    <p className="text-red-800">
                      <span className="font-medium">Pesan error: </span>
                      {result.errorMessage}
                    </p>
                  )}
                  {result.actualOutput !== null && (
                    <div>
                      <h5 className="font-medium">Output aktual</h5>
                      <pre className="mt-1 overflow-x-auto rounded bg-white p-3 text-slate-800">
                        {result.actualOutput}
                      </pre>
                    </div>
                  )}
                </div>
              )}
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}

export default SubmissionReport
