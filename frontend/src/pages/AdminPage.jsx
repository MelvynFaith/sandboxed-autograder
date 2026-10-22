import { useCallback, useEffect, useState } from 'react'
import {
  getAdminAuditLogs,
  getAdminUsers,
  getQueueStatus,
  getRateLimits,
  rateLimitMaximums,
  updateRateLimits,
  updateUserRole,
} from '../api/admin.js'

const PAGE_SIZE = 50

const roleLabels = {
  STUDENT: 'Mahasiswa',
  LECTURER: 'Dosen',
  ADMIN: 'Admin',
}

const rateLimitFields = [
  {
    name: 'max_active_submissions',
    label: 'Submission aktif maksimum',
    hint: 'Jumlah submission berstatus QUEUED atau RUNNING milik satu mahasiswa pada satu assignment.',
  },
  {
    name: 'min_submission_interval_s',
    label: 'Jeda minimum antarsubmission (detik)',
    hint: 'Waktu tunggu sebelum mahasiswa dapat mengumpulkan lagi ke assignment yang sama.',
  },
  {
    name: 'max_submissions_per_assignment',
    label: 'Submission maksimum per assignment',
    hint: 'Jumlah submission satu mahasiswa ke satu assignment; yang dibatalkan tidak dihitung.',
  },
]

const dateTimeFormatter = new Intl.DateTimeFormat('id-ID', {
  dateStyle: 'long',
  timeStyle: 'medium',
  timeZone: 'Asia/Jakarta',
})

// Loads the data of one panel. Each panel has its own result, error and retry, so an endpoint that
// fails never hides another panel: user management (FR-19) must not depend on FR-12, 13 or 14.
// A result remembers the request it answers; until the current request answers, the panel is
// loading and keeps showing the previous data.
function useResource(load) {
  const [attempt, setAttempt] = useState(0)
  const [result, setResult] = useState({ load: null, attempt: -1, data: null, error: '' })

  useEffect(() => {
    let active = true
    load()
      .then((data) => {
        if (active) {
          setResult({ load, attempt, data, error: '' })
        }
      })
      .catch((error) => {
        if (active) {
          setResult((current) => ({ ...current, load, attempt, error: error.message }))
        }
      })
    return () => {
      active = false
    }
  }, [load, attempt])

  const setData = useCallback((update) => {
    setResult((current) => ({ ...current, data: update(current.data) }))
  }, [])
  const retry = useCallback(() => setAttempt((current) => current + 1), [])
  const isCurrent = result.load === load && result.attempt === attempt

  return {
    data: result.data,
    error: isCurrent ? result.error : '',
    isLoading: !isCurrent,
    setData,
    retry,
  }
}

function loadQueueStatus() {
  return getQueueStatus().then((queue) => ({ ...queue, loadedAt: new Date() }))
}

function LoadError({ message, onRetry, subject }) {
  return (
    <div className="space-y-2">
      <p className="text-red-700" role="alert">
        Gagal memuat {subject}: {message}
      </p>
      {onRetry && (
        <button
          aria-label={`Coba lagi memuat ${subject}`}
          className="rounded border border-slate-300 px-3 py-2"
          onClick={onRetry}
          type="button"
        >
          Coba lagi
        </button>
      )}
    </div>
  )
}

function Pagination({ disabled, hasNextPage, label, noun, onChange, page }) {
  return (
    <nav aria-label={`Paginasi ${label}`} className="flex flex-wrap items-center gap-3">
      <button
        className="rounded border border-slate-300 px-3 py-2 disabled:opacity-50"
        disabled={page === 1 || disabled}
        onClick={() => onChange(page - 1)}
        type="button"
      >
        {noun} sebelumnya
      </button>
      <span aria-current="page">Halaman {label} {page}</span>
      <button
        className="rounded border border-slate-300 px-3 py-2 disabled:opacity-50"
        disabled={!hasNextPage || disabled}
        onClick={() => onChange(page + 1)}
        type="button"
      >
        {noun} berikutnya
      </button>
    </nav>
  )
}

function UsersPanel() {
  const [page, setPage] = useState(1)
  const load = useCallback(() => getAdminUsers(page, PAGE_SIZE), [page])
  const { data, error, isLoading, retry, setData } = useResource(load)
  const [updatingUserId, setUpdatingUserId] = useState(null)
  const [actionError, setActionError] = useState('')
  const [notice, setNotice] = useState('')

  async function handleRoleChange(user, role) {
    setActionError('')
    setNotice('')
    setUpdatingUserId(user.id)
    try {
      const updatedUser = await updateUserRole(user.id, role)
      setData((current) => ({
        ...current,
        items: current.items.map((entry) => (entry.id === user.id ? updatedUser : entry)),
      }))
      setNotice(`Peran ${updatedUser.name} diubah menjadi ${roleLabels[updatedUser.role]}.`)
    } catch (roleError) {
      setActionError(roleError.message)
    } finally {
      setUpdatingUserId(null)
    }
  }

  return (
    <section aria-busy={isLoading} aria-labelledby="admin-users-title" className="space-y-4">
      <h2 className="text-xl font-semibold" id="admin-users-title">
        Pengguna
      </h2>
      <p aria-live="polite" className="text-sm text-green-800" role="status">
        {notice}
      </p>
      {actionError && (
        <p className="text-red-700" role="alert">
          {actionError}
        </p>
      )}
      {error ? (
        <LoadError message={error} onRetry={retry} subject="pengguna" />
      ) : !data ? (
        <p className="text-slate-600">Memuat pengguna...</p>
      ) : data.items.length === 0 ? (
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
              {data.items.map((user) => (
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
      <Pagination
        disabled={isLoading}
        hasNextPage={!error && Boolean(data?.hasNextPage)}
        label="pengguna"
        noun="Pengguna"
        onChange={setPage}
        page={page}
      />
    </section>
  )
}

function QueuePanel() {
  const { data, error, isLoading, retry } = useResource(loadQueueStatus)
  const stats = data && [
    { label: 'Pekerjaan di antrian', value: data.queueDepth },
    { label: 'Worker aktif', value: `${data.workersAlive} dari ${data.workersConfigured}` },
    { label: 'Submission menunggu (QUEUED)', value: data.queued },
    { label: 'Submission sedang dinilai (RUNNING)', value: data.running },
  ]

  return (
    <section aria-busy={isLoading} aria-labelledby="admin-queue-title" className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-xl font-semibold" id="admin-queue-title">
          Antrian dan worker
        </h2>
        <button
          className="rounded border border-slate-300 px-3 py-2 disabled:opacity-50"
          disabled={isLoading}
          onClick={retry}
          type="button"
        >
          Muat ulang status antrian
        </button>
      </div>
      {error ? (
        <LoadError message={error} subject="status antrian" />
      ) : !data ? (
        <p className="text-slate-600">Memuat status antrian...</p>
      ) : (
        <>
          <dl className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            {stats.map(({ label, value }) => (
              <div className="rounded-lg border border-slate-200 bg-white p-4" key={label}>
                <dt className="text-sm text-slate-600">{label}</dt>
                <dd className="mt-1 text-2xl font-semibold">{value}</dd>
              </div>
            ))}
          </dl>
          {data.workersAlive < data.workersConfigured && (
            <p className="text-amber-800">
              Sebagian worker tidak berjalan; submission dinilai lebih lambat dari biasanya.
            </p>
          )}
        </>
      )}
      <p aria-live="polite" className="text-sm text-slate-600" role="status">
        {data && !error && (
          <>
            Dimuat{' '}
            <time dateTime={data.loadedAt.toISOString()}>
              {dateTimeFormatter.format(data.loadedAt)} WIB
            </time>
          </>
        )}
      </p>
    </section>
  )
}

function toFormValues(limits) {
  return Object.fromEntries(rateLimitFields.map(({ name }) => [name, String(limits[name])]))
}

function RateLimitForm({ initialLimits }) {
  const [values, setValues] = useState(() => toFormValues(initialLimits))
  const [invalidField, setInvalidField] = useState(null)
  const [isSaving, setIsSaving] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

  async function handleSubmit(event) {
    event.preventDefault()
    setError('')
    setNotice('')
    const limits = {}
    for (const { name, label } of rateLimitFields) {
      const value = values[name].trim()
      if (!/^\d+$/.test(value) || Number(value) > rateLimitMaximums[name]) {
        setInvalidField(name)
        setError(`${label} harus bilangan bulat 0 sampai ${rateLimitMaximums[name]}.`)
        event.currentTarget.elements.namedItem(name).focus()
        return
      }
      limits[name] = Number(value)
    }
    setInvalidField(null)
    setIsSaving(true)
    try {
      setValues(toFormValues(await updateRateLimits(limits)))
      setNotice('Batas laju berhasil disimpan.')
    } catch (saveError) {
      setError(saveError.message)
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <form
      className="space-y-4 rounded-lg border border-slate-200 bg-white p-5"
      noValidate
      onSubmit={handleSubmit}
    >
      <p className="text-sm text-slate-700">
        Batas berlaku untuk setiap mahasiswa pada setiap assignment. Submission yang melewati batas
        ditolak.
      </p>
      {rateLimitFields.map(({ name, label, hint }) => (
        <div key={name}>
          <label className="mb-1 block font-medium" htmlFor={`rate-limit-${name}`}>
            {label}
          </label>
          <input
            aria-describedby={`rate-limit-${name}-hint`}
            aria-invalid={invalidField === name}
            className="w-full rounded border border-slate-300 p-2"
            id={`rate-limit-${name}`}
            inputMode="numeric"
            max={rateLimitMaximums[name]}
            min="0"
            name={name}
            onChange={(event) =>
              setValues((current) => ({ ...current, [name]: event.target.value }))
            }
            required
            step="1"
            type="number"
            value={values[name]}
          />
          <p className="mt-1 text-sm text-slate-600" id={`rate-limit-${name}-hint`}>
            {hint} Isi 0 untuk mematikan aturan ini. Rentang 0 sampai {rateLimitMaximums[name]}.
          </p>
        </div>
      ))}
      {error && (
        <p className="text-red-700" role="alert">
          {error}
        </p>
      )}
      <p aria-live="polite" className="text-sm text-green-800" role="status">
        {notice}
      </p>
      <button
        className="rounded bg-blue-700 px-4 py-2 font-medium text-white disabled:opacity-50"
        disabled={isSaving}
        type="submit"
      >
        {isSaving ? 'Menyimpan...' : 'Simpan batas laju'}
      </button>
    </form>
  )
}

function RateLimitPanel() {
  const { data, error, isLoading, retry } = useResource(getRateLimits)

  return (
    <section aria-busy={isLoading} aria-labelledby="admin-rate-limits-title" className="space-y-4">
      <h2 className="text-xl font-semibold" id="admin-rate-limits-title">
        Batas laju submission
      </h2>
      {error ? (
        <LoadError message={error} onRetry={retry} subject="batas laju" />
      ) : !data ? (
        <p className="text-slate-600">Memuat batas laju...</p>
      ) : (
        <RateLimitForm initialLimits={data} />
      )}
    </section>
  )
}

function AuditLogPanel() {
  const [page, setPage] = useState(1)
  const load = useCallback(() => getAdminAuditLogs(page, PAGE_SIZE), [page])
  const { data, error, isLoading, retry } = useResource(load)

  return (
    <section aria-busy={isLoading} aria-labelledby="audit-log-title" className="space-y-4">
      <h2 className="text-xl font-semibold" id="audit-log-title">
        Audit log keamanan sandbox
      </h2>
      {error ? (
        <LoadError message={error} onRetry={retry} subject="audit log" />
      ) : !data ? (
        <p className="text-slate-600">Memuat audit log...</p>
      ) : data.items.length === 0 ? (
        <p className="text-slate-600">Belum ada audit log.</p>
      ) : (
        <ul className="space-y-3">
          {data.items.map((entry) => (
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
                  <dd>
                    <time dateTime={entry.createdAt}>
                      {dateTimeFormatter.format(new Date(entry.createdAt))} WIB
                    </time>
                  </dd>
                </div>
              </dl>
              <p className="mt-2 text-sm break-words">{entry.detail}</p>
            </li>
          ))}
        </ul>
      )}
      <Pagination
        disabled={isLoading}
        hasNextPage={!error && Boolean(data?.hasNextPage)}
        label="audit log"
        noun="Audit log"
        onChange={setPage}
        page={page}
      />
    </section>
  )
}

function AdminPage() {
  return (
    <main className="mx-auto max-w-6xl space-y-8 px-6 py-10">
      <h1 className="text-2xl font-semibold">Administrasi</h1>
      <UsersPanel />
      <QueuePanel />
      <RateLimitPanel />
      <AuditLogPanel />
    </main>
  )
}

export default AdminPage
