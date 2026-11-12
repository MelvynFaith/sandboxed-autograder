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
| Login | Tab email → password → tombol masuk; Shift+Tab kembali | PARTIAL | Managed browser, mock, 360 px: email/password/tombol difokuskan dan Enter masuk ke dashboard. Fokus terlihat pada kontrol yang diperiksa; audit Shift+Tab lintas seluruh rute/perangkat belum lengkap. |
| Register | Tab melewati semua field dan tombol submit | PASS (alur maju) | Managed browser, mock: Nama → Email → Password → Konfirmasi password → Daftar → Login. CSS fokus pada field menampilkan outline 1 px warna `rgb(229, 151, 0)`. Pengiriman pendaftaran belum dijalankan. |
| Upload submission | Tab ke file input, Enter membuka chooser, pilih berkas, Enter untuk upload | PARTIAL | Mahasiswa, mock: Tab menjangkau input file; Enter memicu event file chooser. Berkas uji sintetis dipilih di browser dan Enter pada Upload menghasilkan “Kode berhasil diunggah.” Pemilihan di dialog OS native belum diverifikasi. |
| Course | Tab melalui form Course dan tombol simpan | PARTIAL | Dosen, mock: urutan Nama → Kode → Term → Simpan Course teramati. Submit Course dengan keyboard belum dikonfirmasi; tidak ada Course yang dibuat dalam pemeriksaan ini. |
| Enrollment | Tab melalui field dan tombol enrollment | PASS (alur dan validasi) | Dosen, mock: Email mahasiswa → Tambah enrollment; Enter saat form kosong menampilkan validasi dan tidak mengubah data. |
| Assignment | Tab melalui field assignment dan tombol aksi | PARTIAL | Dosen, mock: alur fokus dimulai di Judul dan mencapai kontrol Deadline. Tab sesudah input tanggal native tidak terukur di managed browser; submit assignment tetap belum diverifikasi. |
| Resource limit | Tab melalui semua field batas dan tombol simpan | PASS (alur fokus sebelumnya) | Dosen, mock: keenam spinbutton dan tombol Buat assignment dapat difokuskan berurutan. Submit kosong hanya menampilkan validasi. Pengukuran ulang kali ini terhenti pada keterbatasan fokus input tanggal native. |
| Test case | Tab melalui field, checkbox tersembunyi, dan tombol aksi | PASS (alur fokus) | Dosen, mock: ID eksternal → Input → Keluaran → Bobot → Mode pencocokan → checkbox tersembunyi → Tambah test case. Tidak ada test case yang dibuat. |
| Recap / CSV | Tab melalui tabel dan aktifkan ekspor CSV | PARTIAL | Enter pada Unduh CSV menampilkan notifikasi sukses dengan nama `rekap-assignment-1.csv`; Tab menjangkau tautan aksi untuk baris mahasiswa. Pengambilan byte/isi file dan penelusuran penuh semua baris belum diverifikasi. |
| Admin role | Tab ke kontrol perubahan role dan aktifkan dengan Enter/Space | PARTIAL | Select role memiliki nama aksesibel; Space lalu Escape mempertahankan nilai `STUDENT`. Kontrol memiliki outline fokus 1 px. Perubahan role sengaja tidak dikirim dan penelusuran keyboard seluruh panel belum lengkap. |
| Dialog hapus | Fokus awal ada di dialog; Tab/Shift+Tab tidak keluar dari dialog | PASS (managed browser) | Fokus awal Batal; Tab menuju Hapus dan Shift+Tab kembali ke Batal. |
| Dialog hapus | Escape menutup dialog dan fokus kembali ke pemicu | PASS (managed browser) | Escape menutup dialog dan fokus kembali ke tombol Hapus pemicu. |
| Regrade-all | Tab tetap di konfirmasi; Escape menutupnya dan fokus kembali | PASS (managed browser) | Fokus awal Batal; Shift+Tab/Tab berputar antara tombol konfirmasi dan Batal; Escape menutup dan memulihkan fokus ke Nilai ulang semua. |

## Kriteria

- Fokus terlihat jelas pada setiap tautan, tombol, dan field.
- Semua kontrol dapat dijangkau dan diaktifkan dengan keyboard.
- Dialog modal menahan fokus; Tab dan Shift+Tab berputar di dalam dialog.
- Setelah dialog ditutup, fokus kembali ke kontrol yang membukanya.
- Follow-up 2026-11-12 memakai managed browser dan mock. Halaman Course,
  pengelolaan assignment, rekap, dan admin tidak menunjukkan overflow dokumen
  pada viewport 360/768/1280 px; halaman login/register dan daftar assignment
  juga tidak overflow pada pengukuran yang dilakukan. Lebar `document` sama
  dengan viewport efektif browser pada pemeriksaan tersebut.
- Pemeriksaan ini bukan pengganti semua input keyboard manual pada browser
  normal. Date picker native menghentikan pengukuran Tab di managed browser;
  pemilihan berkas melalui dialog OS dan pengiriman CSV masih memerlukan cek
  browser/perangkat pengguna.
- `PARTIAL` dan `BELUM DIVERIFIKASI` bukan hasil lulus NFR-11. Lengkapi
  pemeriksaan yang tersisa dan nilai tampilan fokus secara visual sebelum
  menyatakan seluruh navigasi keyboard lulus.
