# Alur UAT dosen

Panduan ini mencakup alur Course sampai rekap CSV untuk kriteria 2 SRS 11.5.
Semua tindakan dilakukan melalui UI yang tersedia, bukan dengan mengetik rute
secara manual. Gunakan [skenario UAT lengkap](./skenario-uat.md) untuk langkah,
data uji, perhitungan skor, dan pencatatan intervensi. Mahasiswa tidak dapat
mengakses halaman pengelolaan dosen.

## Langkah alur

1. Masuk sebagai dosen melalui aplikasi. Dari **Course**, buat Course baru
   dengan nama, kode, dan term yang belum digunakan.
2. Daftarkan sedikitnya 10 akun mahasiswa UAT melalui halaman Register. Kembali
   ke Course yang dibuat dan enroll seluruh mahasiswa melalui form enrollment.
3. Gunakan tautan **Assignment & test case**. Buat assignment untuk Course
   tersebut, isikan deskripsi soal, deadline, batas akhir keterlambatan, serta
   keenam batas sumber daya yang valid. Tambahkan sedikitnya 5 test case dan
   pastikan seluruhnya tampil setelah disimpan.
4. Masuk sebagai tiap mahasiswa melalui UI dan kirim satu berkas Python melalui
   halaman **Daftar Assignment**. Jangan memakai curl, mengubah database, atau
   membuka rute langsung.
5. Kembali sebagai dosen, buka **Rekap nilai & CSV**, tinjau setiap submission,
   lalu buka kode yang dikirim setiap mahasiswa. Cocokkan hasil otomatis dengan
   perhitungan manual untuk sedikitnya 10 submission.
6. Ubah satu expected output melalui UI, jalankan regrade, dan verifikasi skor
   yang terdampak. Pulihkan expected output yang benar, regrade kembali, dan
   pastikan skor pulih.
7. Pilih **Unduh CSV** dari rekap. Periksa header, data 10 mahasiswa, skor, dan
   nama berkas unduhan.

## Kriteria bukti

- Database harus baru dan terisolasi, misalnya `uat_run_20261025`; sebelum
  server dimulai, koneksi harus diarahkan ke database itu dan diverifikasi
  dengan `SELECT current_database()`.
- Catat jumlah intervensi terlarang secara terpisah: perubahan database manual,
  permintaan API/curl di luar UI, dan navigasi dengan mengetik URL. Target
  kriteria adalah 0.
- Catat browser, commit, data akun/role tanpa password atau token, hasil setiap
  langkah, skor otomatis/manual, dan hasil pemeriksaan CSV.
- Jika database terisolasi tidak bisa disiapkan, jangan mulai server atau
  menyatakan alur backend lulus; tulis blocker dan apa yang belum diverifikasi.

Jangan gunakan `autograder_dev`, database bersama, kredensial di dokumen, atau
token di URL.
