import { errorHint, verdictExplanations } from './reportText.js'

function OutputBox({ empty, heading, Heading, text }) {
  return (
    <div className="min-w-0">
      <Heading className="font-medium">{heading}</Heading>
      <pre className="mt-1 max-h-64 overflow-auto rounded border border-slate-200 bg-white p-3 text-slate-800">
        {text === '' ? <span className="italic text-slate-500">{empty}</span> : text}
      </pre>
    </div>
  )
}

// The details of one visible result: the reason it failed, then its input, the expected output
// and what the program printed, side by side (FR-10). Callers render it only for visible results;
// the report adapter drops the content of hidden ones (FR-03).
function ReportOutputs({ headingLevel = 5, result }) {
  const Heading = `h${headingLevel}`
  const message = result.errorMessage || verdictExplanations[result.verdict]
  const hint = errorHint(result.errorMessage)
  const boxes = [
    result.input != null && { heading: 'Input', empty: '(tanpa input)', text: result.input },
    result.expectedOutput != null && {
      heading: 'Output yang diharapkan',
      empty: '(kosong)',
      text: result.expectedOutput,
    },
    {
      heading: 'Output program',
      empty: 'Program tidak mencetak apa pun.',
      text: result.actualOutput ?? '',
    },
  ].filter(Boolean)

  return (
    <div className="mt-3 space-y-3 text-sm">
      {message && (
        <div className="text-red-800">
          <p>
            <span className="font-medium">Pesan error: </span>
            {message}
          </p>
          {hint && <p className="mt-1 text-slate-700">{hint}</p>}
        </div>
      )}
      <div className={`grid gap-3 ${boxes.length === 3 ? 'lg:grid-cols-3' : 'lg:grid-cols-2'}`}>
        {boxes.map((box) => (
          <OutputBox Heading={Heading} key={box.heading} {...box} />
        ))}
      </div>
      {result.truncated && (
        <p className="text-amber-900">
          Output program dipotong karena melebihi batas ukuran keluaran.
        </p>
      )}
    </div>
  )
}

export default ReportOutputs
