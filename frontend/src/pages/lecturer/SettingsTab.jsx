import { useRef, useState } from 'react'
import { useNavigate, useOutletContext } from 'react-router-dom'
import { deleteLecturerAssignment } from '../../api/lecturerAssignments.js'
import ConfirmDeleteDialog from '../../components/ConfirmDeleteDialog.jsx'
import FormFeedback from '../../components/FormFeedback.jsx'
import AssignmentForm from './AssignmentForm.jsx'
import { useFeedback } from './useFeedback.js'

function SettingsTab() {
  const { assignment, limits, onSaved } = useOutletContext()
  const navigate = useNavigate()
  const { error, success, revision, setError, setSuccess } = useFeedback()
  const [isConfirming, setIsConfirming] = useState(false)
  const deleteTrigger = useRef(null)

  async function confirmDelete() {
    try {
      await deleteLecturerAssignment(assignment.id)
      navigate(`/lecturer/assignments?courseId=${encodeURIComponent(assignment.courseId)}`, {
        state: { notice: 'Assignment berhasil dihapus.' },
      })
    } catch (deleteError) {
      setError(deleteError.message)
      setIsConfirming(false)
    }
  }

  return (
    <div className="space-y-8">
      <FormFeedback announcementKey={revision} error={error} success={success} />
      <AssignmentForm
        assignment={assignment}
        limits={limits}
        onSaved={(saved, savedLimits) => {
          onSaved(saved, savedLimits)
          setSuccess('Assignment berhasil diperbarui.')
        }}
        setError={setError}
        setSuccess={setSuccess}
      />
      <section
        aria-labelledby="danger-title"
        className="rounded-lg border border-danger/50 bg-surface p-5"
      >
        <h2 className="text-lg font-semibold text-danger" id="danger-title">Hapus assignment</h2>
        <p className="mt-1 text-sm text-muted">
          Test case dan submission assignment ini ikut hilang. Data yang dihapus tidak dapat
          dipulihkan.
        </p>
        <button
          className="mt-4 rounded-md border border-danger/50 px-4 py-2 font-medium text-danger"
          onClick={(event) => {
            deleteTrigger.current = event.currentTarget
            event.currentTarget.focus()
            setIsConfirming(true)
          }}
          type="button"
        >
          Hapus
        </button>
      </section>
      {isConfirming && (
        <ConfirmDeleteDialog
          onConfirm={confirmDelete}
          onDismiss={() => setIsConfirming(false)}
          returnFocusRef={deleteTrigger}
        />
      )}
    </div>
  )
}

export default SettingsTab
