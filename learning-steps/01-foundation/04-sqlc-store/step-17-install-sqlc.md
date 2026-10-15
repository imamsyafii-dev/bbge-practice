# Step 17 — Menginstal sqlc

Pada Step 13–16, server sudah memiliki migration untuk membentuk schema PostgreSQL. Langkah berikutnya adalah menulis query SQL dan menghasilkan kode Go yang type-safe dari query tersebut. BBGE memakai `sqlc` untuk proses code generation ini.

BBGE mengunci sqlc pada version `v1.31.1` melalui wrapper `dev/sqlc`. Project latihan memakai version yang sama agar hasil generator dan format kode tetap konsisten dengan referensi BBGE.

## Mengenal sqlc

`sqlc` membaca tiga jenis input:

- schema database, pada latihan ini berasal dari file migration PostgreSQL;
- query SQL yang diberi nama dan annotation sqlc;
- file konfigurasi yang menentukan engine, package, dan direktori output.

Dari input tersebut, sqlc menghasilkan kode Go seperti model row, parameter query, method query, dan interface store. SQL tetap ditulis secara eksplisit; sqlc tidak mengganti SQL dengan query builder atau ORM.

`sqlc` adalah developer tool untuk code generation. Binary ini dipakai ketika source code dibuat atau diperbarui, bukan ketika server berjalan di production.

## Menginstal version yang dipakai BBGE

Masuk ke direktori server, lalu instal sqlc `v1.31.1`:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server
go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
```

Suffix `@v1.31.1` penting karena membuat instalasi reproducible. Jangan menggantinya dengan `@latest`; version terbaru di masa depan dapat menghasilkan output atau validasi yang berbeda.

`go install` mengunduh dan membangun binary `sqlc`, lalu meletakkannya di direktori binary Go. Lokasinya bergantung pada konfigurasi environment Go.

## Menemukan binary secara portable

BBGE tidak mengasumsikan bahwa binary selalu berada pada satu path tertentu. Gunakan `GOBIN` jika dikonfigurasi. Jika nilainya kosong, gunakan fallback `GOPATH/bin`:

```bash
SQLC_BIN_DIR="$(go env GOBIN)"
if [ -z "$SQLC_BIN_DIR" ]; then
  SQLC_BIN_DIR="$(go env GOPATH)/bin"
fi
```

Variable `SQLC_BIN_DIR` sekarang menunjuk ke direktori tempat `go install` menyimpan binary. Cara ini tetap bekerja ketika user memakai `GOBIN` khusus maupun konfigurasi default Go.

Periksa version menggunakan path hasil resolve, tanpa bergantung pada konfigurasi `PATH` shell:

```bash
"$SQLC_BIN_DIR/sqlc" version
```

Hasil yang diharapkan:

```text
v1.31.1
```

Verifikasi ketat berikut harus selesai tanpa output dan memiliki exit status `0`:

```bash
test "$("$SQLC_BIN_DIR/sqlc" version)" = "v1.31.1"
```

## Menambahkan binary ke PATH sesi zsh saat ini

Agar command dapat dipanggil cukup sebagai `sqlc` pada sesi terminal saat ini, tambahkan direktori hasil resolve ke awal `PATH`:

```bash
export PATH="$SQLC_BIN_DIR:$PATH"
sqlc version
```

Perubahan `export` tersebut hanya berlaku pada proses zsh saat ini dan child process yang dijalankannya. Terminal baru tidak otomatis mewarisi variable `SQLC_BIN_DIR` atau perubahan `PATH` dari sesi lama.

Jika ingin membuatnya permanen, tambahkan direktori konkret yang ditampilkan oleh `SQLC_BIN_DIR` ke konfigurasi `~/.zshrc`. Contohnya berbentuk:

```zsh
export PATH="/path/hasil/resolusi/bin:$PATH"
```

Jangan menyalin placeholder tersebut secara literal. Periksa path sebenarnya lebih dahulu dengan:

```bash
printf '%s\n' "$SQLC_BIN_DIR"
```

Mengubah `~/.zshrc` bersifat opsional dan berada di luar repository latihan. Hindari menambahkan baris yang sama berulang kali. Jika tidak ingin mengubah konfigurasi shell, tetap gunakan `"$SQLC_BIN_DIR/sqlc"` pada setiap command atau gunakan wrapper project yang akan dibuat pada langkah berikutnya.

## Perbedaan go install dan go get

Gunakan `go install package@version` untuk memasang executable developer tool:

```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
```

Command tersebut membangun binary secara terpisah dari module aplikasi. Karena version diberikan pada command, instalasi tidak menambahkan sqlc ke dependency runtime server.

Jangan memakai command berikut untuk Step 17:

```bash
go get github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
```

`go get` dipakai untuk mengubah dependency module saat ini. Menambahkan implementation sqlc ke `go.mod` akan mencampurkan developer tool dengan dependency aplikasi, padahal server tidak mengimpor atau menjalankan package sqlc pada runtime.

## Memastikan scope repository tetap bersih

Instalasi Step 17 tidak mengubah file project berikut:

- `go.mod` dan `go.sum`;
- source code Go;
- migration atau schema PostgreSQL;
- data di container PostgreSQL;
- perilaku startup, health endpoint, dan graceful shutdown server.

Binary sqlc berada di `GOBIN` atau `GOPATH/bin`, yaitu di luar repository. Binary tersebut tidak ikut di-commit.

Pada Step 17, satu-satunya file repository yang perlu di-commit adalah note pembelajaran ini:

```text
learning-steps/01-foundation/04-sqlc-store/step-17-install-sqlc.md
```

## Belum menjalankan generate

Jangan menjalankan `sqlc generate` pada Step 17. Project latihan belum memiliki:

- `sqlc.yaml`;
- direktori dan file query di `db/queries`;
- konfigurasi package serta direktori output generator.

Tanpa unit konfigurasi yang lengkap, sqlc belum memiliki input yang valid untuk menghasilkan kode. Keberhasilan Step 17 hanya dibuktikan oleh output version `v1.31.1`.

Pada Step 18, kita akan menambahkan unit generation yang valid: konfigurasi sqlc, schema migration yang sudah ada, query SQL pertama, serta tujuan output Go. Setelah seluruh input tersedia, barulah `sqlc generate` dijalankan dan hasilnya diverifikasi.
