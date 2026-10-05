import {
  BrowserRouter,
  Navigate,
  NavLink,
  Outlet,
  Route,
  Routes,
} from 'react-router-dom'
import AuthProvider from './context/AuthContext.jsx'
import { useAuth } from './context/useAuth.js'
import AssignmentsPage from './pages/AssignmentsPage.jsx'
import HistoryPage from './pages/HistoryPage.jsx'
import LoginPage from './pages/LoginPage.jsx'

const roleMenus = {
  mahasiswa: [
    { label: 'Daftar Assignment', to: '/assignments' },
    { label: 'Riwayat', to: '/history' },
  ],
  dosen: [{ label: 'Daftar Assignment', to: '/assignments' }],
  admin: [],
}

function ProtectedLayout() {
  const { user, logout } = useAuth()

  if (!user) {
    return <Navigate replace to="/login" />
  }

  return (
    <div className="min-h-screen bg-slate-50 text-slate-900">
      <nav
        aria-label="Navigasi utama"
        className="flex flex-wrap items-center gap-4 border-b border-slate-200 bg-white px-6 py-4"
      >
        <span className="mr-auto font-medium capitalize">{user.role}</span>
        {roleMenus[user.role].map(({ label, to }) => (
          <NavLink className="font-medium text-slate-700 hover:text-blue-700" key={to} to={to}>
            {label}
          </NavLink>
        ))}
        <button
          className="rounded border border-slate-300 px-3 py-1.5 font-medium hover:bg-slate-100"
          onClick={logout}
          type="button"
        >
          Logout
        </button>
      </nav>
      <Outlet />
    </div>
  )
}

function StudentHistoryRoute() {
  const { user } = useAuth()

  if (user.role !== 'mahasiswa') {
    return <Navigate replace to="/assignments" />
  }

  return <HistoryPage />
}

function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route element={<ProtectedLayout />}>
            <Route path="/" element={<Navigate replace to="/assignments" />} />
            <Route path="/assignments" element={<AssignmentsPage />} />
            <Route path="/history" element={<StudentHistoryRoute />} />
          </Route>
          <Route path="*" element={<Navigate replace to="/assignments" />} />
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  )
}

export default App
