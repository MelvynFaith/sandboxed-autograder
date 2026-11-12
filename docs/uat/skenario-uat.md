# Skenario UAT penuh SRS 11.5

Skenario ini mencakup kriteria 1 dan 2 SRS 11.5. Catat hasil aktual setiap
langkah; skenario tertulis bukan bukti bahwa UAT sudah dijalankan.

## Keterlacakan kriteria

| Kriteria | Langkah yang dijalankan |
| --- | --- |
| 2: buat Course, enroll mahasiswa, buat assignment dan test case, lihat rekap, ekspor CSV | Alur end-to-end 1–3 dan 7; Course dibuat pada langkah 1, bukan diasumsikan sudah tersedia. |
| 1: uji sedikitnya 10 submission, baca kode, bandingkan skor manual/otomatis, perbaiki test case dan regrade | Alur end-to-end 3–6; tinjauan kode mengacu FR-20 dan perbaikan/regrade mengacu FR-17. |

## Gerbang sebelum mulai

- Buat database khusus yang baru untuk run ini, misalnya
  `uat_run_YYYYMMDD` dengan tanggal run aktual.
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
   Atur resource limit ke CPU 1000 ms, memori 64 MB, maksimum 4 proses,
   timeout 5 detik, output 1048576 byte, dan tmpfs 32 MB.
5. Buat lima test case melalui UI; masing-masing berbobot 20 dan memakai
   pencocokan EXACT. Tandai `uat_05` sebagai tersembunyi:

   | ID | Input | Expected output | Bobot | Visibilitas |
   | --- | --- | --- | ---: | --- |
   | `uat_01` | `1` | `2` | 20 | Terlihat |
   | `uat_02` | `2` | `4` | 20 | Terlihat |
   | `uat_03` | `0` | `0` | 20 | Terlihat |
   | `uat_04` | `-2` | `-4` | 20 | Terlihat |
   | `uat_05` | `7` | `14` | 20 | Tersembunyi |

   Akhiri setiap input dan output dengan newline. Pastikan jumlah bobot bernilai
   100; dosen harus melihat kelima kasus dan status tersembunyinya pada daftar.

6. Untuk 10 submission, gunakan 10 berkas Python yang berbeda berikut, satu
   berkas per akun. Jangan unggah program yang sama untuk mahasiswa berbeda.
   Skor dihitung dari lima kasus berbobot sama dengan pencocokan EXACT:

   | ID | Program Python | Hasil yang diharapkan |
   | --- | --- | --- |
   | `UAT-P01` | `print(int(input()) * 2)` | 100/100 |
   | `UAT-P02` | `n = int(input()); print(n + n)` | 100/100 |
   | `UAT-P03` | `n = int(input().strip()); print(n << 1)` | 100/100 |
   | `UAT-P04` | `n = int(input()); print(abs(n) * 2)` | 80/100; gagal hanya pada `uat_04` |
   | `UAT-P05` | `n = int(input()); print(n * 2 if n >= 0 else 0)` | 80/100; gagal hanya pada `uat_04` |
   | `UAT-P06` | `n = int(input()); print(n * 2 + 1)` | 0/100; semua output salah |
   | `UAT-P07` | `print(f"{int(input()) * 2} ")` | 0/100; trailing space membuat semua output tidak cocok dengan EXACT |
   | `UAT-P08` | `n = int(input()); print(len(bytearray(1 << 30)) if n == 7 else 1 / 0)` | 0/100; runtime error pada empat kasus terlihat, memory limit pada kasus tersembunyi |
   | `UAT-P09` | `print(int(input()) * )` | 0/100; syntax error |
   | `UAT-P10` | `while True: pass` | 0/100; harus dihentikan oleh batas waktu sandbox |

   `UAT-P08` sengaja meminta alokasi 1 GiB pada satu kasus, sedangkan batas
   memori assignment 64 MB. Jalankan `UAT-P08` dan `UAT-P10` hanya jika
   penghentian proses oleh batas memori dan batas waktu sandbox terkonfirmasi
   pada lingkungan terisolasi; bila tidak, hentikan dan catat blocker. Jangan
   menjalankan program tersebut tanpa batas sumber daya yang terverifikasi.
   Catat email uji yang dipetakan ke setiap ID tanpa kata sandi.

## Alur end-to-end

1. **Course dan enrollment:** buat Course dan enroll 10 akun mahasiswa dari
   halaman UI, tanpa mengetik URL atau memakai API langsung.
   - Bukti: Course tampil pada daftar dosen; enrollment menampilkan 10 akun.
2. **Assignment dan test case:** buat assignment, enam resource limit, dan lima
   test case pada Course tersebut.
   - Bukti: assignment dan kelima kasus terlihat di UI setelah disimpan.
3. **Submission mahasiswa:** masuk bergantian melalui UI sebagai 10 mahasiswa.
   Setiap akun mengunggah berkas `.py` yang berbeda: `UAT-P01` untuk akun
   pertama, `UAT-P02` untuk akun kedua, dan seterusnya sampai `UAT-P10`.
   Gunakan kode persis pada tabel data uji, bukan satu solusi yang disalin
   untuk semua akun.
   - Bukti: 10 submission diterima; simpan ID/status yang terlihat tanpa
     mengubah data langsung. Skor akhir yang diharapkan berurutan adalah
     100, 100, 100, 80, 80, 0, 0, 0, 0, 0.
   - Sebagai salah satu mahasiswa, buka laporan submission sendiri. Pastikan
     hasil `uat_05` menampilkan status dan skor, tetapi tidak menampilkan input
     maupun expected output. Jangan membuka laporan melalui akun dosen untuk
     menggantikan pemeriksaan tampilan mahasiswa.
4. **Pembacaan kode (FR-20):** sebagai dosen, buka setiap submission dari rekap
   dan tinjau source code yang dikirim.
   - Bukti: checklist berisi 10 dari 10 submission yang telah ditinjau.
5. **Perbandingan skor manual dan otomatis:** hitung hasil lima kasus secara
   manual untuk seluruh 10 submission dan bandingkan dengan laporan test case.
   - Gunakan tabel tally manual ini sebelum melihat skor otomatis; setiap kasus
     berbobot 20 dan setiap hasil gagal/verdict error mendapat 0:

     | Program | `uat_01` | `uat_02` | `uat_03` | `uat_04` | `uat_05` | Skor manual |
     | --- | ---: | ---: | ---: | ---: | ---: | ---: |
     | `UAT-P01` | 20 | 20 | 20 | 20 | 20 | 100 |
     | `UAT-P02` | 20 | 20 | 20 | 20 | 20 | 100 |
     | `UAT-P03` | 20 | 20 | 20 | 20 | 20 | 100 |
     | `UAT-P04` | 20 | 20 | 20 | 0 | 20 | 80 |
     | `UAT-P05` | 20 | 20 | 20 | 0 | 20 | 80 |
     | `UAT-P06` | 0 | 0 | 0 | 0 | 0 | 0 |
     | `UAT-P07` | 0 | 0 | 0 | 0 | 0 | 0 |
     | `UAT-P08` | 0 | 0 | 0 | 0 | 0 | 0 |
     | `UAT-P09` | 0 | 0 | 0 | 0 | 0 | 0 |
     | `UAT-P10` | 0 | 0 | 0 | 0 | 0 | 0 |

   - Skor akhir manual berurutan adalah 100, 100, 100, 80, 80, 0, 0, 0, 0,
     0. Catat verdict tiap kasus dan penyimpangan satu per satu; jangan
     menyimpulkan pass hanya dari badge warna.
6. **Perbaikan test case dan regrade (FR-17):** melalui UI, ubah `uat_03`
   sementara agar expected output menjadi `1`. Regrade seluruh assignment dan
   pastikan setiap kasus diperiksa ulang. Skor sementara yang diharapkan untuk
   `UAT-P01` sampai `UAT-P10` adalah 80, 80, 80, 60, 60, 20, 0, 0, 0, 0:
   program benar kini gagal `uat_03`; dua program yang juga gagal pada kasus
   negatif kehilangan dua bobot; program salah-output mendapat satu kasus lulus.
   Pulihkan expected output menjadi `0`, regrade seluruh assignment lagi, dan
   pastikan skor kembali ke 100, 100, 100, 80, 80, 0, 0, 0, 0, 0. Jika backend
   tidak menyediakan jalur UI untuk regrade seluruh assignment, catat blocker;
   jangan mengganti test case/database dengan SQL atau curl.
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
- Pemeriksaan terakhir (2026-11-12): pembuatan database khusus
  `uat_run_20261112` ditolak karena role yang tersedia tidak memiliki hak
  `CREATEDB`. `sudo -n` tidak tersedia dan autentikasi peer sebagai `postgres`
  gagal. Operator database perlu memprovisikan database terisolasi dengan owner
  yang sesuai role migrasi, atau menyediakan kredensial khusus untuk database
  terisolasi itu.
- Keselamatan: `autograder_dev` tidak digunakan untuk migrasi, seed, server,
  atau UAT. Setelah operator menyediakan database, verifikasi
  `SELECT current_database()` sebelum menjalankan migrasi atau server.
- Yang belum diverifikasi: migrasi dan seed terisolasi, Course/enrollment,
  assignment, lima test case termasuk kerahasiaan laporan `uat_05`, 10 program
  dan skor submission, pembacaan kode, perbandingan skor, regrade setelah
  perubahan test case, ekspor CSV, dan jumlah intervensi.
- Pemilik langkah berikutnya: operator database memprovisikan database
  terisolasi; tim UAT menjalankan skenario setelah gerbang verifikasi lulus.
