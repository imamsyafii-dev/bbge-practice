# Step 8 — Menginstal pgx

Masuk ke direktori server, lalu instal `pgx` versi yang sama dengan `bbge-biz/srv-server` dan `bbge-biz/srv-ledger`, yaitu `v5.8.0`:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server
go get github.com/jackc/pgx/v5@v5.8.0
```

`pgx` adalah driver dan toolkit PostgreSQL untuk Go. Package `pgxpool`, yang tersedia di dalam module `pgx/v5`, menyediakan connection pool agar aplikasi dapat memakai kembali beberapa koneksi database secara aman dan efisien. Kita akan mengimpor dan memakai `pgxpool` mulai Step 9; tidak perlu menjalankan `go get` terpisah untuk package tersebut.

Verifikasi dependency yang ditambahkan ke `go.mod`:

```bash
grep -E 'pgx|pgpassfile|pgservicefile|golang.org/x/text' go.mod
```

Hasil yang diharapkan:

```text
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.8.0 // indirect
	golang.org/x/text v0.29.0 // indirect
```

`pgpassfile`, `pgservicefile`, dan `golang.org/x/text` adalah dependency transitif yang dibutuhkan oleh `pgx`. Entri checksum-nya juga ditambahkan ke `go.sum` secara otomatis.

Semua baris masih memiliki penanda `// indirect` karena source code server belum mengimpor `pgx` atau `pgxpool` secara langsung. Hal ini normal pada Step 8. Setelah `pgxpool` diimpor pada Step 9, perintah Go berikutnya dapat memindahkan `github.com/jackc/pgx/v5` menjadi dependency langsung.

Pastikan project tetap lulus pengujian:

```bash
go test ./...
```

Jika berhasil, perintah selesai tanpa error; package yang belum memiliki test biasanya ditampilkan dengan status `[no test files]`.

> **Jangan jalankan `go mod tidy` pada Step 8.** Karena belum ada source code yang mengimpor `pgx`, `go mod tidy` dapat menghapus dependency yang baru ditambahkan. Tunggu sampai Step 9 selesai mengimpor dan memakai `pgxpool`.
