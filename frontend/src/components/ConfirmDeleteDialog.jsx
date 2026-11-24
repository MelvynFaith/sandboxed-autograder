import FocusTrapDialog from './FocusTrapDialog.jsx'

// "Are you sure" for a deletion that cannot be undone. Focus starts on Batal, the safe answer.
function ConfirmDeleteDialog({ onConfirm, onDismiss, returnFocusRef }) {
  return (
    <FocusTrapDialog
      labelledBy="confirm-delete-title"
      onDismiss={onDismiss}
      returnFocusRef={returnFocusRef}
    >
      <h2 className="text-lg font-semibold" id="confirm-delete-title">
        Konfirmasi penghapusan
      </h2>
      <p>Data yang dihapus tidak dapat dipulihkan.</p>
      <div className="flex gap-3">
        <button
          className="rounded-md bg-danger-solid px-4 py-2 text-white"
          onClick={onConfirm}
          type="button"
        >
          Hapus
        </button>
        <button
          className="rounded-md border border-line-strong px-4 py-2"
          data-dialog-initial-focus
          onClick={onDismiss}
          type="button"
        >
          Batal
        </button>
      </div>
    </FocusTrapDialog>
  )
}

export default ConfirmDeleteDialog
