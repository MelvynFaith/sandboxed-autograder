# NFR-11 keyboard checklist

Tujuan: menilai urutan fokus, indikasi fokus, dialog, serta navigasi keyboard
tanpa mengklaim hasil yang tidak benar-benar dilakukan.

## Petunjuk

- Gunakan browser normal dan lakukan satu pemeriksaan per halaman.
- Catat hasil sebagai `PASS`, `FAIL`, atau `BELUM DIVERIFIKASI`.
- Jika belum diverifikasi, tuliskan alasannya dan pemeriksaan manual berikutnya.

## Checklist

| Halaman | Langkah | Hasil | Catatan |
| --- | --- | --- | --- |
| Login | Tab email → password → tombol masuk; Shift+Tab kembali | PARTIAL | Managed browser, mock, 360 px: email/password/tombol dapat difokuskan; Enter mengirim login demo; indikator `:focus-visible` tampak pada setiap fokus yang diperiksa. Belum mengecek semua ukuran viewport/perangkat. |
| Register | Tab melewati semua field dan tombol submit | PASS (alur fokus) | Managed browser, mock, 360 px: Nama → Email → Password → Konfirmasi password → Daftar; Shift+Tab kembali ke konfirmasi password. Validasi kirim pendaftaran belum dijalankan. |
| Upload submission | Tab dari file input ke tombol submit dan cancel | PARTIAL | Halaman mahasiswa: kedua kontrol pemilih file untuk assignment dapat difokuskan. Tombol upload tidak aktif tanpa file; native file chooser, pemilihan file, dan pengiriman belum diuji. |
| Course | Tab melalui form Course dan tombol simpan | PASS (alur fokus) | Dosen, mock, 360 px: Nama → Kode → Term → Simpan Course; Enter pada form kosong menampilkan validasi dan tidak membuat data. |
| Enrollment | Tab melalui field dan tombol enrollment | PASS (alur fokus) | Dosen, mock, 360 px: Email mahasiswa → Tambah enrollment; Enter tanpa pilihan/email menampilkan validasi dan tidak membuat enrollment. |
| Assignment | Tab melalui field assignment dan tombol aksi | PARTIAL | Dosen, mock, 360 px: Judul, Deskripsi dan Deadline terfokus; pengujian Tab berhenti pada input tanggal di managed browser. Submit dengan Enter fokus ke field wajib pertama; tidak membuat assignment. |
| Resource limit | Tab melalui semua field batas dan tombol simpan | PASS (alur fokus) | Dosen, mock, 360 px: keenam spinbutton dan tombol Buat assignment dapat difokuskan berurutan. Enter pada formulir kosong hanya menampilkan validasi. |
| Test case | Tab melalui field, checkbox tersembunyi, dan tombol aksi | PASS (alur fokus) | Dosen, mock, 360 px: ID eksternal → Input → Keluaran → Bobot → Mode pencocokan → checkbox tersembunyi → Tambah test case. Enter pada form kosong hanya memicu validasi; tidak ada data dibuat. |
| Recap / CSV | Tab melalui tabel, filter, dan tombol ekspor CSV | PARTIAL | Pengujian managed browser sebelumnya menekan Enter pada Unduh CSV dan melihat notifikasi sukses. Penelusuran seluruh tabel dengan keyboard dan pemeriksaan byte CSV belum dilakukan. |
| Admin role | Tab ke kontrol perubahan role dan aktifkan dengan Enter/Space | PARTIAL | Pemeriksaan managed browser terdahulu menelusuri select tanpa mengubah peran. Pada percobaan ini kontrol dapat difokuskan dengan klik, tetapi Tab tidak berpindah di managed browser; Enter/Space tidak digunakan agar nilai peran tidak berubah. |
| Dialog hapus | Fokus awal ada di dialog; Tab/Shift+Tab tidak keluar dari dialog | PASS (managed browser sebelumnya) | Hasil terdahulu: fokus awal Batal; Tab/Shift+Tab berputar di antara Hapus dan Batal. Periksa ulang di browser normal sebelum klaim audit manual penuh. |
| Dialog hapus | Escape menutup dialog dan fokus kembali ke pemicu | PASS (managed browser sebelumnya) | Hasil terdahulu: Escape menutup dialog dan fokus kembali ke tombol pemicu. |
| Regrade-all | Tab tetap di konfirmasi; Escape menutupnya dan fokus kembali | PASS (managed browser sebelumnya) | Hasil terdahulu: fokus awal Batal, Tab/Shift+Tab tetap di konfirmasi, Escape menutup dan memulihkan fokus. |

## Kriteria

- Fokus terlihat jelas pada setiap tautan, tombol, dan field.
- Semua kontrol dapat dijangkau dan diaktifkan dengan keyboard.
- Dialog modal menahan fokus; Tab dan Shift+Tab berputar di dalam dialog.
- Setelah dialog ditutup, fokus kembali ke kontrol yang membukanya.
- Pemeriksaan di atas hanya sebagian dan memakai managed browser dengan mock,
  bukan pemeriksaan manual lengkap di browser normal pada 360/768/1280 px.
- `PARTIAL` dan `BELUM DIVERIFIKASI` bukan hasil lulus NFR-11. Lengkapi
  pemeriksaan yang tersisa dan nilai tampilan fokus secara visual sebelum
  menyatakan seluruh navigasi keyboard lulus.
