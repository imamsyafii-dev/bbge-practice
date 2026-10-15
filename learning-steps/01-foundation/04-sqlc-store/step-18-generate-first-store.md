# Step 18 — Menghasilkan Store Pertama dengan sqlc

Pada Step 17, binary sqlc `v1.31.1` sudah tersedia sebagai developer tool, tetapi project belum memiliki unit generation yang lengkap. Pada langkah ini, kita menambahkan konfigurasi sqlc, query account pertama, dan wrapper project yang menjaga version generator tetap konsisten. Setelah itu, sqlc menghasilkan package Go `internal/store` dari migration dan query SQL.

Langkah ini hanya melakukan code generation. Server belum memanggil query yang dihasilkan, tidak membuka koneksi tambahan, dan tidak mengubah schema maupun data PostgreSQL.

## Struktur file

Setelah langkah ini selesai, bagian yang relevan di `srv-server` berbentuk:

```text
srv-server/
├── sqlc.yaml
├── dev/
│   └── sqlc
├── db/
│   ├── migrations/
│   │   ├── 0001_create_trigger_function.sql
│   │   └── 0002_create_account.sql
│   └── queries/
│       └── account.sql
└── internal/
    └── store/
        ├── account.sql.go
        ├── db.go
        ├── models.go
        └── querier.go
```

Tiga file pertama—`sqlc.yaml`, `dev/sqlc`, dan `db/queries/account.sql`—ditulis oleh developer. Empat file di `internal/store` dibuat oleh sqlc dan tidak diedit secara manual.

## Menambahkan konfigurasi sqlc

Masuk ke direktori server, lalu buat direktori input dan wrapper jika belum ada:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server
mkdir -p db/queries dev
touch sqlc.yaml
```

Isi `sqlc.yaml` dengan konfigurasi berikut:

```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "db/queries"
    schema: "db/migrations"
    gen:
      go:
        package: "store"
        out: "internal/store"
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_prepared_queries: false
        emit_interface: true
        emit_exact_table_names: false
        emit_empty_slices: true
```

Setiap bagian memiliki fungsi khusus:

- `version: "2"` memilih version 2 dari format konfigurasi sqlc. Ini bukan version binary sqlc dan bukan version migration database.
- `sql` berisi satu atau lebih unit generation. Project latihan baru membutuhkan satu unit PostgreSQL.
- `engine: "postgresql"` membuat parser dan type mapping mengikuti PostgreSQL.
- `queries: "db/queries"` memberi tahu sqlc lokasi query aplikasi yang diberi nama.
- `schema: "db/migrations"` memakai kumpulan migration sebagai sumber struktur tabel. sqlc membaca schema secara statis; command generate tidak menjalankan migration ke database.
- `package: "store"` menentukan nama package Go hasil generation.
- `out: "internal/store"` menempatkan output di package internal agar hanya code di dalam module server yang dapat mengimpornya.
- `sql_package: "pgx/v5"` menghasilkan API yang kompatibel dengan driver pgx version 5 yang sudah dipakai server.
- `emit_json_tags: true` menambahkan tag seperti `json:"user_id"` pada field model.
- `emit_prepared_queries: false` membuat method memakai `QueryRow` atau `Query` langsung tanpa menghasilkan lifecycle prepared statement tambahan.
- `emit_interface: true` menghasilkan interface `Querier`, berguna untuk dependency injection dan test.
- `emit_exact_table_names: false` mengizinkan sqlc mengubah nama tabel menjadi nama type Go yang idiomatis.
- `emit_empty_slices: true` membuat query banyak-row mengembalikan slice kosong ketika tidak ada row, bukan `nil`. Query pertama memakai `:one`, tetapi flag ini menjaga perilaku query list pada langkah berikutnya tetap konsisten.

Konfigurasi ini mengikuti unit generation BBGE, tetapi schema dan query latihan tetap sengaja lebih kecil.

## Menulis query account pertama

Buat file query:

```bash
touch db/queries/account.sql
```

Isi `db/queries/account.sql` dengan:

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
```

Baris annotation berikut dibaca oleh sqlc, bukan PostgreSQL:

```sql
-- name: GetAccountByUserID :one
```

`GetAccountByUserID` menjadi nama method Go. Annotation `:one` menyatakan bahwa query harus menghasilkan tepat satu row untuk dipindai ke satu nilai `Account`. Jika row tidak ditemukan, method mengembalikan error dari pgx; method ini tidak mengembalikan slice.

Placeholder `$1` adalah parameter positional PostgreSQL pertama. Dari kondisi berikut, sqlc menyimpulkan parameter tersebut bertipe sama dengan `account.user_id`, yaitu string Go:

```sql
WHERE user_id = $1
```

Kolom ditulis secara eksplisit dan urut, bukan memakai `SELECT *`. Dengan demikian, kontrak hasil query terlihat langsung dan tidak berubah diam-diam ketika migration masa depan menambah kolom baru. Urutan yang sama juga dipakai oleh kode hasil generation ketika memanggil `row.Scan`.

Query hanya memakai tujuh kolom dari schema latihan: `id`, `user_id`, `name`, `status`, `version`, `created_at`, dan `updated_at`. Tidak ada kolom `email`; schema account latihan memang tidak mendefinisikannya.

## Menambahkan wrapper project

Buat wrapper berikut:

```bash
touch dev/sqlc
```

Isi `dev/sqlc` dengan:

```bash
#!/usr/bin/env bash
set -euo pipefail

SQLC_VERSION="v1.31.1"
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd -- "${SCRIPT_DIR}/.." && pwd)"

if ! command -v go >/dev/null 2>&1; then
  echo "Go is required to install sqlc" >&2
  exit 1
fi

SQLC_BIN_DIR="$(go env GOBIN)"
if [[ -z "${SQLC_BIN_DIR}" ]]; then
  GOPATH_VALUE="$(go env GOPATH)"
  SQLC_BIN_DIR="${GOPATH_VALUE%%:*}/bin"
fi

SQLC_BIN="${SQLC_BIN_DIR}/sqlc"

CURRENT_VERSION=""
if [[ -x "${SQLC_BIN}" ]]; then
  CURRENT_VERSION="$("${SQLC_BIN}" version 2>/dev/null || true)"
fi

if [[ "${CURRENT_VERSION}" != "${SQLC_VERSION}" ]]; then
  echo "Installing sqlc ${SQLC_VERSION}..."
  go install "github.com/sqlc-dev/sqlc/cmd/sqlc@${SQLC_VERSION}"
fi

ACTUAL_VERSION="$("${SQLC_BIN}" version)"
if [[ "${ACTUAL_VERSION}" != "${SQLC_VERSION}" ]]; then
  echo "sqlc version check failed: expected ${SQLC_VERSION}, got ${ACTUAL_VERSION}" >&2
  exit 1
fi

cd "${PROJECT_DIR}"
"${SQLC_BIN}" generate -f sqlc.yaml

echo "sqlc generation completed"
```

Jadikan wrapper executable:

```bash
chmod +x dev/sqlc
```

Wrapper memberi beberapa jaminan:

- `#!/usr/bin/env bash` memilih Bash dari environment user.
- `set -euo pipefail` menghentikan script ketika command gagal, variable belum didefinisikan, atau bagian pipeline gagal.
- `SQLC_VERSION="v1.31.1"` mengunci generator pada version yang sama dengan BBGE.
- `SCRIPT_DIR` dihitung dari lokasi file wrapper, bukan current working directory pemanggil.
- `PROJECT_DIR` adalah parent dari `dev`, yaitu root `srv-server`.
- pemeriksaan `command -v go` memberikan error yang jelas sebelum mencoba instalasi.
- wrapper mencari binary di `GOBIN`. Jika `GOBIN` kosong, wrapper memakai entri pertama `GOPATH` dan menambahkan `/bin`.
- path binary dipanggil secara langsung, sehingga generation tidak bergantung pada apakah direktori binary Go sudah masuk `PATH`.
- jika binary belum ada atau version-nya berbeda, wrapper memasang tepat `sqlc@v1.31.1` dengan `go install`.
- version diperiksa lagi setelah instalasi. Generation hanya dilanjutkan jika hasilnya tepat `v1.31.1`.
- `cd "${PROJECT_DIR}"` membuat path relatif di `sqlc.yaml` selalu di-resolve dari root server, walaupun wrapper dipanggil dari direktori lain.
- `generate -f sqlc.yaml` menyatakan file konfigurasi secara eksplisit.

Wrapper ini adalah developer tooling. Ia tidak dipanggil oleh `main.go` dan tidak menjadi dependency runtime server.

## Konvensi final newline

Simpan `sqlc.yaml`, `db/queries/account.sql`, dan `dev/sqlc` dengan satu newline setelah baris terakhir. Parser tetap dapat membaca file tanpa final newline, tetapi newline akhir adalah konvensi file teks repository, mencegah output terminal menyatu dengan baris terakhir, dan menghindari marker `No newline at end of file` pada diff.

## Memeriksa wrapper sebelum generation

Validasi syntax Bash tanpa menjalankan instalasi atau generation:

```bash
bash -n dev/sqlc
```

Command yang berhasil tidak menghasilkan output dan memiliki exit status `0`.

Periksa permission wrapper:

```bash
test -x dev/sqlc
```

Command tersebut juga harus selesai tanpa output. Jika gagal, jalankan kembali `chmod +x dev/sqlc`.

## Menjalankan generation pertama

Dari root server, jalankan:

```bash
./dev/sqlc
```

Jika binary belum ada atau version lokal berbeda, wrapper lebih dahulu menampilkan proses instalasi. Akhir eksekusi yang berhasil menampilkan:

```text
sqlc generation completed
```

Generation bersifat offline terhadap database: sqlc membaca file migration dan query dari repository. Container PostgreSQL tidak harus berjalan, dan command ini tidak membuat tabel, menerapkan migration, atau membaca data account.

## Memahami empat file hasil generation

sqlc menghasilkan tepat empat file untuk unit kecil ini:

- `internal/store/db.go` mendefinisikan `DBTX`, `Queries`, constructor `New`, dan helper `WithTx`.
- `internal/store/models.go` mendefinisikan model `Account` berdasarkan tabel pada migration.
- `internal/store/querier.go` mendefinisikan interface `Querier` dari seluruh named query.
- `internal/store/account.sql.go` menyimpan SQL dan implementasi method `GetAccountByUserID`.

Header setiap file menunjukkan ownership dan version generator:

```go
// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.31.1
```

Model penting yang dihasilkan berbentuk:

```go
type Account struct {
	ID        int64            `json:"id"`
	UserID    string           `json:"user_id"`
	Name      string           `json:"name"`
	Status    int16            `json:"status"`
	Version   int64            `json:"version"`
	CreatedAt pgtype.Timestamp `json:"created_at"`
	UpdatedAt pgtype.Timestamp `json:"updated_at"`
}
```

Mapping tersebut berasal dari schema PostgreSQL: `BIGSERIAL` dan `BIGINT` menjadi `int64`, `VARCHAR NOT NULL` menjadi `string`, `SMALLINT` menjadi `int16`, dan `TIMESTAMP NOT NULL` menjadi `pgtype.Timestamp`. Tidak ada field `Email` karena kolom itu tidak ada pada migration atau query latihan.

API query yang dihasilkan berbentuk:

```go
func (q *Queries) GetAccountByUserID(
	ctx context.Context,
	userID string,
) (Account, error)
```

Interface hasil `emit_interface: true` memuat kontrak yang sama:

```go
type Querier interface {
	GetAccountByUserID(ctx context.Context, userID string) (Account, error)
}
```

`db.go` menyediakan constructor dan dukungan transaksi:

```go
func New(db DBTX) *Queries
func (q *Queries) WithTx(tx pgx.Tx) *Queries
```

Snippet di atas dipakai untuk memahami hasil generator, bukan sebagai instruksi untuk mengetik file generated secara manual.

## Generated code tetap di-commit

Empat file di `internal/store` harus di-commit bersama konfigurasi dan query. Dengan demikian, build aplikasi tidak perlu menginstal atau menjalankan sqlc lebih dahulu, dan reviewer dapat melihat perubahan API store bersama perubahan SQL yang menyebabkannya.

Namun, file generated tidak boleh diedit manual. Jika schema, query, atau konfigurasi berubah, perbarui input tersebut lalu jalankan kembali:

```bash
./dev/sqlc
```

Perubahan manual pada generated file akan hilang saat generation berikutnya dan dapat membuat source tidak lagi cocok dengan query aslinya.

## Memverifikasi hasil

Pastikan empat file yang diharapkan tersedia:

```bash
test -f internal/store/account.sql.go
test -f internal/store/db.go
test -f internal/store/models.go
test -f internal/store/querier.go
```

Periksa header version dan pastikan tidak ada field email:

```bash
grep -R 'sqlc v1\.31\.1' internal/store
! grep -R -i 'email' db/queries internal/store
```

Lalu compile dan test seluruh package:

```bash
go test ./...
```

`go test ./...` memastikan generated package cocok dengan dependency pgx dan module server. Test ini tidak menjalankan `main`, tidak memanggil migration runner, dan tidak mengubah database.

## Memastikan generation deterministik

Hitung aggregate hash seluruh generated file, jalankan wrapper untuk kedua kalinya tanpa mengubah input, lalu hitung hash kembali:

```bash
before_hash="$(shasum internal/store/*.go | shasum | awk '{print $1}')"
./dev/sqlc
after_hash="$(shasum internal/store/*.go | shasum | awk '{print $1}')"
printf 'before: %s\nafter:  %s\n' "$before_hash" "$after_hash"
test "$before_hash" = "$after_hash"
```

Nilai `before` dan `after` harus sama, dan command `test` harus selesai dengan exit status `0`. Kesamaan aggregate hash berarti isi empat generated file tidak berubah pada generation kedua, sehingga hasil generation deterministik. Prosedur ini bekerja walaupun file masih untracked dan tidak membuat file sementara.

Hasil deterministik bergantung pada tiga hal yang sudah dijaga: input schema dan query yang sama, konfigurasi yang sama, serta binary sqlc yang dikunci pada `v1.31.1`.

## Batas scope Step 18

Step ini menambahkan input dan output code generation saja. Tidak ada perubahan pada:

- perilaku runtime `main.go`;
- startup, health endpoint, timeout, atau graceful shutdown;
- koneksi database dan migration runner;
- schema atau data PostgreSQL;
- `go.mod` dan `go.sum`; wrapper memasang binary dengan `go install package@version`, sehingga sqlc tidak menjadi dependency aplikasi;
- API HTTP maupun pemanggilan store dari handler.

Method yang tersedia baru pembacaan `GetAccountByUserID`; method tersebut juga belum dipanggil oleh runtime. Pada Step 19, kita akan menambahkan named query `CreateAccount`, menjalankan generation ulang, dan mempelajari parameter untuk operasi insert.
