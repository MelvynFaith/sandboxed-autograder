import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import AssignmentDescription from './AssignmentDescription.jsx'

afterEach(cleanup)

describe('AssignmentDescription', () => {
  it('renders Markdown: headings below the page title, lists, code', () => {
    render(
      <AssignmentDescription
        text={'# Masukan\nSatu bilangan `n`.\n\n- baris pertama\n- baris kedua\n\n```\nprint(n * n)\n```'}
      />,
    )

    expect(screen.getByRole('heading', { level: 2, name: 'Masukan' })).toBeInTheDocument()
    expect(screen.getAllByRole('listitem')).toHaveLength(2)
    expect(screen.getByText('n', { selector: 'code' })).toBeInTheDocument()
    expect(screen.getByText('print(n * n)')).toBeInTheDocument()
  })

  it('keeps a single newline of a plain-text statement as a line break', () => {
    render(<AssignmentDescription text={'Baris satu\nBaris dua'} />)

    expect(screen.getByTestId('assignment-description').querySelector('br')).not.toBeNull()
  })

  it('never renders raw HTML, scripts or javascript: links', () => {
    render(
      <AssignmentDescription
        text={
          '<script>alert(1)</script><b onclick="x()">tebal</b>\n\n' +
          '[klik](javascript:alert(1)) [aman](https://example.test/soal)\n\n![gambar](https://example.test/a.png)'
        }
      />,
    )

    const root = screen.getByTestId('assignment-description')
    expect(root.querySelector('script')).toBeNull()
    expect(root.querySelector('b')).toBeNull()
    expect(root.querySelector('img')).toBeNull()
    expect(root.innerHTML).not.toContain('javascript:')
    const safe = screen.getByRole('link', { name: 'aman' })
    expect(safe).toHaveAttribute('href', 'https://example.test/soal')
    expect(safe).toHaveAttribute('rel', 'noopener noreferrer')
  })

  it('says when the lecturer wrote no statement', () => {
    render(<AssignmentDescription text="   " />)

    expect(screen.getByText('Dosen belum menulis deskripsi soal.')).toBeInTheDocument()
  })
})
