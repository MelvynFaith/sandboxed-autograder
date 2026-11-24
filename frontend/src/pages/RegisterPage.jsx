import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { register } from '../api/auth.js'
import AuthShell from '../components/AuthShell.jsx'

function RegisterPage() {
  const navigate = useNavigate()
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [passwordConfirmation, setPasswordConfirmation] = useState('')
  const [error, setError] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)

  async function handleSubmit(event) {
    event.preventDefault()
    setError('')
    if (password !== passwordConfirmation) {
      setError('Konfirmasi kata sandi tidak cocok.')
      return
    }
    setIsSubmitting(true)
    try {
      await register(name, email, password)
      navigate('/login', { replace: true })
    } catch (registerError) {
      setError(registerError.message)
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <AuthShell subtitle="Buat akun mahasiswa baru." title="Registrasi">
      <form className="mt-6 space-y-4 rounded-lg border border-line bg-surface p-5" onSubmit={handleSubmit}>
        <div>
          <label className="mb-1 block font-medium" htmlFor="register-name">Nama</label>
          <input
            autoComplete="name"
            className="w-full rounded-md border border-line-strong bg-surface px-3 py-2"
            id="register-name"
            name="name"
            onChange={(event) => setName(event.target.value)}
            required
            type="text"
            value={name}
          />
        </div>
        <div>
          <label className="mb-1 block font-medium" htmlFor="register-email">Email</label>
          <input
            autoComplete="email"
            className="w-full rounded-md border border-line-strong bg-surface px-3 py-2"
            id="register-email"
            name="email"
            onChange={(event) => setEmail(event.target.value)}
            required
            type="email"
            value={email}
          />
        </div>
        <div>
          <label className="mb-1 block font-medium" htmlFor="register-password">Password</label>
          <input
            autoComplete="new-password"
            className="w-full rounded-md border border-line-strong bg-surface px-3 py-2"
            id="register-password"
            name="password"
            onChange={(event) => setPassword(event.target.value)}
            required
            type="password"
            value={password}
          />
        </div>
        <div>
          <label className="mb-1 block font-medium" htmlFor="register-password-confirmation">Konfirmasi password</label>
          <input
            autoComplete="new-password"
            className="w-full rounded-md border border-line-strong bg-surface px-3 py-2"
            id="register-password-confirmation"
            name="password-confirmation"
            onChange={(event) => setPasswordConfirmation(event.target.value)}
            required
            type="password"
            value={passwordConfirmation}
          />
        </div>
        {error && <p className="text-sm text-danger" role="alert">{error}</p>}
        <button
          className="w-full rounded-md bg-accent px-4 py-2 font-medium text-white hover:bg-accent-hover disabled:opacity-60"
          disabled={isSubmitting}
          type="submit"
        >
          {isSubmitting ? 'Memproses...' : 'Daftar'}
        </button>
      </form>
      <p className="mt-4 rounded-lg border border-line p-4 text-center text-sm text-ink">
        Sudah punya akun? <Link className="font-medium text-link underline" to="/login">Login</Link>
      </p>
    </AuthShell>
  )
}

export default RegisterPage
