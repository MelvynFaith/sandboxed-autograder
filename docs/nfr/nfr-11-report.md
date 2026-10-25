# Laporan audit NFR-11: responsif dan aksesibilitas

- Tanggal audit: 2026-10-25
- Commit viewport dan target sentuh yang diukur: `9f44d9f`
- Commit axe-core yang dipindai: `9f44d9f`
- Commit dialog yang diuji dengan keyboard browser: `b3108a0`
- Lingkungan viewport: frontend mock `VITE_USE_MOCK=true`, Vite `http://127.0.0.1:5177`
- Lingkungan keyboard: frontend mock `VITE_USE_MOCK=true`, Vite `http://127.0.0.1:5176`
- Viewport: 360 × 900, 768 × 900, 1280 × 900 CSS px
- Issue: [#112](https://github.com/MelvynFaith/sandboxed-autograder/issues/112)

## Ringkasan

Di antara 63 kombinasi halaman konten, peran, dan lebar viewport, tidak ada
overflow horizontal dokumen, field tanpa label, atau tautan/tombol tanpa nama.
Tombol, field, pilihan, tautan navigasi dan aksi, serta label checkbox yang
diukur memenuhi ukuran target minimum 44 × 44 CSS px. Pemindaian axe-core
untuk 21 kombinasi halaman/peran menemukan 0 pelanggaran WCAG 2.0/2.1 A/AA.
Tabel rekap dan administrasi memakai wadah `overflow-x-auto`; pada 360 px
keduanya dapat menggulir di dalam wadah tanpa memperlebar dokumen.

Keyboard diuji memakai managed browser pada sebagian alur: login, register,
Course/enrollment, assignment/test-case, kontrol peran admin, ekspor CSV, dan
dialog hapus, pembatalan submission, serta regrade-all. Browser menerima Tab,
Shift+Tab, Enter, dan Escape pada pemeriksaan tersebut. Ini bukan pemeriksaan
manual lengkap pada setiap kontrol atau perangkat fisik; item yang belum diuji
tetap ditandai `BELUM DIVERIFIKASI, perlu pengecekan manual`.

Chromium Playwright mandiri berhasil diunduh, tetapi tidak dapat dijalankan
karena host tidak memiliki `libnspr4.so`, `libnss3.so`, dan `libasound.so.2`.
Instalasi paket sistem terhalang karena akses sudo interaktif tidak tersedia.
Karena itu hasil keyboard di bawah berasal dari managed browser, bukan dari
Chromium mandiri.

## Rute dan peran

Pemetaan berdasarkan `frontend/src/App.jsx`. Rute proteksi API seperti
kepemilikan course/submission tetap di luar cakupan audit UI ini.

| Rute | Peran yang diizinkan | Catatan akses |
| --- | --- | --- |
| `/login` | Publik | Halaman konten diukur. |
| `/register` | Publik | Halaman konten diukur. |
| `/` | Mahasiswa, dosen, admin | Mengarahkan ke `/assignments`; redirect runtime belum diverifikasi pada pengukuran ini. |
| `/assignments` | Mahasiswa, dosen, admin | Halaman konten diukur untuk semua peran. |
| `/courses` | Dosen, admin | Halaman konten diukur untuk peran yang diizinkan; mahasiswa ditolak melalui redirect ke `/assignments` (runtime belum diverifikasi). |
| `/history` | Mahasiswa | Halaman konten diukur untuk mahasiswa; dosen/admin ditolak melalui redirect (runtime belum diverifikasi). |
| `/lecturer/assignments` | Dosen, admin | Halaman konten diukur untuk peran yang diizinkan; mahasiswa ditolak melalui redirect (runtime belum diverifikasi). |
| `/admin` | Admin | Halaman konten diukur untuk admin; mahasiswa/dosen ditolak melalui redirect (runtime belum diverifikasi). |
| `/profile` | Mahasiswa, dosen, admin | Halaman konten diukur untuk semua peran. |
| `/lecturer/assignments/:assignmentId/recap` | Dosen, admin | Halaman konten diukur untuk peran yang diizinkan; mahasiswa ditolak melalui redirect (runtime belum diverifikasi). |
| `/submissions/:submissionId/report` | Dosen, admin | Halaman konten diukur untuk peran yang diizinkan; mahasiswa ditolak melalui redirect (runtime belum diverifikasi). |
| `/submissions/:submissionId/code` | Mahasiswa, dosen, admin | Halaman konten diukur untuk semua peran. |
| `*` | Semua; navigasi anonim akhirnya memerlukan login | Fallback mengarahkan ke `/assignments`; redirect runtime belum diverifikasi pada pengukuran ini. |

Rute dinamis menggunakan assignment `4` dan submission `submission-1`.
Rute redirect `/`, fallback `*`, dan pengalihan untuk peran yang tidak diizinkan
tercatat dari konfigurasi rute, bukan hasil navigasi browser yang terukur.

## Hasil viewport per halaman dan peran

Nilai adalah `max(document.documentElement.scrollWidth, body.scrollWidth) /
innerWidth`. Lulus jika lebar dokumen tidak melebihi viewport. Untuk semua
baris berikut, kolom label field dan nama kontrol lulus pada setiap lebar yang
tertera: field isian yang ditemukan semuanya memiliki label aksesibel, dan
setiap tautan/tombol memiliki nama aksesibel. Ukuran target dan keyboard
dijelaskan pada bagian tersendiri.

| Halaman | Peran | 360 px | 768 px | 1280 px |
| --- | --- | ---: | ---: | ---: |
| `/login` | Publik | 360/360 lulus | 768/768 lulus | 1280/1280 lulus |
| `/register` | Publik | 360/360 lulus | 768/768 lulus | 1280/1280 lulus |
| `/assignments` | Mahasiswa | 345/360 lulus | 753/768 lulus | 1265/1280 lulus |
| `/history` | Mahasiswa | 345/360 lulus | 768/768 lulus | 1280/1280 lulus |
| `/profile` | Mahasiswa | 345/360 lulus | 768/768 lulus | 1280/1280 lulus |
| `/submissions/submission-1/code` | Mahasiswa | 360/360 lulus | 768/768 lulus | 1280/1280 lulus |
| `/assignments` | Dosen | 360/360 lulus | 768/768 lulus | 1280/1280 lulus |
| `/courses` | Dosen | 345/360 lulus | 753/768 lulus | 1265/1280 lulus |
| `/lecturer/assignments` | Dosen | 345/360 lulus | 753/768 lulus | 1265/1280 lulus |
| `/profile` | Dosen | 345/360 lulus | 768/768 lulus | 1280/1280 lulus |
| `/lecturer/assignments/4/recap` | Dosen | 360/360 lulus | 768/768 lulus | 1280/1280 lulus |
| `/submissions/submission-1/report` | Dosen | 345/360 lulus | 753/768 lulus | 1265/1280 lulus |
| `/submissions/submission-1/code` | Dosen | 360/360 lulus | 768/768 lulus | 1280/1280 lulus |
| `/assignments` | Admin | 360/360 lulus | 768/768 lulus | 1280/1280 lulus |
| `/courses` | Admin | 345/360 lulus | 753/768 lulus | 1265/1280 lulus |
| `/lecturer/assignments` | Admin | 345/360 lulus | 753/768 lulus | 1265/1280 lulus |
| `/admin` | Admin | 345/360 lulus | 753/768 lulus | 1265/1280 lulus |
| `/profile` | Admin | 345/360 lulus | 768/768 lulus | 1280/1280 lulus |
| `/lecturer/assignments/4/recap` | Admin | 360/360 lulus | 768/768 lulus | 1280/1280 lulus |
| `/submissions/submission-1/report` | Admin | 345/360 lulus | 753/768 lulus | 1265/1280 lulus |
| `/submissions/submission-1/code` | Admin | 360/360 lulus | 768/768 lulus | 1280/1280 lulus |

### Label dan nama kontrol

Audit DOM pada 63 kombinasi route/peran/viewport menemukan:

- 0 field tanpa label terasosiasi atau nama aksesibel.
- 0 tombol/tautan tanpa nama aksesibel.
- Elemen fokus pada alur yang diuji dapat dicapai dengan Tab; urutan seluruh
  kontrol di semua rute/peran belum diuji tuntas.

### Pemindaian axe-core

axe-core 4.10.3 dijalankan pada viewport 360 × 900 CSS px di managed browser,
untuk tag WCAG 2.0 A/AA dan WCAG 2.1 A/AA. Jumlah pelanggaran mencakup semua
tingkat dampak; masing-masing baris berikut menghasilkan 0 pelanggaran.

| Rute | Peran | Pelanggaran |
| --- | --- | ---: |
| `/login` | Publik | 0 |
| `/register` | Publik | 0 |
| `/assignments` | Mahasiswa | 0 |
| `/history` | Mahasiswa | 0 |
| `/profile` | Mahasiswa | 0 |
| `/submissions/submission-1/code` | Mahasiswa | 0 |
| `/assignments` | Dosen | 0 |
| `/courses` | Dosen | 0 |
| `/lecturer/assignments` | Dosen | 0 |
| `/profile` | Dosen | 0 |
| `/lecturer/assignments/4/recap` | Dosen | 0 |
| `/submissions/101/report` | Dosen | 0 |
| `/submissions/101/code` | Dosen | 0 |
| `/assignments` | Admin | 0 |
| `/courses` | Admin | 0 |
| `/lecturer/assignments` | Admin | 0 |
| `/admin` | Admin | 0 |
| `/profile` | Admin | 0 |
| `/lecturer/assignments/4/recap` | Admin | 0 |
| `/submissions/101/report` | Admin | 0 |
| `/submissions/101/code` | Admin | 0 |

### Fokus, urutan Tab, dan dialog

| Pemeriksaan | Hasil |
| --- | --- |
| Tab pada login/register | Managed browser: Tab pertama pada login memfokuskan email; register menelusuri nama, email, password, konfirmasi password, tombol daftar, dan tautan login. |
| Tab pada Course/enrollment | Pemeriksaan lanjutan pada 2026-10-25, managed browser, mock, 360 px: Course menelusuri Nama → Kode → Term → Simpan Course; enrollment menelusuri Email mahasiswa → Tambah enrollment. Enter pada kedua form kosong hanya menampilkan validasi. |
| Tab pada assignment dan resource limit | Pemeriksaan lanjutan pada 2026-10-25, managed browser, mock, 360 px: Judul, Deskripsi dan Deadline tercapai; urutan Tab dari input tanggal tidak dapat dipastikan karena managed browser tidak menunjukkan perubahan fokus. Keenam spinbutton dan tombol assignment dapat dicapai berurutan. Enter pada form kosong memfokuskan field wajib tanpa membuat data. |
| Tab pada form test-case | Pemeriksaan lanjutan pada 2026-10-25, managed browser, mock, 360 px: ID eksternal, Input, Keluaran, Bobot, Mode pencocokan, checkbox tersembunyi, dan tombol tambah dapat difokuskan berurutan. Enter pada form kosong memicu validasi dan tidak membuat data. |
| Tab pada kontrol peran admin | Pemeriksaan terdahulu mencatat select dapat ditelusuri tanpa perubahan peran. Percobaan ulang 2026-10-25 hanya dapat memfokuskan select melalui klik; Tab tidak menunjukkan perpindahan fokus di managed browser. Hasil penuh tetap **BELUM DIVERIFIKASI**. |
| Aktivasi Enter pada ekspor CSV | Managed browser terdahulu menekan Enter pada tombol ekspor; status menyatakan `Rekap nilai diunduh sebagai rekap-assignment-4.csv`. Pemeriksaan lanjutan belum menelusuri seluruh tabel; isi byte CSV belum diperiksa. |
| Tampilan fokus yang terlihat | Login: `:focus-visible` aktif dan browser menampilkan outline 1 px pada field, tombol, dan tautan yang difokuskan. Penilaian visual lengkap untuk seluruh kontrol membutuhkan pengecekan manual di browser normal dan tetap **BELUM DIVERIFIKASI**. |
| Dialog hapus | Managed browser: fokus awal `Batal`; Tab/Shift+Tab berputar antara `Hapus` dan `Batal`; Escape menutup dialog dan fokus kembali ke tombol pemicu. |
| Dialog batalkan submission | Managed browser: fokus awal `Tidak`; Tab/Shift+Tab berputar dengan `Ya, batalkan`; Escape menutup dialog dan fokus kembali ke pemicu. |
| Dialog regrade-all | Managed browser: fokus awal `Batal`; Tab/Shift+Tab berputar dengan `Ya, nilai ulang semua`; Escape menutup dialog dan fokus kembali ke pemicu. |
| Upload submission dengan file | Managed browser memfokuskan tombol pemilih berkas untuk kedua assignment. **BELUM DIVERIFIKASI, perlu pengecekan manual** untuk membuka file chooser, memilih file, dan mengirim submission. |
| Fokus trap dalam tes otomatis | RTL memeriksa Tab/Shift+Tab, Escape, dan pemulihan fokus untuk dialog hapus, pembatalan submission, serta regrade-all. |

### Status, target sentuh, dan tabel

| Pemeriksaan | Hasil |
| --- | --- |
| Status/verdict selain warna | Lulus pemeriksaan implementasi dan DOM: status submission, verdict, dan status pada rekap ditampilkan dengan teks label selain warna. |
| Target sentuh | Pada 63 kombinasi halaman/peran/lebar (360/768/1280 px), semua tombol, field non-checkbox/radio, select, textarea, tautan navigasi, tautan aksi, dan label checkbox yang diukur berukuran minimal 44 × 44 CSS px. Tautan inline `Register` (52 × 19 px) dan `Login` (35 × 19 px) tetap berupa tautan di dalam kalimat dan termasuk pengecualian inline WCAG 2.2 SC 2.5.8. |
| Tabel rekap | Wrapper tabel memiliki `overflow-x-auto`; pada 360 px tabel menggulir di wadahnya sendiri tanpa overflow dokumen. |
| Tabel administrasi | Wrapper daftar pengguna memiliki `overflow-x-auto`; pada 360 px tabel menggulir di wadahnya sendiri tanpa overflow dokumen. |

## Validasi

Validasi yang dijalankan pada branch laporan:

| Perintah | Hasil |
| --- | --- |
| `cd frontend && npm ci` | Lulus |
| `cd frontend && npm run lint` | Lulus |
| `cd frontend && npm test` | Lulus; 24 berkas, 192 tes |
| `cd frontend && npm run build` | Lulus |
| `cd frontend && npm test -- src/pages/LecturerAssignmentsPage.test.jsx -t "confirms assignment deletion and supports Escape to dismiss the dialog"` | Lulus; 1 tes |
| Mutasi sementara kondisi Escape menjadi `Escape-mutation`, lalu jalankan tes Escape | Gagal sesuai harapan karena dialog tetap terbuka; setelah kondisi dipulihkan, tes lulus dan `git diff --exit-code -- frontend/src/pages/LecturerAssignmentsPage.jsx` lulus. |

Viewport: halaman publik dan rute terlindungi diukur pada 360, 768, dan 1280 px;
63 dari 63 kombinasi tidak menunjukkan overflow dokumen atau kontrol terukur
di bawah 44 × 44 px. Pemindaian axe-core 4.10.3 pada 21 kombinasi halaman/peran
mencatat 0 pelanggaran. Pemeriksaan browser keyboard untuk dialog dan alur yang
disebutkan di atas berhasil pada managed browser; Chromium mandiri gagal
diluncurkan karena dependensi sistem tidak tersedia.

## Kesimpulan

Tidak ditemukan kegagalan terukur untuk overflow dokumen, target sentuh yang
diukur, label, nama kontrol, status tekstual, atau scrolling tabel dalam wadah.
axe-core tidak menemukan pelanggaran pada rute/peran yang dipindai. Dialog
konfirmasi yang diuji berfungsi dengan keyboard pada managed browser, tetapi
laporan ini tidak menyatakan seluruh pengalaman keyboard lulus. Fokus terlihat,
upload submission melalui file chooser keyboard, pemeriksaan seluruh kontrol,
dan pemeriksaan manual pada perangkat fisik tetap **BELUM DIVERIFIKASI, perlu
pengecekan manual** sebelum menyatakan lulus penuh NFR-11.
