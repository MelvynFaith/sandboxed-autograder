import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { register } from '../api/auth.js'

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
    <main className="mx-auto max-w-md px-6 py-12">
      <h1 className="text-2xl font-semibold">Registrasi</h1>
      <p className="mt-2 text-slate-600">Buat akun mahasiswa baru.</p>
      <form className="mt-6 space-y-4" onSubmit={handleSubmit}>
        <div>
          <label className="mb-1 block font-medium" htmlFor="register-name">Nama</label>
          <input
            autoComplete="name"
            className="w-full rounded border border-slate-300 bg-white px-3 py-2"
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
            className="w-full rounded border border-slate-300 bg-white px-3 py-2"
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
            className="w-full rounded border border-slate-300 bg-white px-3 py-2"
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
            className="w-full rounded border border-slate-300 bg-white px-3 py-2"
            id="register-password-confirmation"
            name="password-confirmation"
            onChange={(event) => setPasswordConfirmation(event.target.value)}
            required
            type="password"
            value={passwordConfirmation}
          />
        </div>
        {error && <p className="text-sm text-red-700" role="alert">{error}</p>}
        <button
          className="w-full rounded bg-blue-700 px-4 py-2 font-medium text-white hover:bg-blue-800 disabled:opacity-60"
          disabled={isSubmitting}
          type="submit"
        >
          {isSubmitting ? 'Memproses...' : 'Daftar'}
        </button>
      </form>
      <p className="mt-4 text-sm text-slate-700">
        Sudah punya akun? <Link className="font-medium text-blue-700 underline" to="/login">Login</Link>
      </p>
    </main>
  )
}

export default RegisterPage
