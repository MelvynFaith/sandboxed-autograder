# Skenario UAT penuh SRS 11.5

Skenario ini mencakup kriteria 1 dan 2 SRS 11.5. Catat hasil aktual setiap
langkah; skenario tertulis bukan bukti bahwa UAT sudah dijalankan.

## Keterlacakan kriteria

| Kriteria | Langkah yang dijalankan |
| --- | --- |
| 2: buat Course, enroll mahasiswa, buat assignment dan test case, lihat rekap, ekspor CSV | Alur end-to-end 1–3 dan 7; Course dibuat pada langkah 1, bukan diasumsikan sudah tersedia. |
| 1: uji sedikitnya 10 submission, baca kode, bandingkan skor manual/otomatis, perbaiki test case dan regrade | Alur end-to-end 3–6; tinjauan kode mengacu FR-20 dan perbaikan/regrade mengacu FR-17. |

## Gerbang sebelum mulai

- Buat database khusus yang baru untuk run ini, misalnya `uat_run_20261025`.
  Jangan gunakan `autograder_dev`, database bersama, atau produksi.
- Atur `DATABASE_URL` untuk menunjuk ke database terisolasi **sebelum**
  menjalankan migrasi, seed, atau server. Jalankan
  `SELECT current_database()` dan pastikan hasilnya tepat nama database UAT.
  Simpan hanya nama database dan hasil verifikasi, bukan kredensial atau URL.
- Seed hanya akun dosen/admin yang diperlukan melalui prosedur resmi. Jangan
  mengisi data lewat SQL manual.
- Gunakan build frontend/backend yang SHA-nya dicatat. Jalankan server backend
  hanya setelah verifikasi database; gunakan frontend port 5174 sesuai rencana UAT.
- Siapkan unduhan CSV dari browser UI dan catatan skor manual yang bisa
  dibandingkan dengan laporan otomatis.

Jika database khusus tidak tersedia atau `current_database()` tidak cocok,
hentikan UAT backend tanpa menjalankan migrasi/server dan tandai bukti sebagai
`BELUM DIVERIFIKASI`.

## Data uji

1. Daftarkan melalui UI 10 akun mahasiswa unik dan catat email uji tanpa kata
   sandi.
2. Sebagai dosen, buat Course `UAT Algorithm`, kode unik `UAT-2026`, term
   `2026`.
3. Enroll seluruh 10 mahasiswa lewat UI Course.
4. Buat assignment berjudul `UAT Multiply by Two`, dengan deskripsi meminta
   program Python membaca satu integer dan mencetak nilainya dikali dua.
   Pilih deadline dan resource limit yang diterima form.
5. Buat lima test case melalui UI; masing-masing berbobot 20 dan memakai
   pencocokan EXACT:

   | ID | Input | Expected output | Bobot |
   | --- | --- | --- | ---: |
   | `uat_01` | `1` | `2` | 20 |
   | `uat_02` | `2` | `4` | 20 |
   | `uat_03` | `0` | `0` | 20 |
   | `uat_04` | `-2` | `-4` | 20 |
   | `uat_05` | `7` | `14` | 20 |

   Akhiri setiap input dan output dengan newline. Pastikan jumlah bobot bernilai
   100 dan kelima kasus tampil di daftar test case.

## Alur end-to-end

1. **Course dan enrollment:** buat Course dan enroll 10 akun mahasiswa dari
   halaman UI, tanpa mengetik URL atau memakai API langsung.
   - Bukti: Course tampil pada daftar dosen; enrollment menampilkan 10 akun.
2. **Assignment dan test case:** buat assignment, enam resource limit, dan lima
   test case pada Course tersebut.
   - Bukti: assignment dan kelima kasus terlihat di UI setelah disimpan.
3. **Submission mahasiswa:** masuk bergantian melalui UI sebagai 10 mahasiswa.
   Setiap akun mengunggah berkas `.py` berisi implementasi benar:
   `print(int(input()) * 2)`.
   - Bukti: 10 submission diterima; simpan ID/status yang terlihat tanpa
     mengubah data langsung.
4. **Pembacaan kode (FR-20):** sebagai dosen, buka setiap submission dari rekap
   dan tinjau source code yang dikirim.
   - Bukti: checklist berisi 10 dari 10 submission yang telah ditinjau.
5. **Perbandingan skor manual dan otomatis:** hitung hasil lima kasus secara
   manual untuk seluruh 10 submission dan bandingkan dengan laporan test case.
   - Nilai yang diharapkan untuk setiap berkas benar adalah 5/5 lulus dan
     100/100. Catat penyimpangan satu per satu; jangan menyimpulkan pass hanya
     dari badge warna.
6. **Perbaikan test case dan regrade (FR-17):** melalui UI, ubah `uat_03`
   sementara agar expected output menjadi `1`. Regrade submission terkait dan
   pastikan hasil kasus tersebut gagal serta skor berubah menjadi 80/100.
   Pulihkan expected output menjadi `0`, regrade ulang, dan pastikan lima kasus
   lulus serta skor kembali 100/100. Jika backend tidak menyediakan jalur UI
   untuk regrade satu submission atau semua submission, catat blocker; jangan
   mengganti test case/database dengan SQL atau curl.
7. **Rekap dan CSV:** buka rekap dari navigasi UI dan unduh CSV menggunakan
   tombol **Unduh CSV**. Periksa nama berkas, header, 10 baris mahasiswa, dan
   skor akhir; cocokkan isinya dengan rekap.
   - Bukti: simpan CSV hasil unduhan sebagai artefak lokal UAT dan catat hash
     serta jumlah baris. Jangan menambahkan token ke URL.

## Penghitungan intervensi

Hitung sebagai intervensi setiap tindakan di luar alur UI yang disediakan:

| Jenis intervensi | Target | Hasil aktual |
| --- | ---: | --- |
| Edit/insert/delete database secara manual | 0 | BELUM DIVERIFIKASI |
| Panggilan curl/API di luar UI | 0 | BELUM DIVERIFIKASI |
| Mengetik URL untuk melewati navigasi | 0 | BELUM DIVERIFIKASI |
| Total intervensi | 0 | BELUM DIVERIFIKASI |

Pendaftaran akun, pengisian form, login, upload berkas, dan klik navigasi adalah
langkah skenario UI, bukan intervensi. Catat setiap penyimpangan dari alur UI
sebagai intervensi dan jangan menargetkan hasil 0 dengan menghilangkan catatan.

## Hasil run

- Status: **BELUM DIVERIFIKASI**; belum ada bukti E2E pada database UAT terisolasi.
- Alasan: `AUTOGRADER_TEST_DATABASE_URL` pada lingkungan kerja menunjuk ke
  `autograder_dev`; role database yang tersedia tidak memiliki hak membuat
  database, dan kredensial server UAT khusus belum tersedia. Database tersebut
  tidak digunakan untuk pengujian ini.
- Yang belum diverifikasi: migrasi dan seed terisolasi, Course/enrollment,
  assignment, lima test case, 10 submission, pembacaan kode, perbandingan skor,
  regrade setelah perubahan test case, ekspor CSV, dan jumlah intervensi.
- Pemilik run berikutnya: tim UAT setelah database terisolasi diprovisikan.
