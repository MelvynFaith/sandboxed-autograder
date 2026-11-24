// Texts of the per-test-case report (FR-10).

export const verdictExplanations = {
  FAILED: 'Keluaran program tidak sesuai dengan yang diharapkan.',
  TIMEOUT: 'Program melewati batas waktu eksekusi.',
  MEMORY_LIMIT_EXCEEDED: 'Program melebihi batas penggunaan memori.',
  RUNTIME_ERROR: 'Program mengalami kesalahan saat dijalankan.',
  SYNTAX_ERROR: 'Program memiliki kesalahan sintaks.',
  SECURITY_VIOLATION: 'Program melakukan operasi yang tidak diizinkan oleh sandbox.',
}

// Submissions may use the Python standard library only (SRS 2.6), so an import of anything else
// fails in the sandbox; the bare Python message does not say why.
const importError = /^(ModuleNotFoundError|ImportError)\b/

export function errorHint(message) {
  if (message && importError.test(message)) {
    return 'Hanya standard library Python yang tersedia; modul dari luar (misalnya yang dipasang dengan pip) tidak dapat di-import.'
  }
  return null
}
