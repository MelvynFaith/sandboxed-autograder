import { useEffect, useState } from 'react'
import { login as requestLogin, logout as requestLogout } from '../api/auth.js'
import { updatePassword as requestPasswordChange, updateProfileName as requestProfileName } from '../api/profile.js'
import { AUTH_SESSION_INVALIDATED } from '../api/client.js'
import { AuthContext } from './AuthContext.js'

const TOKEN_KEY = 'token'
const REFRESH_TOKEN_KEY = 'refresh_token'
const USER_KEY = 'user'
const roles = ['mahasiswa', 'dosen', 'admin']

function readStoredAuth() {
  const token = localStorage.getItem(TOKEN_KEY)
  const refreshToken = localStorage.getItem(REFRESH_TOKEN_KEY)
  const serializedUser = localStorage.getItem(USER_KEY)

  if (!token || !refreshToken || !serializedUser) {
    return null
  }

  let user
  try {
    user = JSON.parse(serializedUser)
  } catch (error) {
    if (!(error instanceof SyntaxError)) {
      throw error
    }
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(REFRESH_TOKEN_KEY)
    localStorage.removeItem(USER_KEY)
    return null
  }

  if (
    (typeof user?.id !== 'string' && typeof user?.id !== 'number') ||
    typeof user?.nama !== 'string' ||
    typeof user?.email !== 'string' ||
    !roles.includes(user.role)
  ) {
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(REFRESH_TOKEN_KEY)
    localStorage.removeItem(USER_KEY)
    return null
  }

  return { token, refreshToken, user: { ...user, id: String(user.id) } }
}

function AuthProvider({ children }) {
  const [auth, setAuth] = useState(readStoredAuth)
  const [sessionNotice, setSessionNotice] = useState('')

  useEffect(() => {
    function invalidateAuth() {
      setAuth(null)
      setSessionNotice('Sesi berakhir. Silakan masuk kembali.')
    }

    window.addEventListener(AUTH_SESSION_INVALIDATED, invalidateAuth)
    return () => window.removeEventListener(AUTH_SESSION_INVALIDATED, invalidateAuth)
  }, [])

  async function login(email, password) {
    setSessionNotice('')
    const result = await requestLogin(email, password)
    localStorage.setItem(TOKEN_KEY, result.token)
    localStorage.setItem(REFRESH_TOKEN_KEY, result.refreshToken)
    localStorage.setItem(USER_KEY, JSON.stringify(result.user))
    setAuth(result)
    return result.user
  }

  async function logout() {
    try {
      await requestLogout(auth?.refreshToken)
    } catch (error) {
      setSessionNotice(`Sesi lokal sudah dihapus. Logout server gagal: ${error.message}`)
      throw error
    } finally {
      localStorage.removeItem(TOKEN_KEY)
      localStorage.removeItem(REFRESH_TOKEN_KEY)
      localStorage.removeItem(USER_KEY)
      setAuth(null)
    }
  }

  async function updateName(name) {
    const profile = await requestProfileName(name, auth.user)
    const user = { ...auth.user, nama: profile.name }
    const nextAuth = { ...auth, user }
    localStorage.setItem(USER_KEY, JSON.stringify(user))
    setAuth(nextAuth)
    return profile
  }

  async function changePassword(currentPassword, newPassword) {
    await requestPasswordChange(currentPassword, newPassword, auth.user)
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(REFRESH_TOKEN_KEY)
    localStorage.removeItem(USER_KEY)
    setAuth(null)
    setSessionNotice('Silakan masuk kembali')
  }

  const value = {
    token: auth?.token ?? null,
    user: auth?.user ?? null,
    sessionNotice,
    login,
    logout,
    updateName,
    changePassword,
  }

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export default AuthProvider
