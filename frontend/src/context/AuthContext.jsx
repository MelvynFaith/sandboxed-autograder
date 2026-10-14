import { useState } from 'react'
import { login as requestLogin, logout as requestLogout } from '../api/auth.js'
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
    typeof user?.id !== 'string' ||
    typeof user?.nama !== 'string' ||
    typeof user?.email !== 'string' ||
    !roles.includes(user.role)
  ) {
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(REFRESH_TOKEN_KEY)
    localStorage.removeItem(USER_KEY)
    return null
  }

  return { token, refreshToken, user }
}

function AuthProvider({ children }) {
  const [auth, setAuth] = useState(readStoredAuth)

  async function login(email, password) {
    const result = await requestLogin(email, password)
    localStorage.setItem(TOKEN_KEY, result.token)
    localStorage.setItem(REFRESH_TOKEN_KEY, result.refreshToken)
    localStorage.setItem(USER_KEY, JSON.stringify(result.user))
    setAuth(result)
    return result.user
  }

  async function logout() {
    await requestLogout(auth?.refreshToken)
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(REFRESH_TOKEN_KEY)
    localStorage.removeItem(USER_KEY)
    setAuth(null)
  }

  const value = {
    token: auth?.token ?? null,
    user: auth?.user ?? null,
    login,
    logout,
  }

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export default AuthProvider
