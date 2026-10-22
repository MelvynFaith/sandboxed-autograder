import { useState } from 'react'
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
import CoursesPage from './pages/CoursesPage.jsx'
import HistoryPage from './pages/HistoryPage.jsx'
import LecturerAssignmentsPage from './pages/LecturerAssignmentsPage.jsx'
import AdminPage from './pages/AdminPage.jsx'
import LoginPage from './pages/LoginPage.jsx'
import RegisterPage from './pages/RegisterPage.jsx'
import ProfilePage from './pages/ProfilePage.jsx'
import RecapPage from './pages/RecapPage.jsx'
import SubmissionCodePage from './pages/SubmissionCodePage.jsx'
import SubmissionReportPage from './pages/SubmissionReportPage.jsx'

const roleMenus = {
  mahasiswa: [
    { label: 'Daftar Assignment', to: '/assignments' },
    { label: 'Riwayat', to: '/history' },
  ],
  dosen: [
    { label: 'Daftar Assignment', to: '/assignments' },
    { label: 'Course', to: '/courses' },
    { label: 'Kelola Assignment', to: '/lecturer/assignments' },
  ],
  admin: [
    { label: 'Course', to: '/courses' },
    { label: 'Kelola Assignment', to: '/lecturer/assignments' },
    { label: 'Administrasi', to: '/admin' },
  ],
}

function ProtectedLayout() {
  const { user, logout } = useAuth()
  const [logoutError, setLogoutError] = useState('')

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
        <NavLink className="font-medium text-slate-700 hover:text-blue-700" to="/profile">
          Profil
        </NavLink>
        <button
          className="rounded border border-slate-300 px-3 py-1.5 font-medium hover:bg-slate-100"
          onClick={async () => {
            setLogoutError('')
            try {
              await logout()
            } catch (error) {
              setLogoutError(error.message)
            }
          }}
          type="button"
        >
          Logout
        </button>
      </nav>
      {logoutError && (
        <p className="bg-red-50 px-6 py-3 text-sm text-red-800" role="alert">
          {logoutError}
        </p>
      )}
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

function LecturerAssignmentsRoute() {
  const { user } = useAuth()

  if (user.role !== 'dosen' && user.role !== 'admin') {
    return <Navigate replace to="/assignments" />
  }

  return <LecturerAssignmentsPage />
}

// The recap and the lecturer report are for the course lecturer and admins; the API enforces
// ownership, this only keeps students off pages they cannot use.
function GradingStaffRoute({ children }) {
  const { user } = useAuth()

  if (user.role !== 'dosen' && user.role !== 'admin') {
    return <Navigate replace to="/assignments" />
  }

  return children
}

function AdminRoute() {
  const { user } = useAuth()
  if (user.role !== 'admin') {
    return <Navigate replace to="/assignments" />
  }
  return <AdminPage />
}

function CoursesRoute() {
  const { user } = useAuth()

  if (user.role === 'mahasiswa') {
    return <Navigate replace to="/assignments" />
  }

  return <CoursesPage />
}

function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/register" element={<RegisterPage />} />
          <Route element={<ProtectedLayout />}>
            <Route path="/" element={<Navigate replace to="/assignments" />} />
            <Route path="/assignments" element={<AssignmentsPage />} />
            <Route path="/courses" element={<CoursesRoute />} />
            <Route path="/history" element={<StudentHistoryRoute />} />
            <Route path="/lecturer/assignments" element={<LecturerAssignmentsRoute />} />
            <Route path="/admin" element={<AdminRoute />} />
            <Route path="/profile" element={<ProfilePage />} />
            <Route
              path="/lecturer/assignments/:assignmentId/recap"
              element={<GradingStaffRoute><RecapPage /></GradingStaffRoute>}
            />
            <Route
              path="/submissions/:submissionId/report"
              element={<GradingStaffRoute><SubmissionReportPage /></GradingStaffRoute>}
            />
            <Route path="/submissions/:submissionId/code" element={<SubmissionCodePage />} />
          </Route>
          <Route path="*" element={<Navigate replace to="/assignments" />} />
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  )
}

export default App
