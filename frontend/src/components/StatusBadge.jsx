import { verdictStyles } from './submissionLabels.js'

// Text label plus colour, so the status never relies on colour alone (NFR-06, NFR-11).
function StatusBadge({ status, label, className }) {
  return (
    <span
      className={`inline-flex rounded-full px-3 py-1 text-sm font-semibold ${className}`}
      data-testid={verdictStyles[status] ? 'verdict-badge' : undefined}
    >
      {label ?? status}
    </span>
  )
}

export default StatusBadge
