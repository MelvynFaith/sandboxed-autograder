// navigator.clipboard exists only on HTTPS or localhost. The campus server is served over plain
// HTTP until it has a certificate, so the copy falls back to a hidden textarea and execCommand.
export async function copyText(text) {
  if (window.isSecureContext && navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text)
    return
  }
  const area = document.createElement('textarea')
  area.value = text
  area.setAttribute('readonly', '')
  area.style.position = 'fixed'
  area.style.opacity = '0'
  document.body.append(area)
  area.select()
  try {
    if (!document.execCommand('copy')) {
      throw new Error('copy refused')
    }
  } finally {
    area.remove()
  }
}
