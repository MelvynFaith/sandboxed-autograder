import { BrowserRouter, NavLink, Navigate, Route, Routes } from 'react-router-dom'
import AssignmentsPage from './pages/AssignmentsPage.jsx'
import HistoryPage from './pages/HistoryPage.jsx'
import LoginPage from './pages/LoginPage.jsx'

function App() {
  return (
    <BrowserRouter>
      <div className="min-h-screen bg-slate-50 text-slate-900">
        <nav
          aria-label="Navigasi utama"
          className="flex flex-wrap gap-4 border-b border-slate-200 bg-white px-6 py-4"
        >
          <NavLink className="font-medium text-slate-700 hover:text-blue-700" to="/login">
            Login
          </NavLink>
          <NavLink
            className="font-medium text-slate-700 hover:text-blue-700"
            to="/assignments"
          >
            Daftar Assignment
          </NavLink>
          <NavLink className="font-medium text-slate-700 hover:text-blue-700" to="/history">
            Riwayat
          </NavLink>
        </nav>
        <Routes>
          <Route path="/" element={<Navigate replace to="/login" />} />
          <Route path="/login" element={<LoginPage />} />
          <Route path="/assignments" element={<AssignmentsPage />} />
          <Route path="/history" element={<HistoryPage />} />
          <Route path="*" element={<Navigate replace to="/login" />} />
        </Routes>
      </div>
    </BrowserRouter>
  )
}

export default App
