import Markdown from 'react-markdown'
import remarkBreaks from 'remark-breaks'

// The problem statement a lecturer wrote, as Markdown. react-markdown builds React elements and
// ignores raw HTML, and its default URL check drops javascript: and other unsafe links, so a
// statement cannot inject script (NFR-10). Images are left out: the CSP only allows this origin.
// A single newline is kept as a line break, so a statement typed as plain text reads as written.
// Headings start one level below the page title.
const components = {
  h1: ({ children }) => <h2 className="mt-5 text-xl font-semibold">{children}</h2>,
  h2: ({ children }) => <h3 className="mt-4 text-lg font-semibold">{children}</h3>,
  h3: ({ children }) => <h4 className="mt-4 font-semibold">{children}</h4>,
  h4: ({ children }) => <h5 className="mt-3 font-semibold">{children}</h5>,
  h5: ({ children }) => <h6 className="mt-3 font-semibold">{children}</h6>,
  h6: ({ children }) => <h6 className="mt-3 font-semibold">{children}</h6>,
  p: ({ children }) => <p className="mt-3 leading-relaxed">{children}</p>,
  ul: ({ children }) => <ul className="mt-3 list-disc space-y-1 pl-6">{children}</ul>,
  ol: ({ children }) => <ol className="mt-3 list-decimal space-y-1 pl-6">{children}</ol>,
  blockquote: ({ children }) => (
    <blockquote className="mt-3 border-l-4 border-line-strong pl-4 text-ink">
      {children}
    </blockquote>
  ),
  pre: ({ children }) => (
    <pre className="mt-3 overflow-x-auto rounded-md border border-line bg-inset p-3 text-sm [&_code]:bg-transparent [&_code]:p-0">
      {children}
    </pre>
  ),
  code: ({ children }) => (
    <code className="rounded-md bg-inset px-1 font-mono text-sm">{children}</code>
  ),
  a: ({ children, href }) => (
    <a className="text-link underline" href={href} rel="noopener noreferrer" target="_blank">
      {children}
    </a>
  ),
  hr: () => <hr className="my-4 border-line" />,
}

function AssignmentDescription({ text }) {
  if (!text.trim()) {
    return <p className="mt-3 text-muted">Dosen belum menulis deskripsi soal.</p>
  }
  return (
    <div className="text-ink" data-testid="assignment-description">
      <Markdown
        components={components}
        disallowedElements={['img']}
        remarkPlugins={[remarkBreaks]}
        skipHtml
        unwrapDisallowed
      >
        {text}
      </Markdown>
    </div>
  )
}

export default AssignmentDescription
