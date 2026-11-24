import { useState } from 'react'
import {
  createLecturerAssignment,
  updateLecturerAssignment,
  updateResourceLimit,
} from '../../api/lecturerAssignments.js'
import {
  assignmentToForm,
  emptyAssignment,
  emptyLimits,
  limitsToForm,
  resourceLimitFields,
  validateResourceLimits,
} from './assignmentFields.js'

// The fields of an assignment and of its resource limits, as one form. Without `assignment` it
// creates one in `courseId` (an assignment is always created together with its limits, SRS 7.1);
// with `assignment` and `limits` it changes them. After a failed limit save the assignment fields
// are already stored, so a retry only repeats what failed.
function AssignmentForm({ assignment, courseId, limits, onSaved, setError, setSuccess }) {
  const isEditing = Boolean(assignment)
  const [form, setForm] = useState(() => (isEditing ? assignmentToForm(assignment) : emptyAssignment))
  const [limitForm, setLimitForm] = useState(() => (isEditing ? limitsToForm(limits) : emptyLimits))

  function handleChange(event) {
    setForm((current) => ({ ...current, [event.target.name]: event.target.value }))
  }

  async function save(event) {
    event.preventDefault()
    setError('')
    setSuccess('')
    const numericLimits = Object.fromEntries(
      Object.entries(limitForm).map(([name, value]) => [name, Number(value)]),
    )
    const limitError = validateResourceLimits(numericLimits)
    if (limitError) {
      setError(limitError)
      return
    }
    if (
      !form.title.trim() ||
      !form.description.trim() ||
      !Number.isFinite(Date.parse(form.deadline_at)) ||
      !Number.isFinite(Date.parse(form.late_until_at))
    ) {
      setError('Judul, deskripsi, deadline, dan batas akhir keterlambatan wajib diisi.')
      return
    }
    if (Date.parse(form.late_until_at) < Date.parse(form.deadline_at)) {
      setError('Batas akhir keterlambatan harus sama dengan atau setelah deadline.')
      return
    }
    const payload = {
      title: form.title.trim(),
      description: form.description.trim(),
      deadline_at: new Date(form.deadline_at).toISOString(),
      late_until_at: new Date(form.late_until_at).toISOString(),
    }

    try {
      const saved = isEditing
        ? await updateLecturerAssignment(assignment.id, payload)
        : await createLecturerAssignment(courseId, payload, numericLimits)
      if (isEditing) {
        await updateResourceLimit(saved.id, numericLimits)
      }
      onSaved(saved, numericLimits)
    } catch (saveError) {
      setError(saveError.message)
    }
  }

  return (
    <form className="space-y-5 rounded-lg border border-line bg-surface p-5" onSubmit={save}>
      <h2 className="text-lg font-semibold">{isEditing ? 'Ubah assignment' : 'Buat assignment'}</h2>
      <label className="block">
        <span className="mb-1 block font-medium">Judul</span>
        <input
          className="w-full rounded-md border border-line-strong p-2"
          name="title"
          onChange={handleChange}
          required
          value={form.title}
        />
      </label>
      <label className="block">
        <span className="mb-1 block font-medium">Deskripsi</span>
        <textarea
          className="w-full rounded-md border border-line-strong p-2"
          name="description"
          onChange={handleChange}
          required
          rows="3"
          value={form.description}
        />
      </label>
      <div className="grid gap-4 sm:grid-cols-2">
        {[
          { name: 'deadline_at', label: 'Deadline' },
          { name: 'late_until_at', label: 'Batas akhir keterlambatan' },
        ].map(({ name, label }) => (
          <label className="block" key={name}>
            <span className="mb-1 block font-medium">{label}</span>
            <input
              className="w-full rounded-md border border-line-strong p-2"
              name={name}
              onChange={handleChange}
              required
              type="datetime-local"
              value={form[name]}
            />
          </label>
        ))}
      </div>
      <fieldset className="grid gap-4 sm:grid-cols-2">
        <legend className="mb-3 font-semibold">Batas sumber daya</legend>
        {resourceLimitFields.map(({ name, label }) => (
          <label className="block" key={name}>
            <span className="mb-1 block font-medium">{label}</span>
            <input
              className="w-full rounded-md border border-line-strong p-2"
              min="1"
              name={name}
              onChange={(event) =>
                setLimitForm((current) => ({ ...current, [name]: event.target.value }))
              }
              required
              step="1"
              type="number"
              value={limitForm[name]}
            />
          </label>
        ))}
      </fieldset>
      <div>
        <button className="rounded-md bg-accent px-4 py-2 font-medium text-white" type="submit">
          {isEditing ? 'Simpan perubahan' : 'Buat assignment'}
        </button>
      </div>
    </form>
  )
}

export default AssignmentForm
