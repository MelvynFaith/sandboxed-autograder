import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Link } from 'react-router-dom'
import AuthShell from '../components/AuthShell.jsx'
import { useAuth } from '../context/useAuth.js'

function LoginPage() {
  const { user, login, sessionNotice } = useAuth()
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const isLoggingIn = useRef(false)

  useEffect(() => {
    if (user && !isLoggingIn.current) {
      navigate('/assignments', { replace: true })
    }
  }, [navigate, user])

  async function handleSubmit(event) {
    event.preventDefault()
    setError('')
    setIsSubmitting(true)
    isLoggingIn.current = true

    try {
      const authenticatedUser = await login(email, password)
      const destination = authenticatedUser.role === 'admin' ? '/admin' : '/assignments'
      navigate(destination, { replace: true })
    } catch (loginError) {
      isLoggingIn.current = false
      setError(loginError.message)
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <AuthShell subtitle="Masuk dengan email dan password akun Anda." title="Login">
      {import.meta.env.VITE_USE_MOCK === 'true' && (
        <p className="mt-4 rounded-md bg-accent-tint p-3 text-sm text-link">
          Mode mock: gunakan mahasiswa@example.test, dosen@example.test, atau admin@example.test
          dengan password password123.
        </p>
      )}
      {sessionNotice && (
        <p className="mt-4 text-sm text-attention" role="status">
          {sessionNotice}
        </p>
      )}
      <form className="mt-6 space-y-4 rounded-lg border border-line bg-surface p-5" onSubmit={handleSubmit}>
        <div>
          <label className="mb-1 block font-medium" htmlFor="email">
            Email
          </label>
          <input
            autoComplete="email"
            className="w-full rounded-md border border-line-strong bg-surface px-3 py-2"
            id="email"
            name="email"
            onChange={(event) => setEmail(event.target.value)}
            required
            type="email"
            value={email}
          />
        </div>
        <div>
          <label className="mb-1 block font-medium" htmlFor="password">
            Password
          </label>
          <input
            autoComplete="current-password"
            className="w-full rounded-md border border-line-strong bg-surface px-3 py-2"
            id="password"
            name="password"
            onChange={(event) => setPassword(event.target.value)}
            required
            type="password"
            value={password}
          />
        </div>
        {error && (
          <p className="text-sm text-danger" role="alert">
            {error}
          </p>
        )}
        <button
          className="w-full rounded-md bg-accent px-4 py-2 font-medium text-white hover:bg-accent-hover disabled:opacity-60"
          disabled={isSubmitting}
          type="submit"
        >
          {isSubmitting ? 'Memproses...' : 'Masuk'}
        </button>
      </form>
      <p className="mt-4 rounded-lg border border-line p-4 text-center text-sm text-ink">
        Belum punya akun? <Link className="font-medium text-link underline" to="/register">Register</Link>
      </p>
    </AuthShell>
  )
}

export default LoginPage
