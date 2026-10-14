import { useEffect, useState } from 'react'
import { getAdminAuditLogs, getAdminUsers, updateUserRole } from '../api/admin.js'

const roleLabels = {
  STUDENT: 'Mahasiswa',
  LECTURER: 'Dosen',
  ADMIN: 'Admin',
}

function AdminPage() {
  const [users, setUsers] = useState([])
  const [auditLogs, setAuditLogs] = useState([])
  const [isLoading, setIsLoading] = useState(true)
  const [loadError, setLoadError] = useState('')
  const [actionError, setActionError] = useState('')
  const [updatingUserId, setUpdatingUserId] = useState(null)

  useEffect(() => {
    let active = true
    async function loadData() {
      const results = await Promise.allSettled([getAdminUsers(), getAdminAuditLogs()])
      if (!active) {
        return
      }
      const errors = results.filter((result) => result.status === 'rejected')
      if (errors.length) {
        setLoadError(errors.map((result) => result.reason.message).join(' '))
      } else {
        setUsers(results[0].value.items)
        setAuditLogs(results[1].value.items)
      }
      setIsLoading(false)
    }
    loadData()
    return () => {
      active = false
    }
  }, [])

  async function handleRoleChange(user, role) {
    setActionError('')
    setUpdatingUserId(user.id)
    try {
      const updatedUser = await updateUserRole(user.id, role)
      setUsers((current) => current.map((entry) => (entry.id === user.id ? updatedUser : entry)))
    } catch (error) {
      setActionError(error.message)
    } finally {
      setUpdatingUserId(null)
    }
  }

  return (
    <main className="mx-auto max-w-6xl space-y-8 px-6 py-10">
      <h1 className="text-2xl font-semibold">Administrasi</h1>
      {isLoading && <p className="text-slate-600">Memuat data administrasi...</p>}
      {loadError && (
        <p className="text-red-700" role="alert">
          Gagal memuat data administrasi: {loadError}
        </p>
      )}
      {actionError && (
        <p className="text-red-700" role="alert">
          {actionError}
        </p>
      )}
      {!isLoading && !loadError && (
        <>
          <section aria-labelledby="admin-users-title" className="space-y-4">
            <h2 className="text-xl font-semibold" id="admin-users-title">
              Pengguna
            </h2>
            {users.length === 0 ? (
              <p className="text-slate-600">Belum ada pengguna.</p>
            ) : (
              <div className="overflow-x-auto rounded-lg border border-slate-200 bg-white">
                <table className="min-w-full text-left text-sm">
                  <thead className="bg-slate-100">
                    <tr>
                      <th className="p-3" scope="col">Nama</th>
                      <th className="p-3" scope="col">Email</th>
                      <th className="p-3" scope="col">Peran</th>
                      <th className="p-3" scope="col">Ubah peran</th>
                    </tr>
                  </thead>
                  <tbody>
                    {users.map((user) => (
                      <tr className="border-t border-slate-200" key={user.id}>
                        <td className="p-3">{user.name}</td>
                        <td className="p-3">{user.email}</td>
                        <td className="p-3">{roleLabels[user.role]}</td>
                        <td className="p-3">
                          <select
                            aria-label={`Ubah peran ${user.name}`}
                            className="rounded border border-slate-300 p-2"
                            disabled={updatingUserId === user.id}
                            onChange={(event) => handleRoleChange(user, event.target.value)}
                            value={user.role}
                          >
                            {Object.entries(roleLabels).map(([role, label]) => (
                              <option key={role} value={role}>{label}</option>
                            ))}
                          </select>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </section>

          <section aria-labelledby="audit-log-title" className="space-y-4">
            <h2 className="text-xl font-semibold" id="audit-log-title">
              Audit log keamanan sandbox
            </h2>
            {auditLogs.length === 0 ? (
              <p className="text-slate-600">Belum ada audit log.</p>
            ) : (
              <ul className="space-y-3">
                {auditLogs.map((entry) => (
                  <li className="rounded-lg border border-slate-200 bg-white p-4" key={entry.id}>
                    <h3 className="font-semibold">{entry.eventType}</h3>
                    <dl className="mt-2 grid gap-2 text-sm sm:grid-cols-2">
                      <div>
                        <dt className="text-slate-600">Pengguna</dt>
                        <dd>{entry.userId}</dd>
                      </div>
                      <div>
                        <dt className="text-slate-600">Submission</dt>
                        <dd>{entry.submissionId ?? 'Tidak terkait submission'}</dd>
                      </div>
                      <div>
                        <dt className="text-slate-600">Syscall ditolak</dt>
                        <dd>{entry.syscallNumber ?? '—'}</dd>
                      </div>
                      <div>
                        <dt className="text-slate-600">Waktu</dt>
                        <dd><time dateTime={entry.createdAt}>{entry.createdAt}</time></dd>
                      </div>
                    </dl>
                    <p className="mt-2 text-sm">{entry.detail}</p>
                  </li>
                ))}
              </ul>
            )}
          </section>
        </>
      )}
    </main>
  )
}

export default AdminPage
