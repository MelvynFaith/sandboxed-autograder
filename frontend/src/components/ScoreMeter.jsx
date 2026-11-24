// A short bar next to a score: how much of the maximum was earned. The score text stays next to it
// (the bar is a second view of the same number, not the only one), so the bar is hidden from
// assistive technology. Full marks use the success colour, anything less the accent.
function ScoreMeter({ total, max }) {
  if (total == null || !max) {
    return null
  }
  const ratio = Math.max(0, Math.min(1, total / max))
  return (
    <span
      aria-hidden="true"
      className="inline-block h-1.5 w-16 shrink-0 overflow-hidden rounded-full bg-raised align-middle"
      data-testid="score-meter"
    >
      <span
        className={`block h-full rounded-full ${ratio === 1 ? 'bg-success' : 'bg-accent'}`}
        style={{ width: `${ratio * 100}%` }}
      />
    </span>
  )
}

export default ScoreMeter
