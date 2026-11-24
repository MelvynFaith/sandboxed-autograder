import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import ScoreMeter from './ScoreMeter.jsx'

afterEach(cleanup)

describe('ScoreMeter', () => {
  it('fills the bar by the share of the maximum and marks full marks', () => {
    const { rerender } = render(<ScoreMeter max={4} total={1} />)
    expect(screen.getByTestId('score-meter').firstChild).toHaveStyle({ width: '25%' })
    expect(screen.getByTestId('score-meter').firstChild).toHaveClass('bg-accent')
    rerender(<ScoreMeter max={4} total={4} />)
    expect(screen.getByTestId('score-meter').firstChild).toHaveClass('bg-success')
  })

  it('draws a real zero as an empty bar and is hidden from assistive technology', () => {
    render(<ScoreMeter max={3} total={0} />)
    expect(screen.getByTestId('score-meter')).toHaveAttribute('aria-hidden', 'true')
    expect(screen.getByTestId('score-meter').firstChild).toHaveStyle({ width: '0%' })
  })

  it('draws nothing before a submission is graded', () => {
    const { container } = render(<ScoreMeter max={null} total={null} />)
    expect(container).toBeEmptyDOMElement()
  })
})
