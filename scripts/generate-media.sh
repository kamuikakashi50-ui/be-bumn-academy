#!/usr/bin/env bash
# Membuat aset video pelajaran ruang kelas (MP4 + poster) memakai Pillow + ffmpeg.
#
# Video yang dihasilkan adalah berkas MP4 asli (H.264 + faststart) yang di-host
# sendiri oleh backend pada /api/v1/media/lessons/*.mp4, sehingga pemutar video di
# front-end benar-benar memutar berkas nyata — termasuk permintaan HTTP Range saat
# peserta melompat ke tengah video.
#
# Pemakaian:
#   scripts/generate-media.sh            # buat video yang belum ada
#   FORCE=1 scripts/generate-media.sh    # buat ulang semuanya
#
# Prasyarat (hanya untuk membuat aset):
#   - ffmpeg dengan encoder libx264
#   - python3 + Pillow  (pip3 install pillow)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="$ROOT/media/lessons"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

if ! command -v ffmpeg >/dev/null 2>&1; then
  echo "ffmpeg tidak ditemukan. Pasang lebih dulu (mis. brew install ffmpeg)." >&2
  exit 1
fi
if ! python3 -c 'import PIL' >/dev/null 2>&1; then
  echo "Pillow belum terpasang. Jalankan: pip3 install pillow" >&2
  exit 1
fi

mkdir -p "$OUT"

cat > "$TMP/spec.json" <<'JSON'
[
  {
    "name": "numerik-deret",
    "badge": "Penalaran Numerik",
    "slides": [
      ["Deret Angka & Pola", "Modul 1 · Penalaran Numerik"],
      ["Cara membaca soal", "Kenali lebih dulu jenis deretnya: aritmetika, geometri, atau campuran.", "Cek selisih antar suku, lalu rasio bila selisih tidak tetap."],
      ["Latihan", "3, 7, 15, 31, ... tentukan dua suku berikutnya.", "Kerjakan di ruang kelas lalu bandingkan dengan pembahasan."]
    ]
  },
  {
    "name": "numerik-perbandingan",
    "badge": "Penalaran Numerik",
    "slides": [
      ["Perbandingan & Persentase", "Modul 2 · Penalaran Numerik"],
      ["Cara membaca soal", "Terjemahkan soal cerita menjadi perbandingan senilai atau berbalik nilai.", "Gunakan faktor skala agar hitungan tetap ringkas."],
      ["Latihan", "Jika 5 pekerja menyelesaikan proyek dalam 8 hari, berapa hari untuk 8 pekerja?", "Tuliskan langkahnya, lalu cocokkan dengan pembahasan."]
    ]
  },
  {
    "name": "verbal-analogi",
    "badge": "Penalaran Verbal",
    "slides": [
      ["Analogi & Sinonim", "Modul 1 · Penalaran Verbal"],
      ["Cara membaca soal", "Tentukan hubungan kata pada pasangan contoh lebih dulu.", "Hubungan bisa berupa fungsi, sifat, bagian, atau tingkatan."],
      ["Latihan", "MANDIRI : SWATANTRA = ...", "Pilih pasangan dengan hubungan yang sama persis."]
    ]
  },
  {
    "name": "analitis-silogisme",
    "badge": "Logika Analitis",
    "slides": [
      ["Silogisme & Penarikan Kesimpulan", "Modul 1 · Logika Analitis"],
      ["Cara membaca soal", "Gambarkan premis sebagai diagram himpunan sederhana.", "Kesimpulan sah hanya mengikuti kedua premis, bukan pengetahuan umum."],
      ["Latihan", "Semua A adalah B. Sebagian B adalah C. Apa yang pasti benar?", "Uji tiap pilihan jawaban pada diagram."]
    ]
  },
  {
    "name": "akhlaq-studi-kasus",
    "badge": "Nilai AKHLAK",
    "slides": [
      ["Nilai AKHLAK: Studi Kasus", "Modul 1 · Nilai AKHLAK & Wawasan"],
      ["Cara membaca soal", "Enam nilai inti: Amanah, Kompeten, Harmonis, Loyal, Adaptif, Kolaboratif.", "Pilih nilai yang paling langsung tercermin pada situasi soal."],
      ["Latihan", "Pegawai melaporkan kelebihan pembayaran vendor — nilai apa yang tampak?", "Bandingkan dua nilai terdekat sebelum menjawab."]
    ]
  },
  {
    "name": "inggris-reading",
    "badge": "Bahasa Inggris",
    "slides": [
      ["Reading Comprehension: Strategi Cepat", "Modul 1 · Bahasa Inggris"],
      ["Cara membaca soal", "Baca pertanyaan lebih dulu, lalu cari kata kunci pada teks.", "Waspadai pilihan yang benar sebagian atau terlalu luas."],
      ["Latihan", "Temukan gagasan utama paragraf ketiga.", "Tandai kalimat topik sebelum memilih jawaban."]
    ]
  }
]
JSON

echo "Merender slide…"
python3 "$ROOT/scripts/render_media_slides.py" "$TMP/frames" "$TMP/spec.json"

echo "Mengemas video…"
for name in numerik-deret numerik-perbandingan verbal-analogi analitis-silogisme akhlaq-studi-kasus inggris-reading; do
  target="$OUT/$name.mp4"

  if [ -f "$target" ] && [ "${FORCE:-0}" != "1" ]; then
    echo "  • $name.mp4 sudah ada (lewati)"
  else
    # Tiga slide masing-masing 8 detik, digabung dengan filter concat sehingga
    # durasi berkas tepat 24 detik (tanpa bergantung kuirk concat demuxer).
    ffmpeg -hide_banner -loglevel error -y \
      -loop 1 -t 8 -i "$TMP/frames/$name-1.png" \
      -loop 1 -t 8 -i "$TMP/frames/$name-2.png" \
      -loop 1 -t 8 -i "$TMP/frames/$name-3.png" \
      -filter_complex "[0:v][1:v][2:v]concat=n=3:v=1:a=0,fps=24,format=yuv420p[v]" \
      -map "[v]" -c:v libx264 -preset veryfast -crf 30 -movflags +faststart -an \
      "$target"
    echo "  ✓ $name.mp4 ($(du -h "$target" | cut -f1))"
  fi

  cp -f "$TMP/frames/$name-poster.jpg" "$OUT/$name-poster.jpg"
done

echo "Selesai. Berkas di $OUT: $(ls -1 "$OUT" | wc -l | tr -d ' ')"
