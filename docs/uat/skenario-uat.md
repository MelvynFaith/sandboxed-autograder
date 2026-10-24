# Skenario UAT alur dosen

## Prasyarat

- Gunakan akun dosen atau admin yang sah dan data Course khusus UAT.
- Untuk pengujian frontend otomatis, jalankan perintah pada dokumentasi frontend tanpa
  menghubungkan browser ke backend.
- Jika pengujian backend nyata diperlukan, siapkan database terisolasi baru bernama
  `uat_run` dengan prosedur seed internal yang aman. Jangan menjalankan backend atau
  seed terhadap `autograder_dev`, database bersama, atau produksi. Jika database baru
  tidak dapat dibuat dan diverifikasi dengan aman, hentikan pengujian backend dan tandai
  belum diverifikasi.

## Skenario dosen

1. Masuk sebagai dosen dan buka **Course**.
   - **Diharapkan:** daftar Course yang dapat dikelola tampil dan navigasi menunjukkan
     langkah Course & enrollment.
2. Pilih Course, daftarkan mahasiswa UAT, lalu buka **Assignment & test case**.
   - **Diharapkan:** URL membawa `courseId` Course yang dipilih dan halaman assignment
     menampilkan Course yang sama.
3. Buat assignment dengan tenggat, batas akhir keterlambatan, dan batas sumber daya
   valid. Tambahkan satu test case terlihat dan satu tersembunyi.
   - **Diharapkan:** assignment tersimpan, test case tampil, dan validasi menolak batas
     sumber daya yang tidak valid.
4. Buka **Rekap nilai & CSV** untuk assignment.
   - **Diharapkan:** rekap sesuai assignment dan langkah navigasi kembali ke Course serta
     assignment yang benar.
5. Pilih **Unduh CSV**.
   - **Diharapkan:** berkas CSV diunduh dan halaman menampilkan nama berkas; kegagalan
     ekspor menampilkan pesan error, bukan sukses.

## Pemeriksaan navigasi dan akses

- Gunakan tautan langkah untuk bergerak maju dan kembali; Course terpilih tetap cocok.
- Muat ulang halaman assignment dengan `?courseId=<id>` dan pastikan Course tersebut
  tetap dipilih.
- Masuk sebagai mahasiswa dan buka route pengelolaan dosen atau rekap.
  - **Diharapkan:** mahasiswa dialihkan dan tidak melihat data pengelolaan dosen.

## Catatan hasil

Catat tanggal, browser, akun/role (tanpa kata sandi atau token), Course dan assignment
UAT, hasil tiap langkah, serta jumlah intervensi manual yang benar-benar dilakukan.
Pisahkan hasil tes otomatis, pemeriksaan browser, dan UAT backend nyata. Jangan
menyatakan langkah lolos bila belum diamati.
