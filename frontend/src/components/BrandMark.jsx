// The product mark: three test cells, two passed and one still open. Drawn in the theme colours so
// it follows light and dark.
function BrandMark({ className = 'h-6 w-6' }) {
  return (
    <svg aria-hidden="true" className={className} fill="none" viewBox="0 0 24 24">
      <rect fill="var(--accent)" height="7" rx="1.75" width="7" x="1" y="8.5" />
      <rect fill="var(--accent)" height="7" rx="1.75" width="7" x="8.5" y="8.5" />
      <rect height="6" rx="1.25" stroke="var(--line-strong)" strokeWidth="1.5" width="6" x="16.5" y="9" />
    </svg>
  )
}

export default BrandMark
