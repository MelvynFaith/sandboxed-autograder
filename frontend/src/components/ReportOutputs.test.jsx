import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import ReportOutputs from './ReportOutputs.jsx'
import { errorHint } from './reportText.js'

afterEach(cleanup)

const visible = {
  verdict: 'FAILED',
  errorMessage: null,
  actualOutput: '10\n',
  input: '3\n',
  expectedOutput: '9\n',
  truncated: false,
}

function box(heading) {
  return screen.getByRole('heading', { name: heading }).nextElementSibling
}

describe('ReportOutputs', () => {
  it('shows the input, the expected output and the program output side by side', () => {
    render(<ReportOutputs result={visible} />)

    expect(box('Input')).toHaveTextContent('3')
    expect(box('Output yang diharapkan')).toHaveTextContent('9')
    expect(box('Output program')).toHaveTextContent('10')
    expect(screen.getByText(/Pesan error:/).parentElement).toHaveTextContent(
      'Keluaran program tidak sesuai dengan yang diharapkan.',
    )
    expect(screen.queryByText(/dipotong/)).not.toBeInTheDocument()
  })

  it('names empty input, empty expected output and a program that printed nothing', () => {
    render(<ReportOutputs result={{ ...visible, input: '', expectedOutput: '', actualOutput: '' }} />)

    expect(box('Input')).toHaveTextContent('(tanpa input)')
    expect(box('Output yang diharapkan')).toHaveTextContent('(kosong)')
    expect(box('Output program')).toHaveTextContent('Program tidak mencetak apa pun.')
  })

  it('shows only the program output when the report carries no test case content', () => {
    render(<ReportOutputs result={{ ...visible, input: null, expectedOutput: null, actualOutput: null }} />)

    expect(screen.queryByRole('heading', { name: 'Input' })).not.toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Output yang diharapkan' })).not.toBeInTheDocument()
    expect(box('Output program')).toHaveTextContent('Program tidak mencetak apa pun.')
  })

  it('says when the program output was cut at the output limit', () => {
    render(<ReportOutputs result={{ ...visible, truncated: true }} />)

    expect(
      screen.getByText('Output program dipotong karena melebihi batas ukuran keluaran.'),
    ).toBeInTheDocument()
  })

  it('explains that only the standard library can be imported', () => {
    render(
      <ReportOutputs
        result={{
          ...visible,
          verdict: 'RUNTIME_ERROR',
          errorMessage: "ModuleNotFoundError: No module named 'cv2'",
          actualOutput: '',
        }}
      />,
    )

    expect(screen.getByText(/Pesan error:/).parentElement).toHaveTextContent(
      "ModuleNotFoundError: No module named 'cv2'",
    )
    expect(screen.getByText(/Hanya standard library Python yang tersedia/)).toBeInTheDocument()
  })

  it('uses the heading level the page asks for', () => {
    render(<ReportOutputs headingLevel={3} result={visible} />)

    expect(screen.getByRole('heading', { level: 3, name: 'Input' })).toBeInTheDocument()
  })
})

describe('errorHint', () => {
  it.each([
    ["ModuleNotFoundError: No module named 'numpy'", true],
    ['ImportError: cannot import name x', true],
    ['ZeroDivisionError: division by zero', false],
    ['NameError: ModuleNotFoundError is not defined', false],
    [null, false],
  ])('%s gives a hint: %s', (message, expected) => {
    expect(errorHint(message) !== null).toBe(expected)
  })
})
