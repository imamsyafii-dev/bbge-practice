# Step 13 — Menginstal Goose untuk Database Migration

Pada Step 5–12, server sudah memiliki PostgreSQL, connection pool, startup timeout, dan graceful shutdown. Struktur database belum dikelola oleh aplikasi. Mulai langkah ini, perubahan schema PostgreSQL akan dicatat sebagai migration yang dapat dijalankan secara berurutan dan konsisten.

BBGE memakai module `github.com/pressly/goose/v3` versi `v3.27.0`. Project latihan memakai module dan versi yang sama agar pola migration yang dipelajari tetap dekat dengan implementasi BBGE.

## Mengenal schema migration

Schema migration adalah perubahan struktur database yang disimpan sebagai file dan diberi urutan. Contohnya meliputi:

- membuat atau menghapus tabel;
- menambah atau mengubah kolom;
- membuat index dan constraint;
- menyimpan langkah `up` untuk menerapkan perubahan;
- menyimpan langkah `down` untuk membatalkan perubahan ketika rollback memang didukung.

Migration membuat perubahan schema dapat ditinjau bersama source code dan diterapkan dengan urutan yang sama pada environment berbeda. Aplikasi tidak perlu mengandalkan perubahan manual melalui database client.

Pada tahap ini, kita hanya menambahkan dependency Goose. Belum ada file migration yang dibuat dan belum ada perubahan yang dijalankan terhadap PostgreSQL.

## Library dan CLI Goose

Goose menyediakan dua cara penggunaan yang berbeda:

1. **Library Go** melalui import `github.com/pressly/goose/v3`. Aplikasi dapat memanggil fungsi seperti `goose.SetDialect`, `goose.SetBaseFS`, dan `goose.Up` dari source code.
2. **Command-line interface** melalui package `github.com/pressly/goose/v3/cmd/goose`. CLI menyediakan command seperti `goose up`, `goose down`, dan `goose status` dari terminal.

BBGE memakai Goose sebagai library yang terhubung ke startup aplikasi. Step 13 karena itu menambahkan module library ke `go.mod`; perintah ini bukan instalasi binary CLI. Tidak perlu menjalankan `go install github.com/pressly/goose/v3/cmd/goose` untuk mengikuti alur latihan ini.

## Menambahkan dependency Goose

Masuk ke direktori server, lalu tambahkan Goose dengan versi yang dipakai BBGE:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server
go get github.com/pressly/goose/v3@v3.27.0
```

Verifikasi versi module yang dipilih oleh Go:

```bash
go list -m github.com/pressly/goose/v3
```

Hasil yang diharapkan:

```text
github.com/pressly/goose/v3 v3.27.0
```

Kemudian jalankan test project:

```bash
go test ./...
```

Jika semua checksum dependency sudah tersedia, command selesai tanpa error. Package yang belum memiliki test biasanya ditampilkan dengan status `[no test files]`.

## Perubahan pada go.mod dan go.sum

`go get` memperbarui dua file dependency:

- `go.mod` mencatat module dan versi yang dipilih oleh project.
- `go.sum` mencatat checksum module agar Go dapat memverifikasi isi dependency yang diunduh.

Pada akhir Step 13, entri Goose di `go.mod` masih ditandai `// indirect`:

```text
github.com/pressly/goose/v3 v3.27.0 // indirect
```

Penanda tersebut benar untuk tahap ini karena belum ada source code project yang mengimpor Goose. Setelah langkah berikutnya membuat migration runner dan mengimpor `github.com/pressly/goose/v3`, Goose akan menjadi dependency langsung.

Goose juga membawa dependency transitif. Perubahan yang umum terlihat setelah `go get` antara lain:

- penambahan `github.com/mfridman/interpolate`;
- penambahan `github.com/sethvargo/go-retry`;
- penambahan `go.uber.org/multierr`;
- pembaruan versi terpilih `golang.org/x/sync`;
- pembaruan versi terpilih `golang.org/x/text`.

`go.sum` dapat berisi lebih dari satu versi module, termasuk checksum versi lama yang masih tersisa. Hal ini normal. Tidak perlu menyalin, menulis, atau menghapus checksum secara manual.

> **Jangan jalankan `go mod tidy` pada Step 13.** Karena source code belum mengimpor Goose, `go mod tidy` dapat menghapus dependency yang baru ditambahkan. Jalankan perintah tersebut setelah migration runner memakai Goose pada langkah berikutnya.

## Memperbaiki checksum golang.org/x/text jika diperlukan

Pada instalasi latihan ini, `go get` memilih `golang.org/x/text v0.34.0`, tetapi awalnya hanya checksum file `go.mod` untuk versi tersebut yang tersedia. Akibatnya, `go test ./...` dapat berhenti dengan pesan `missing go.sum entry` untuk package dari `golang.org/x/text`.

Jika error tersebut muncul, unduh module yang tepat untuk menambahkan checksum kontennya:

```bash
go mod download golang.org/x/text@v0.34.0
```

Kemudian jalankan test kembali:

```bash
go test ./...
```

Perintah recovery menggunakan versi `v0.34.0` secara eksplisit agar sesuai dengan versi yang sudah dipilih di `go.mod`. Jangan memperbaiki masalah checksum dengan mengedit `go.sum` secara manual.

## Memastikan runtime belum berubah

Step 13 hanya mengubah metadata dependency pada `go.mod` dan `go.sum`. Tidak ada source code yang berubah, Goose belum dipanggil, dan belum ada migration yang dijalankan. Karena itu:

- startup server tetap memakai alur dari Step 12;
- health endpoint tetap merespons seperti sebelumnya;
- schema database PostgreSQL tetap sama;
- mengulang `go run .` belum membuat tabel migration.

Langkah berikutnya akan membuat migration runner yang mengimpor Goose, menyematkan file SQL ke dalam binary, dan menjalankan migration melalui connection pool PostgreSQL yang sudah tersedia.
