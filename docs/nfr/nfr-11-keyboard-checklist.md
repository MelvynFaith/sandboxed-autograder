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
| Login | Tab email → password → tombol masuk; Shift+Tab kembali |  |  |
| Register | Tab melewati semua field dan tombol submit |  |  |
| Upload submission | Tab dari file input ke tombol submit dan cancel |  |  |
| Course | Tab melalui form Course dan tombol simpan |  |  |
| Enrollment | Tab melalui field dan tombol enrollment |  |  |
| Assignment | Tab melalui field assignment dan tombol aksi |  |  |
| Resource limit | Tab melalui semua field batas dan tombol simpan |  |  |
| Test case | Tab melalui field, checkbox tersembunyi, dan tombol aksi |  |  |
| Recap / CSV | Tab melalui tabel, filter, dan tombol ekspor CSV |  |  |
| Admin role | Tab ke kontrol perubahan role dan aktifkan dengan Enter/Space |  |  |
| Dialog hapus | Fokus awal ada di dialog; Tab/Shift+Tab tidak keluar dari dialog |  |  |
| Dialog hapus | Escape menutup dialog dan fokus kembali ke pemicu |  |  |
| Regrade-all | Tab tetap di konfirmasi; Escape menutupnya dan fokus kembali |  |  |

## Kriteria

- Fokus terlihat jelas pada setiap tautan, tombol, dan field.
- Semua kontrol dapat dijangkau dan diaktifkan dengan keyboard.
- Dialog modal menahan fokus; Tab dan Shift+Tab berputar di dalam dialog.
- Setelah dialog ditutup, fokus kembali ke kontrol yang membukanya.
- Tidak ada hasil yang diisi tanpa menjalankan pemeriksaan tersebut.
