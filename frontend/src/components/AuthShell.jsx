import BrandMark from './BrandMark.jsx'

// The frame of the login and register pages: the product mark, the title, and the form below it.
function AuthShell({ title, subtitle, children }) {
  return (
    <main className="mx-auto flex min-h-screen w-full max-w-sm flex-col justify-center px-4 py-10">
      <div className="flex flex-col items-center text-center">
        <BrandMark className="h-10 w-10" />
        <h1 className="mt-4 text-2xl font-semibold tracking-tight">{title}</h1>
        <p className="mt-1 text-muted">{subtitle}</p>
      </div>
      {children}
    </main>
  )
}

export default AuthShell
