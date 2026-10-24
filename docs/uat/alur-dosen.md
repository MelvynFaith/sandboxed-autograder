# Alur UAT dosen

Panduan ini menjelaskan alur UI untuk dosen atau admin dari pengelolaan Course sampai
unduhan rekap. Semua halaman memerlukan autentikasi; mahasiswa tidak dapat mengakses
halaman pengelolaan dosen.

## Langkah alur

1. Buka **Course** (`/courses`), pilih Course, lalu kelola enrollment dengan email
   mahasiswa yang sudah terdaftar. Pilihan Course tersimpan pada URL ketika pindah
   ke langkah berikutnya.
2. Pilih **2. Assignment & test case** pada navigasi alur. Di
   **Kelola Assignment** (`/lecturer/assignments`), buat assignment pada Course yang
   dipilih, tetapkan batas sumber daya, lalu tambahkan test case. Assignment yang
   telah ada dapat dibuka dengan **Kelola**; **Rekap nilai & CSV** membuka rekapnya.
3. Pada **Rekap Nilai**, tinjau submission, status, skor, tenggat, dan nilai usang.
   Pilih **Unduh CSV** untuk mengekspor rekap. Tautan navigasi mengembalikan pengguna
   ke Course atau daftar assignment yang terkait.

## Pemeriksaan praktis

- Pastikan kode dan nama Course yang dipilih benar sebelum mendaftarkan mahasiswa atau
  membuat assignment.
- Pastikan test case terlihat pada bagian **Test case** setelah disimpan.
- Pastikan rekap menampilkan assignment yang benar. CSV diunduh melalui tombol UI agar
  autentikasi aplikasi tetap digunakan; jangan menyalin token ke URL atau dokumen.
- Gunakan navigasi langkah dan periksa parameter `courseId` tetap sesuai ketika
  berpindah halaman.

Pengujian backend nyata hanya boleh dilakukan pada database lokal yang baru dan
terisolasi bernama `uat_run`. Jangan menggunakan `autograder_dev` atau database bersama.
Lihat [skenario UAT](./skenario-uat.md); jangan publikasikan kredensial atau token.
