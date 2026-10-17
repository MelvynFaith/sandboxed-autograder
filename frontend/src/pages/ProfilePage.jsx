import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { getProfile } from '../api/profile.js'
import { useAuth } from '../context/useAuth.js'

const roleNames = { STUDENT: 'Mahasiswa', LECTURER: 'Dosen', ADMIN: 'Admin' }

function ProfilePage() {
  const { user, updateName, changePassword } = useAuth()
  const navigate = useNavigate()
  const [profile, setProfile] = useState(null)
  const [name, setName] = useState('')
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [isLoading, setIsLoading] = useState(true)
  const [isSavingName, setIsSavingName] = useState(false)
  const [isChangingPassword, setIsChangingPassword] = useState(false)

  useEffect(() => {
    let active = true
    getProfile(user)
      .then((loadedProfile) => {
        if (active) {
          setProfile(loadedProfile)
          setName(loadedProfile.name)
        }
      })
      .catch((loadError) => {
        if (active) setError(loadError.message)
      })
      .finally(() => {
        if (active) setIsLoading(false)
      })
    return () => { active = false }
  }, [user])

  async function handleNameSubmit(event) {
    event.preventDefault()
    setError('')
    setNotice('')
    setIsSavingName(true)
    try {
      const updatedProfile = await updateName(name)
      setProfile(updatedProfile)
      setNotice('Nama berhasil diperbarui.')
    } catch (updateError) {
      setError(updateError.message)
    } finally {
      setIsSavingName(false)
    }
  }

  async function handlePasswordSubmit(event) {
    event.preventDefault()
    setError('')
    setIsChangingPassword(true)
    try {
      await changePassword(currentPassword, newPassword)
      navigate('/login', { replace: true })
    } catch (changeError) {
      setError(changeError.message)
      setIsChangingPassword(false)
    }
  }

  if (isLoading) {
    return <main className="mx-auto max-w-2xl px-6 py-8">Memuat profil...</main>
  }

  return (
    <main className="mx-auto max-w-2xl px-6 py-8">
      <h1 className="text-2xl font-semibold">Profil</h1>
      {error && <p className="mt-4 text-sm text-red-700" role="alert">{error}</p>}
      {notice && <p className="mt-4 text-sm text-green-800" role="status">{notice}</p>}
      {profile && (
        <>
          <dl className="mt-6 grid gap-3 rounded border border-slate-200 bg-white p-4 sm:grid-cols-2">
            <div className="min-w-0"><dt className="font-medium">Nama</dt><dd className="break-all">{profile.name}</dd></div>
            <div className="min-w-0"><dt className="font-medium">Email</dt><dd className="break-all">{profile.email}</dd></div>
            <div className="min-w-0"><dt className="font-medium">Peran</dt><dd>{roleNames[profile.role]}</dd></div>
          </dl>
          <form className="mt-6 space-y-4 rounded border border-slate-200 bg-white p-4" onSubmit={handleNameSubmit}>
            <h2 className="text-lg font-semibold">Ubah nama</h2>
            <div>
              <label className="mb-1 block font-medium" htmlFor="profile-name">Nama tampilan</label>
              <input
                autoComplete="name"
                className="w-full rounded border border-slate-300 bg-white px-3 py-2"
                id="profile-name"
                name="name"
                onChange={(event) => setName(event.target.value)}
                required
                type="text"
                value={name}
              />
            </div>
            <button className="rounded bg-blue-700 px-4 py-2 font-medium text-white disabled:opacity-60" disabled={isSavingName} type="submit">
              {isSavingName ? 'Menyimpan...' : 'Simpan nama'}
            </button>
          </form>
          <form className="mt-6 space-y-4 rounded border border-slate-200 bg-white p-4" onSubmit={handlePasswordSubmit}>
            <h2 className="text-lg font-semibold">Ganti kata sandi</h2>
            <div>
              <label className="mb-1 block font-medium" htmlFor="current-password">Kata sandi lama</label>
              <input
                autoComplete="current-password"
                className="w-full rounded border border-slate-300 bg-white px-3 py-2"
                id="current-password"
                name="current-password"
                onChange={(event) => setCurrentPassword(event.target.value)}
                required
                type="password"
                value={currentPassword}
              />
            </div>
            <div>
              <label className="mb-1 block font-medium" htmlFor="new-password">Kata sandi baru</label>
              <input
                autoComplete="new-password"
                className="w-full rounded border border-slate-300 bg-white px-3 py-2"
                id="new-password"
                name="new-password"
                onChange={(event) => setNewPassword(event.target.value)}
                required
                type="password"
                value={newPassword}
              />
            </div>
            <button className="rounded bg-blue-700 px-4 py-2 font-medium text-white disabled:opacity-60" disabled={isChangingPassword} type="submit">
              {isChangingPassword ? 'Mengganti...' : 'Ganti kata sandi'}
            </button>
          </form>
        </>
      )}
    </main>
  )
}

export default ProfilePage
