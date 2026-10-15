# Step 19 — Menambahkan Query CreateAccount

Pada Step 18, project sudah memiliki unit generation sqlc yang lengkap dan query pembacaan `GetAccountByUserID`. Sekarang kita menambahkan operasi tulis pertama: `CreateAccount`.

Query baru hanya menerima `user_id` dan `name`. Nilai lain tetap menjadi tanggung jawab schema PostgreSQL melalui identity/sequence dan default column. Setelah query ditambahkan, wrapper `dev/sqlc` dipakai kembali untuk memperbarui package `internal/store` secara konsisten dengan sqlc `v1.31.1`.

Step ini masih berfokus pada definisi query dan code generation. Method hasil generation belum dipanggil oleh runtime, sehingga langkah ini belum memasukkan row ke database.

## Memperbarui file query account

Masuk ke root server:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server
```

Isi final `db/queries/account.sql` harus memuat query pembacaan dari Step 18 dan query pembuatan account yang baru:

```sql
-- name: GetAccountByUserID :one
SELECT
    id,
    user_id,
    name,
    status,
    version,
    created_at,
    updated_at
FROM account
WHERE user_id = $1
LIMIT 1;

-- name: CreateAccount :one
INSERT INTO account (
    user_id,
    name
) VALUES (
    $1,
    $2
)
RETURNING
    id,
    user_id,
    name,
    status,
    version,
    created_at,
    updated_at;
```

Simpan file dengan satu newline setelah titik koma terakhir. Final newline mengikuti konvensi file teks repository, menjaga output terminal tetap rapi, dan menghindari marker `No newline at end of file` pada diff.

## Memahami annotation CreateAccount

Annotation berikut menjadi kontrak antara SQL dan API Go yang dihasilkan:

```sql
-- name: CreateAccount :one
```

`CreateAccount` menjadi nama method Go. Suffix `:one` menyatakan bahwa statement mengembalikan satu row. Karena query memakai `RETURNING`, sqlc menghasilkan method dengan return value `Account`, bukan hanya jumlah row yang berubah.

Jika query insert tidak memiliki `RETURNING`, annotation yang sesuai biasanya `:exec` dan method tidak dapat langsung mengembalikan account hasil insert. Pada latihan ini, caller membutuhkan ID, nilai default, dan timestamp yang dipilih PostgreSQL, sehingga `:one` dan `RETURNING` dipakai bersama.

## Input positional dan parameter Go

Bagian insert mendeklarasikan dua input secara eksplisit:

```sql
INSERT INTO account (
    user_id,
    name
) VALUES (
    $1,
    $2
)
```

Placeholder PostgreSQL dipetakan berdasarkan posisi:

1. `$1` mengisi `user_id` dan menjadi field Go `UserID string`.
2. `$2` mengisi `name` dan menjadi field Go `Name string`.

Urutan placeholder harus sama dengan urutan kolom insert. Binding parameter tetap dilakukan oleh pgx, bukan melalui penggabungan string SQL, sehingga value dikirim terpisah dari statement.

Karena query memiliki lebih dari satu input, sqlc mengelompokkannya dalam satu struct parameter bernama `CreateAccountParams`. Caller tidak perlu mengingat urutan argumen ketika memanggil method Go; field struct membuat setiap value terlihat jelas.

## Membiarkan PostgreSQL menerapkan default

Tabel `account` memiliki tujuh kolom, tetapi insert hanya mengirim dua kolom. Lima kolom lain diperoleh dari schema:

- `id` dibuat oleh sequence milik `BIGSERIAL`;
- `status` memakai default `1`;
- `version` memakai default `0`;
- `created_at` memakai `CURRENT_TIMESTAMP`;
- `updated_at` memakai `CURRENT_TIMESTAMP`.

Query tidak mengirim nilai default tersebut dari Go. Menjaga default di database memastikan setiap jalur insert memperoleh aturan awal yang sama dan membuat parameter method tetap kecil.

Schema dan query latihan tidak memiliki kolom `email`. Identitas account tetap `user_id`; jangan menambahkan `email` hanya karena field tersebut umum ditemukan pada contoh aplikasi lain.

## Memakai RETURNING yang eksplisit

Query mengembalikan tujuh kolom dalam urutan model:

```sql
RETURNING
    id,
    user_id,
    name,
    status,
    version,
    created_at,
    updated_at;
```

Daftar eksplisit dipilih daripada `RETURNING *` karena:

- kontrak hasil query dapat dibaca langsung;
- urutan scan stabil dan terlihat jelas;
- penambahan kolom pada migration masa depan tidak otomatis memperluas hasil query;
- query tetap konsisten dengan `SELECT` eksplisit milik `GetAccountByUserID`.

Urutan kolom `RETURNING` menjadi urutan target pada `row.Scan`. Karena semua kolom account dikembalikan, method dapat mengembalikan model `Account` lengkap, termasuk nilai yang dibuat oleh database.

## Menjalankan code generation

Validasi syntax wrapper lebih dahulu:

```bash
bash -n dev/sqlc
```

Lalu jalankan generator dari root server:

```bash
./dev/sqlc
```

Wrapper memastikan binary yang dipakai adalah sqlc `v1.31.1`, berpindah ke root project, lalu menjalankan konfigurasi `sqlc.yaml`. Generation membaca migration dan query secara offline; container PostgreSQL tidak harus berjalan.

Output akhir yang diharapkan:

```text
sqlc generation completed
```

## Parameter yang dihasilkan

Di `internal/store/account.sql.go`, sqlc menghasilkan struct berikut:

```go
type CreateAccountParams struct {
	UserID string `json:"user_id"`
	Name   string `json:"name"`
}
```

Tag JSON berasal dari `emit_json_tags: true` pada `sqlc.yaml`. Struct ini hanya berisi dua input yang benar-benar dikirim oleh query. `ID`, `Status`, `Version`, `CreatedAt`, dan `UpdatedAt` bukan parameter karena nilainya berasal dari PostgreSQL.

## Method CreateAccount yang dihasilkan

Signature method baru berbentuk:

```go
func (q *Queries) CreateAccount(
	ctx context.Context,
	arg CreateAccountParams,
) (Account, error)
```

Implementasinya memakai dua field parameter saat memanggil `QueryRow`:

```go
row := q.db.QueryRow(ctx, createAccount, arg.UserID, arg.Name)
```

Kemudian sqlc memindai hasil `RETURNING` ke model `Account` dalam urutan yang sama:

```go
err := row.Scan(
	&i.ID,
	&i.UserID,
	&i.Name,
	&i.Status,
	&i.Version,
	&i.CreatedAt,
	&i.UpdatedAt,
)
```

Snippet ini ditampilkan untuk memahami output generator. Jangan menyalin atau mengeditnya secara manual di generated file.

## Interface Querier yang diperbarui

Karena `emit_interface: true`, `internal/store/querier.go` sekarang memuat kedua operasi account:

```go
type Querier interface {
	CreateAccount(ctx context.Context, arg CreateAccountParams) (Account, error)
	GetAccountByUserID(ctx context.Context, userID string) (Account, error)
}
```

Interface tersebut memungkinkan service bergantung pada kontrak query tanpa harus bergantung langsung pada concrete `*Queries`.

## File generated yang berubah dan tetap sama

Generation Step 19 memperbarui dua file:

- `internal/store/account.sql.go` memperoleh constant SQL, `CreateAccountParams`, dan method `CreateAccount`;
- `internal/store/querier.go` memperoleh method `CreateAccount` pada interface.

Dua file lain tetap sama secara semantik:

- `internal/store/models.go` tetap memiliki model `Account` tujuh field karena schema tidak berubah;
- `internal/store/db.go` tetap memiliki `DBTX`, `Queries`, `New`, dan `WithTx` karena infrastruktur eksekusi query tidak berubah.

Semua generated file harus tetap memiliki header:

```go
// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.31.1
```

Generated code tetap di-commit agar build tidak memerlukan sqlc dan perubahan API dapat direview bersama query sumbernya. Namun, generated code tidak pernah diedit manual. Untuk mengubah API, edit migration, query, atau konfigurasi yang menjadi input, lalu jalankan `./dev/sqlc` kembali.

## Memverifikasi output dengan grep

Pastikan parameter, method concrete, dan method interface sudah dihasilkan:

```bash
grep -n 'type CreateAccountParams struct' internal/store/account.sql.go
grep -n 'func (q \*Queries) CreateAccount' internal/store/account.sql.go
grep -n 'CreateAccount(ctx context.Context' internal/store/querier.go
```

Periksa version generator pada semua generated file:

```bash
grep -R 'sqlc v1\.31\.1' internal/store
```

Pastikan `email` tidak masuk ke query maupun generated store:

```bash
! grep -R -i 'email' db/queries internal/store
```

Operator `!` membalik exit status `grep`: verifikasi berhasil ketika pencarian tidak menemukan `email`.

## Memastikan project tetap dapat dikompilasi

Jalankan test seluruh package:

```bash
go test ./...
```

Command ini memastikan query hasil generation, model, pgx, dan module server tetap cocok. `go test ./...` tidak menjalankan `main.go`, tidak memanggil migration runner, dan tidak mengeksekusi `CreateAccount` terhadap PostgreSQL.

## Memastikan generation deterministik

Generation pertama setelah mengubah query memang harus memperbarui output. Setelah output Step 19 terbentuk, hitung aggregate hash di memory, jalankan generation kedua tanpa perubahan input, lalu bandingkan hash:

```bash
before_hash="$(shasum internal/store/*.go | shasum | awk '{print $1}')"
./dev/sqlc
after_hash="$(shasum internal/store/*.go | shasum | awk '{print $1}')"
printf 'before: %s\nafter:  %s\n' "$before_hash" "$after_hash"
test "$before_hash" = "$after_hash"
```

Nilai `before` dan `after` harus sama, dan command `test` harus selesai dengan exit status `0`. Kesamaan aggregate hash membuktikan generation kedua tidak mengubah isi generated file.

Prosedur ini tidak bergantung pada status tracking Git dan tidak membuat file sementara. Karena itu, pemeriksaan tetap bekerja ketika generated file masih untracked.

## Batas scope Step 19

Step ini hanya menambahkan named query dan memperbarui generated store. Tidak ada perubahan pada:

- migration atau schema PostgreSQL;
- konfigurasi koneksi database;
- lifecycle startup dan graceful shutdown;
- endpoint HTTP;
- pemanggilan query dari runtime;
- data di tabel `account`.

`CreateAccount` baru tersedia sebagai API Go; belum ada code pada runtime yang memanggilnya. Menjalankan `./dev/sqlc` dan `go test ./...` juga tidak melakukan insert database.

Pada Step 20, kita akan menulis PostgreSQL integration test yang memanggil `CreateAccount` lalu `GetAccountByUserID`, memeriksa nilai default database, dan me-rollback transaction agar data test tidak tertinggal.
