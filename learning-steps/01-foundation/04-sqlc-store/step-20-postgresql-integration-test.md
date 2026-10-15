# Step 20 — Integration Test PostgreSQL untuk Store Account

Pada Step 19, sqlc sudah menghasilkan method `CreateAccount` dan `GetAccountByUserID`. Namun, `go test ./...` saat itu baru membuktikan bahwa source code dapat dikompilasi. Belum ada test yang benar-benar mengirim query ke PostgreSQL.

Step ini menambahkan integration test untuk membuktikan bahwa generated store cocok dengan database nyata. Test akan:

1. membuka koneksi ke PostgreSQL practice;
2. memulai transaction;
3. membuat account dengan `CreateAccount`;
4. memeriksa nilai input dan default database;
5. mengambil row yang sama dengan `GetAccountByUserID`;
6. me-rollback transaction;
7. membuktikan dari koneksi di luar transaction bahwa row tidak tersimpan.

Tidak ada endpoint HTTP, domain rule, atau use case pada step ini. Fokusnya hanya kontrak antara generated store dan PostgreSQL.

## Prasyarat database

Masuk ke root server:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server
```

Pastikan container PostgreSQL practice berjalan dan sehat:

```bash
docker compose -f docker/docker-compose.yml up -d practice-postgres
docker compose -f docker/docker-compose.yml ps
```

Container yang dipakai harus bernama `practice-postgres`, dengan port host default `25432`. Status health yang diharapkan adalah `healthy`.

Integration test ini **tidak** menjalankan `db.AutoMigrate`. Database harus sudah memiliki migration sampai version `2`, yaitu function trigger dari `0001_create_trigger_function.sql` dan tabel account dari `0002_create_account.sql`.

Version migration dapat diperiksa dengan:

```bash
docker exec practice-postgres \
  psql \
  -U practice_user \
  -d practice_server \
  -c 'SELECT version_id, is_applied FROM goose_db_version ORDER BY id;'
```

Jika migration belum diterapkan, jalankan server sekali agar startup migration berjalan, lalu hentikan server dengan `Ctrl+C`:

```bash
go run .
```

## Membuat file integration test

Buat file:

```text
internal/store/account_postgres_test.go
```

Isi final file tersebut adalah:

```go
package store_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/imamsyafii-dev/bbge-practice/srv-server/config"
	"github.com/imamsyafii-dev/bbge-practice/srv-server/internal/store"
	"github.com/imamsyafii-dev/bbge-practice/srv-server/pkg/postgres"
	"github.com/jackc/pgx/v5"
)

func TestAccountQueriesAgainstPostgres(t *testing.T) {
	if os.Getenv("PRACTICE_RUN_POSTGRES_TESTS") != "1" {
		t.Skip(
			"set PRACTICE_RUN_POSTGRES_TESTS=1 to run PostgreSQL integration tests",
		)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	cfg := config.Load()

	pool, err := postgres.Open(
		ctx,
		cfg.Postgres,
	)
	if err != nil {
		t.Fatalf(
			"open PostgreSQL: %v",
			err,
		)
	}
	t.Cleanup(pool.Close)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf(
			"begin transaction: %v",
			err,
		)
	}

	defer func() {
		rollbackCtx, cancelRollback := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancelRollback()

		rollbackErr := tx.Rollback(rollbackCtx)
		if rollbackErr != nil &&
			!errors.Is(rollbackErr, pgx.ErrTxClosed) {
			t.Errorf(
				"rollback transaction during cleanup: %v",
				rollbackErr,
			)
		}
	}()

	queries := store.New(tx)

	input := store.CreateAccountParams{
		UserID: fmt.Sprintf(
			"practice-store-test-%d",
			time.Now().UnixNano(),
		),
		Name: "Practice Store Test",
	}

	created, err := queries.CreateAccount(
		ctx,
		input,
	)
	if err != nil {
		t.Fatalf(
			"create account: %v",
			err,
		)
	}

	if created.ID <= 0 {
		t.Fatalf(
			"created ID = %d, want positive value",
			created.ID,
		)
	}

	if created.UserID != input.UserID {
		t.Fatalf(
			"created user_id = %q, want %q",
			created.UserID,
			input.UserID,
		)
	}

	if created.Name != input.Name {
		t.Fatalf(
			"created name = %q, want %q",
			created.Name,
			input.Name,
		)
	}

	if created.Status != 1 {
		t.Fatalf(
			"created status = %d, want 1",
			created.Status,
		)
	}

	if created.Version != 0 {
		t.Fatalf(
			"created version = %d, want 0",
			created.Version,
		)
	}

	if !created.CreatedAt.Valid ||
		created.CreatedAt.Time.IsZero() {
		t.Fatal("created_at is not valid")
	}

	if !created.UpdatedAt.Valid ||
		created.UpdatedAt.Time.IsZero() {
		t.Fatal("updated_at is not valid")
	}

	found, err := queries.GetAccountByUserID(
		ctx,
		input.UserID,
	)
	if err != nil {
		t.Fatalf(
			"get account: %v",
			err,
		)
	}

	if found.ID != created.ID ||
		found.UserID != created.UserID ||
		found.Name != created.Name ||
		found.Status != created.Status ||
		found.Version != created.Version ||
		!found.CreatedAt.Time.Equal(created.CreatedAt.Time) ||
		!found.UpdatedAt.Time.Equal(created.UpdatedAt.Time) {
		t.Fatalf(
			"found account = %+v, want %+v",
			found,
			created,
		)
	}

	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf(
			"rollback transaction: %v",
			err,
		)
	}

	_, err = store.New(pool).GetAccountByUserID(
		ctx,
		input.UserID,
	)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"get account after rollback error = %v, want pgx.ErrNoRows",
			err,
		)
	}
}
```

Simpan file dengan satu newline setelah kurung kurawal terakhir.

## Mengapa memakai package store_test

File test memakai:

```go
package store_test
```

Ini adalah external test package. Test hanya dapat memakai API yang diekspor oleh package `store`, sama seperti package pemanggil sebenarnya. Dengan demikian, test tidak bergantung pada detail private seperti field `Queries.db`.

Nama package ini berbeda dari `package store`, tetapi tetap berada di direktori `internal/store`. Karena itu, import package berikut diperlukan:

```go
"github.com/imamsyafii-dev/bbge-practice/srv-server/internal/store"
```

## Peran setiap import

- `context` memberi batas waktu pada koneksi dan operasi database.
- `errors` memeriksa wrapped error memakai `errors.Is`.
- `fmt` membentuk `user_id` unik.
- `os` membaca flag opt-in integration test.
- `testing` menyediakan `*testing.T`.
- `time` menyediakan timeout, timestamp unik, dan pemeriksaan waktu.
- `config` membaca konfigurasi PostgreSQL yang sama dengan server.
- `store` menyediakan generated queries dan parameter.
- `postgres` membuka serta melakukan ping connection pool.
- `pgx/v5` menyediakan `pgx.ErrNoRows` dan `pgx.ErrTxClosed`.

## Integration test harus diaktifkan secara eksplisit

Bagian awal test adalah gate:

```go
if os.Getenv("PRACTICE_RUN_POSTGRES_TESTS") != "1" {
	t.Skip(
		"set PRACTICE_RUN_POSTGRES_TESTS=1 to run PostgreSQL integration tests",
	)
}
```

Tanpa environment variable bernilai tepat `1`, `t.Skip` menghentikan eksekusi test sebelum koneksi database dibuka. Ini menjaga test biasa tetap cepat dan dapat dijalankan ketika Docker tidak aktif.

Walaupun dilewati saat runtime, file integration test tetap dikompilasi oleh `go test ./...`. Artinya, import, nama type, signature generated method, dan syntax test tetap diperiksa. Gate ini hanya melewati akses PostgreSQL; gate tidak menyembunyikan compile error.

## Config default dan override

`config.Load()` menggunakan nilai default practice berikut:

```text
host:     127.0.0.1
port:     25432
user:     practice_user
password: practice_password
database: practice_server
sslmode:  disable
```

Nilainya dapat diubah melalui:

- `PRACTICE_POSTGRES_HOST`;
- `PRACTICE_POSTGRES_PORT`;
- `PRACTICE_POSTGRES_USER`;
- `PRACTICE_POSTGRES_PASSWORD`;
- `PRACTICE_POSTGRES_DB`;
- `PRACTICE_POSTGRES_SSLMODE`.

Periksa shell jika test tersambung ke database yang tidak diharapkan:

```bash
env | grep '^PRACTICE_POSTGRES_'
```

Environment variable dari shell mengalahkan nilai default. Jangan menjalankan integration test dengan konfigurasi yang menunjuk ke database BBGE utama, production, atau database yang bukan milik latihan. Test melakukan `INSERT`, meskipun row tersebut kemudian di-rollback.

## Timeout untuk seluruh operasi utama

Context utama memiliki batas waktu 10 detik:

```go
ctx, cancel := context.WithTimeout(
	context.Background(),
	10*time.Second,
)
defer cancel()
```

Context ini dipakai saat membuka pool, melakukan ping, memulai transaction, menjalankan query, melakukan rollback utama, dan memeriksa hasil rollback. Jika database tidak merespons, test berhenti dengan error alih-alih menunggu tanpa batas.

`defer cancel()` melepaskan resource timer ketika test selesai lebih cepat dari 10 detik.

## Lifecycle connection pool

Pool dibuka melalui helper aplikasi:

```go
pool, err := postgres.Open(ctx, cfg.Postgres)
```

`postgres.Open` membangun PostgreSQL URL, membuat `pgxpool.Pool`, lalu melakukan `Ping`. Karena itu, keberhasilan pemanggilan ini membuktikan database dapat dijangkau dengan credential yang diberikan.

Pool didaftarkan untuk ditutup setelah test:

```go
t.Cleanup(pool.Close)
```

`t.Cleanup` tetap dijalankan ketika test berhenti melalui `t.Fatal` atau `t.Fatalf`. Pool ditutup setelah deferred cleanup transaction selesai.

## Transaction menjaga data test tidak tertinggal

Test memulai transaction nyata:

```go
tx, err := pool.Begin(ctx)
```

Semua operasi create dan get pertama dilakukan melalui transaction yang sama. Selama belum di-commit, perubahan hanya terlihat menurut aturan transaction PostgreSQL dan dapat dibatalkan sekaligus dengan rollback.

Tidak ada pemanggilan `Commit` pada test ini. Tujuannya memang memastikan data latihan tidak tertinggal di tabel `account`.

## store.New(tx) dan kontrak DBTX

Generated constructor menerima interface `DBTX`:

```go
func New(db DBTX) *Queries
```

`DBTX` hanya membutuhkan method `Exec`, `Query`, dan `QueryRow`. `pgx.Tx` memenuhi interface tersebut, sehingga transaction dapat diberikan langsung:

```go
queries := store.New(tx)
```

Akibatnya, semua method pada `queries` memakai transaction yang sama. Constructor yang sama juga dapat menerima `*pgxpool.Pool`, seperti pada pemeriksaan setelah rollback. Inilah manfaat bergantung pada kontrak kecil: generated query tidak perlu memiliki versi terpisah untuk pool dan transaction.

## Membuat user_id yang unik dan valid

Input dibentuk dengan nanosecond timestamp:

```go
UserID: fmt.Sprintf(
	"practice-store-test-%d",
	time.Now().UnixNano(),
),
```

Tabel memiliki unique constraint pada `user_id`. Nilai dinamis menghindari benturan jika test dijalankan berulang atau ada proses test lain.

Panjang prefix `practice-store-test-` adalah 20 karakter. Nilai `UnixNano()` saat ini memiliki 19 digit, sehingga totalnya 39 karakter dan masih berada di bawah batas `VARCHAR(40)`. Jika format `user_id` diubah, panjang akhirnya harus diperiksa kembali.

## Memeriksa hasil CreateAccount

`CreateAccount` hanya menerima `UserID` dan `Name`, tetapi query memakai `RETURNING` untuk mengembalikan row lengkap. Test memeriksa:

- `ID` lebih besar dari nol karena dibuat oleh sequence `BIGSERIAL`;
- `UserID` sama dengan input;
- `Name` sama dengan input;
- `Status` bernilai default `1`;
- `Version` bernilai default `0`;
- `CreatedAt` valid dan bukan zero time;
- `UpdatedAt` valid dan bukan zero time.

Pemeriksaan `pgtype.Timestamp` membutuhkan dua hal:

```go
created.CreatedAt.Valid
!created.CreatedAt.Time.IsZero()
```

`Valid` menunjukkan PostgreSQL menghasilkan nilai non-NULL yang berhasil dipindai. `Time.IsZero()` memastikan nilainya bukan zero value Go.

Default status, version, dan timestamp tidak dibuat oleh test atau Go. Nilai itu berasal dari schema PostgreSQL, sehingga assertion ini sekaligus menguji migration `0002_create_account.sql`.

## Membaca kembali row dalam transaction yang sama

Setelah insert, test memanggil:

```go
found, err := queries.GetAccountByUserID(ctx, input.UserID)
```

Karena `queries` masih menggunakan `tx`, query dapat melihat row yang baru dibuat dalam transaction tersebut. Test membandingkan seluruh field hasil get dengan hasil create.

Timestamp dibandingkan memakai method `Time.Equal`, bukan operator `==`:

```go
found.CreatedAt.Time.Equal(created.CreatedAt.Time)
```

`time.Time.Equal` membandingkan instant waktu secara tepat tanpa bergantung pada representasi internal `time.Time`.

## Rollback eksplisit dan bukti dari luar transaction

Setelah seluruh assertion dalam transaction berhasil, test menjalankan rollback eksplisit:

```go
if err := tx.Rollback(ctx); err != nil {
	t.Fatalf("rollback transaction: %v", err)
}
```

Kemudian test membuat `Queries` baru yang memakai pool, bukan transaction yang sudah ditutup:

```go
_, err = store.New(pool).GetAccountByUserID(ctx, input.UserID)
```

Query pool berjalan di luar transaction sebelumnya. Hasil yang benar adalah:

```go
errors.Is(err, pgx.ErrNoRows)
```

Ini merupakan bukti bahwa row benar-benar dibatalkan, bukan hanya asumsi karena `Rollback` tidak mengembalikan error.

## Deferred rollback melindungi jalur gagal

Rollback eksplisit hanya tercapai jika assertion sebelumnya berhasil. Karena `t.Fatalf` langsung menghentikan goroutine test, transaction juga membutuhkan deferred cleanup yang didaftarkan segera setelah `Begin` berhasil.

Cleanup memakai context baru selama 5 detik:

```go
rollbackCtx, cancelRollback := context.WithTimeout(
	context.Background(),
	5*time.Second,
)
```

Context ini tidak diturunkan dari context utama. Jadi cleanup masih mempunyai kesempatan melakukan rollback jika context utama telah timeout atau dibatalkan.

Pada jalur sukses, rollback eksplisit sudah menutup transaction. Deferred rollback kemudian mengembalikan `pgx.ErrTxClosed`; kondisi tersebut normal dan sengaja diabaikan:

```go
!errors.Is(rollbackErr, pgx.ErrTxClosed)
```

Error cleanup selain `pgx.ErrTxClosed` dilaporkan dengan `t.Errorf`. Pada jalur gagal sebelum rollback eksplisit, deferred function mencoba membatalkan transaction agar koneksi tidak kembali ke pool dengan transaction terbuka.

## Sequence PostgreSQL tidak ikut di-rollback

Rollback menghapus efek row insert, tetapi pemanggilan sequence untuk `BIGSERIAL` tidak dibatalkan. Karena itu, nilai `account.id` dapat meloncat setiap kali integration test dijalankan.

Contohnya, test dapat menghasilkan ID `5`, rollback, lalu insert berikutnya menghasilkan ID `6`. Hilangnya angka `5` bukan kebocoran row dan bukan error. Primary key sequence menjamin keunikan, bukan nomor tanpa celah.

## Memformat source code

Jalankan:

```bash
gofmt -w internal/store/account_postgres_test.go
```

## Menjalankan test biasa

Jalankan semua test tanpa opt-in database:

```bash
go test ./...
```

Integration test dikompilasi tetapi dilewati. Tanpa flag `-v`, Go biasanya hanya menampilkan package sebagai berhasil. Untuk melihat status skip secara eksplisit:

```bash
go test ./internal/store -run '^TestAccountQueriesAgainstPostgres$' -v
```

Output penting yang diharapkan:

```text
=== RUN   TestAccountQueriesAgainstPostgres
    account_postgres_test.go:...: set PRACTICE_RUN_POSTGRES_TESTS=1 to run PostgreSQL integration tests
--- SKIP: TestAccountQueriesAgainstPostgres (...s)
PASS
```

`PASS` berarti package test berhasil; baris `SKIP` menjelaskan bahwa akses database sengaja tidak dijalankan.

## Menjalankan integration test PostgreSQL

Dengan container sehat dan migration sudah pada version `2`, jalankan:

```bash
PRACTICE_RUN_POSTGRES_TESTS=1 \
go test ./internal/store \
-run '^TestAccountQueriesAgainstPostgres$' \
-v
```

Output utama yang diharapkan:

```text
=== RUN   TestAccountQueriesAgainstPostgres
--- PASS: TestAccountQueriesAgainstPostgres (...s)
PASS
ok      github.com/imamsyafii-dev/bbge-practice/srv-server/internal/store
```

Nama package dapat diikuti durasi atau marker cache yang berbeda. Hal terpenting adalah test tersebut berstatus `PASS`, bukan `SKIP`.

## Troubleshooting

### Test masih SKIP

Pastikan flag berada pada command yang sama dan nilainya tepat `1`:

```bash
PRACTICE_RUN_POSTGRES_TESTS=1 go test ./internal/store -run '^TestAccountQueriesAgainstPostgres$' -v
```

### Connection refused pada 127.0.0.1:25432

Container belum aktif atau belum sehat. Periksa:

```bash
docker compose -f docker/docker-compose.yml ps
docker compose -f docker/docker-compose.yml logs practice-postgres
```

### Password authentication failed atau database tidak ditemukan

Periksa override environment:

```bash
env | grep '^PRACTICE_POSTGRES_'
```

Nilai default harus cocok dengan konfigurasi container practice. Jika memakai override pada Docker Compose, pakai override yang sama saat menjalankan test.

### Relation account does not exist

Database dapat dijangkau tetapi migration version `2` belum diterapkan pada database itu. Jalankan startup server sekali atau periksa `goose_db_version` seperti pada bagian prasyarat.

### Duplicate key untuk user_id

Pastikan kode memakai `time.Now().UnixNano()` dan format akhirnya tidak dipotong. Row dari run yang sukses seharusnya hilang setelah rollback. Unique value tetap dibutuhkan agar test tidak bertabrakan dengan run paralel atau data lama.

### Value too long for type character varying(40)

Format `user_id` telah diperpanjang melewati batas schema. Gunakan kembali format step ini atau pendekkan prefix sehingga hasil maksimal tidak lebih dari 40 karakter.

### get account after rollback bukan pgx.ErrNoRows

Pastikan query verifikasi dibuat dengan `store.New(pool)`, bukan memakai `queries` atau `tx` yang sudah ditutup. Jika sebuah row ditemukan, periksa apakah ada proses lain yang menulis `user_id` identik.

## File yang berubah dan tidak berubah

Step 20 hanya menambahkan satu source file:

- `internal/store/account_postgres_test.go` — integration test PostgreSQL.

Dokumentasi latihan menambahkan:

- `learning-steps/01-foundation/04-sqlc-store/step-20-postgresql-integration-test.md`.

Step ini tidak mengubah:

- `db/queries/account.sql`;
- `db/migrations/*.sql`;
- `sqlc.yaml`;
- `go.mod` dan `go.sum`;
- runtime `main.go`;
- generated file di `internal/store`.

Jangan mengedit `internal/store/account.sql.go`, `internal/store/db.go`, `internal/store/models.go`, atau `internal/store/querier.go` secara manual. Jika query atau schema perlu berubah, edit source SQL lalu jalankan `./dev/sqlc`.

## Batas scope Step 20

Integration test ini membuktikan generated SQL dan migration bekerja bersama PostgreSQL nyata. Test belum menetapkan business rule account dan belum menjadi API aplikasi.

Pada Step 21, kita akan membuat domain service account yang minimal. Service akan bergantung pada interface query yang sempit sesuai kebutuhan, bukan seluruh generated `store.Querier`. Unit test service akan memakai fake implementation agar business logic dapat diuji tanpa PostgreSQL.
