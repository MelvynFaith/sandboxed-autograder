import { useState } from 'react'
import { getStoredTheme, saveTheme } from '../themePreference.js'

const options = [
  { value: 'system', label: 'Ikuti sistem', hint: 'Mengikuti pengaturan terang atau gelap di perangkat Anda.' },
  { value: 'light', label: 'Terang', hint: 'Selalu terang.' },
  { value: 'dark', label: 'Gelap', hint: 'Selalu gelap.' },
]

// Appearance setting on the profile page. Native radio buttons, so the arrow keys and the labels
// work without extra code.
function ThemeSetting() {
  const [theme, setTheme] = useState(getStoredTheme)

  function choose(value) {
    setTheme(value)
    saveTheme(value)
  }

  return (
    <fieldset className="mt-6 rounded-md border border-line bg-surface p-4">
      <legend className="px-1 text-lg font-semibold">Tampilan</legend>
      <div className="mt-2 grid gap-3 sm:grid-cols-3">
        {options.map((option) => (
          <label
            className={`flex min-h-11 cursor-pointer gap-3 rounded-md border p-3 ${
              theme === option.value ? 'border-accent bg-accent-tint' : 'border-line-strong hover:bg-raised'
            }`}
            key={option.value}
          >
            <input
              checked={theme === option.value}
              className="mt-1 shrink-0 self-start accent-[var(--accent)]"
              name="theme"
              onChange={() => choose(option.value)}
              type="radio"
              value={option.value}
            />
            <span>
              <span className="block font-medium">{option.label}</span>
              <span className="block text-sm text-muted">{option.hint}</span>
            </span>
          </label>
        ))}
      </div>
    </fieldset>
  )
}

export default ThemeSetting
