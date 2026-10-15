# Step 34 — Menguji error wrapping pada migration

Step ini melanjutkan **database boundary testing**. Pada Step 33, kita baru menguji guard untuk dependency `nil`. Sekarang kita membuat batas kecil di sekitar `goose.Up` agar kegagalan migration dapat diuji tanpa PostgreSQL dan tanpa Docker.

API public tetap sama. Perubahan hanya menambahkan type function dan helper private yang memungkinkan test mengirim fake function secara langsung.

## Tujuan

Setelah menyelesaikan step ini, kita dapat:

- memahami alur `Migrate -> runMigrations -> goose.Up`;
- membuat function type yang cocok dengan signature function library;
- memahami parameter variadic `...goose.OptionsFunc`;
- menerapkan dependency injection melalui parameter function;
- membedakan local dependency injection dari mutable package-global test seam;
- memakai fake function untuk memeriksa argument yang diteruskan;
- menambahkan context pada error dengan `%w`;
- membuktikan error asli tetap dapat ditemukan dengan `errors.Is`;
- memeriksa prefix pesan error tanpa mengunci seluruh pesan;
- menguji kegagalan migration tanpa database aktif.

## Prasyarat

Step 1 sampai Step 33 sudah selesai. File berikut sudah tersedia:

```text
srv-server/db/migrate.go
srv-server/db/migrate_test.go
```

Dependency Goose juga sudah ada di module:

```go
github.com/pressly/goose/v3
```

Jalankan perintah dari root module server:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server
```

## File yang diubah

```text
srv-server/db/migrate.go
srv-server/db/migrate_test.go
```

Dokumentasi offline step ini dibuat di:

```text
learning-steps/01-foundation/11-database-boundary-testing/step-34-test-migration-error-wrapping.md
```

## Isi lengkap akhir `db/migrate.go`

```go
package db

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

type gooseUpFunc func(
	*sql.DB,
	string,
	...goose.OptionsFunc,
) error

func AutoMigrate(
	pool *pgxpool.Pool,
) error {
	if pool == nil {
		return fmt.Errorf("postgres pool is nil")
	}

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()

	return Migrate(sqlDB)
}

func Migrate(
	database *sql.DB,
) error {
	if database == nil {
		return fmt.Errorf("sql database is nil")
	}

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf(
			"set goose dialect: %w",
			err,
		)
	}

	goose.SetBaseFS(migrations)

	return runMigrations(
		database,
		goose.Up,
	)
}

func runMigrations(
	database *sql.DB,
	up gooseUpFunc,
) error {
	if err := up(
		database,
		"migrations",
	); err != nil {
		return fmt.Errorf(
			"run migrations: %w",
			err,
		)
	}

	return nil
}
```

Blok di atas adalah isi lengkap `db/migrate.go` setelah `gofmt`.

## Isi lengkap akhir `db/migrate_test.go`

```go
package db

import (
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestAutoMigrateRejectsNilPool(
	t *testing.T,
) {
	err := AutoMigrate(nil)
	if err == nil {
		t.Fatal(
			"AutoMigrate(nil) error = nil, want non-nil",
		)
	}

	const want = "postgres pool is nil"

	if got := err.Error(); got != want {
		t.Fatalf(
			"AutoMigrate(nil) error = %q, want %q",
			got,
			want,
		)
	}
}

func TestMigrateRejectsNilDatabase(
	t *testing.T,
) {
	err := Migrate(nil)
	if err == nil {
		t.Fatal(
			"Migrate(nil) error = nil, want non-nil",
		)
	}

	const want = "sql database is nil"

	if got := err.Error(); got != want {
		t.Fatalf(
			"Migrate(nil) error = %q, want %q",
			got,
			want,
		)
	}
}

func TestRunMigrationsWrapsUpError(
	t *testing.T,
) {
	database := &sql.DB{}

	upErr := errors.New(
		"practice migration failure",
	)

	called := false

	fakeUp := func(
		gotDatabase *sql.DB,
		gotDirectory string,
		options ...goose.OptionsFunc,
	) error {
		called = true

		if gotDatabase != database {
			t.Errorf(
				"database = %p, want %p",
				gotDatabase,
				database,
			)
		}

		const wantDirectory = "migrations"

		if gotDirectory != wantDirectory {
			t.Errorf(
				"directory = %q, want %q",
				gotDirectory,
				wantDirectory,
			)
		}

		if len(options) != 0 {
			t.Errorf(
				"options count = %d, want 0",
				len(options),
			)
		}

		return upErr
	}

	err := runMigrations(
		database,
		fakeUp,
	)

	if !called {
		t.Fatal("up was not called")
	}

	if err == nil {
		t.Fatal(
			"runMigrations() error = nil, want non-nil",
		)
	}

	if !errors.Is(err, upErr) {
		t.Fatalf(
			"errors.Is(error, upErr) = false; error = %v",
			err,
		)
	}

	const wantPrefix = "run migrations: "

	if got := err.Error(); !strings.HasPrefix(
		got,
		wantPrefix,
	) {
		t.Fatalf(
			"error = %q, want prefix %q",
			got,
			wantPrefix,
		)
	}
}
```

Blok di atas adalah isi lengkap `db/migrate_test.go` setelah `gofmt`. Dua test nil guard dari Step 33 tetap dipertahankan, lalu test error wrapping ditambahkan di bawahnya.

## Perubahan alur migration

Sebelum Step 34, `Migrate` memanggil `goose.Up` secara langsung. Sekarang pemanggilan tersebut melewati helper private:

```text
Migrate(database)
        ↓
validasi database != nil
        ↓
goose.SetDialect("postgres")
        ↓
goose.SetBaseFS(migrations)
        ↓
runMigrations(database, goose.Up)
        ↓
up(database, "migrations")
```

Pada production, nilai parameter `up` adalah function asli `goose.Up`. Pada unit test, nilai parameter yang sama diganti dengan `fakeUp`:

```text
runMigrations(database, fakeUp)
        ↓
fakeUp menerima argument
        ↓
fake memeriksa database, directory, dan options
        ↓
fake mengembalikan upErr
        ↓
runMigrations membungkus upErr
```

Helper ini tidak mengubah perilaku production. Ia hanya membuat batas pemanggilan external library dapat diuji secara terisolasi.

## `gooseUpFunc` dan kesesuaian signature

Kita mendefinisikan function type:

```go
type gooseUpFunc func(
	*sql.DB,
	string,
	...goose.OptionsFunc,
) error
```

Signature ini harus cocok dengan function `goose.Up`:

```go
func Up(
	db *sql.DB,
	dir string,
	opts ...OptionsFunc,
) error
```

Nama parameter tidak memengaruhi kesesuaian type. Hal yang harus sama adalah:

- urutan parameter;
- type setiap parameter;
- sifat variadic pada parameter terakhir;
- jumlah dan type return value.

Karena signature-nya cocok, function asli dapat diteruskan sebagai value:

```go
runMigrations(database, goose.Up)
```

Fake function dalam test juga dapat diteruskan karena memiliki signature yang sama:

```go
runMigrations(database, fakeUp)
```

Ini disebut **function assignability**: sebuah function value dapat dipakai sebagai value dari named function type jika underlying signature-nya identik.

## Mengapa parameter variadic harus ikut ditulis

Parameter terakhir berbentuk:

```go
...goose.OptionsFunc
```

Tiga titik berarti pemanggil boleh mengirim nol atau lebih option:

```go
up(database, "migrations")
up(database, "migrations", optionA)
up(database, "migrations", optionA, optionB)
```

Di dalam implementation function, parameter itu diterima sebagai slice:

```go
options []goose.OptionsFunc
```

Namun pada declaration signature, `...goose.OptionsFunc` berbeda dari parameter biasa `[]goose.OptionsFunc`. Jika `gooseUpFunc` ditulis memakai slice biasa, `goose.Up` tidak assignable ke type tersebut.

Step ini memanggil `up` tanpa option:

```go
up(database, "migrations")
```

Karena itu fake memeriksa:

```go
if len(options) != 0 {
```

Assertion ini akan menangkap perubahan jika nanti helper tanpa sengaja mulai mengirim option tambahan.

## Dependency injection melalui parameter function

`runMigrations` menerima dependency sebagai parameter lokal:

```go
func runMigrations(
	database *sql.DB,
	up gooseUpFunc,
) error
```

Pada production:

```go
runMigrations(database, goose.Up)
```

Pada test:

```go
runMigrations(database, fakeUp)
```

Kita tidak membuat interface karena dependency yang dibutuhkan hanya satu operasi function. Function type sudah cukup kecil dan eksplisit untuk kasus ini.

Parameter juga membuat dependency terlihat langsung pada signature. Pembaca dapat mengetahui bahwa `runMigrations` tidak memilih implementation sendiri; pemanggillah yang memberikan operasi `up`.

## Mengapa tidak memakai mutable package-global seam

Alternatif yang sering terlihat adalah:

```go
var gooseUp = goose.Up
```

Kemudian test menggantinya sementara:

```go
original := gooseUp
gooseUp = fakeUp
defer func() {
	gooseUp = original
}()
```

Pendekatan itu membuat mutable state di tingkat package. Risikonya:

- test yang berjalan bersamaan dapat melihat function yang sedang diganti;
- lupa mengembalikan value global dapat memengaruhi test berikutnya;
- `t.Parallel()` menjadi berbahaya;
- race detector dapat menemukan read/write concurrent;
- dependency tersembunyi karena tidak tampak pada parameter function.

Dengan local parameter injection, setiap pemanggilan membawa dependency sendiri:

```text
call A -> fakeUpA
call B -> fakeUpB
```

Tidak ada variable global yang perlu dimutasi. Karena itu test lebih terisolasi dan lebih aman terhadap eksekusi concurrent.

## Apa yang diperiksa oleh fake

Fake tidak hanya mengembalikan error. Ia juga memeriksa kontrak interaksi helper.

### 1. Function benar-benar dipanggil

```go
called := false
```

Fake mengubahnya saat dieksekusi:

```go
called = true
```

Setelah helper kembali, test memeriksa:

```go
if !called {
	t.Fatal("up was not called")
}
```

Tanpa assertion ini, implementation yang langsung mengembalikan error tanpa memanggil dependency mungkin masih terlihat lulus pada sebagian assertion lain.

### 2. Pointer database diteruskan tanpa diganti

```go
if gotDatabase != database {
```

Perbandingan pointer memastikan helper meneruskan object yang sama persis kepada `up`, bukan membuat atau memilih handle database lain.

### 3. Direktori migration benar

```go
const wantDirectory = "migrations"
```

String ini harus sesuai dengan lokasi file di embedded filesystem. Typo pada path akan ditangkap oleh unit test sebelum menjadi kegagalan saat startup.

### 4. Tidak ada option tambahan

```go
if len(options) != 0 {
```

Kontrak Step 34 adalah memanggil Goose dengan database dan direktori saja. Assertion ini mendokumentasikan sekaligus mengunci kontrak tersebut.

### 5. Error fake diteruskan sebagai penyebab

```go
return upErr
```

Error buatan yang stabil ini memungkinkan test membuktikan bahwa helper memberi context tanpa kehilangan identity error awal.

## Mengapa menggunakan `t.Errorf` di dalam fake

Assertion argument di dalam fake memakai `t.Errorf`, bukan `t.Fatalf`:

```go
t.Errorf(...)
```

`t.Errorf` menandai test gagal tetapi membiarkan fake menyelesaikan eksekusi dan mengembalikan `upErr`. Dengan demikian assertion setelah pemanggilan masih dapat memeriksa error wrapping.

Untuk kondisi yang membuat kelanjutan test tidak aman, `t.Fatal` atau `t.Fatalf` lebih tepat. Pada pemeriksaan pointer, directory, dan jumlah option di sini, test masih aman untuk diteruskan.

## Mengapa `&sql.DB{}` aman hanya pada helper fake

Test membuat:

```go
database := &sql.DB{}
```

Ini adalah pointer non-`nil` menuju zero-value `sql.DB`. Dalam Step 34, object tersebut aman karena:

- hanya dipakai sebagai identity pointer;
- hanya diteruskan ke `runMigrations`;
- `runMigrations` memanggil `fakeUp`;
- `fakeUp` tidak melakukan query atau memanggil method database.

Alurnya berhenti sepenuhnya di kode test:

```text
&sql.DB{}
   ↓
runMigrations
   ↓
fakeUp
   ↓
perbandingan pointer saja
```

Jangan memberikan zero-value tersebut ke `Migrate`:

```go
Migrate(&sql.DB{}) // jangan dipakai sebagai pengganti koneksi nyata
```

`Migrate` akan melewati nil guard, mengatur Goose, lalu akhirnya mencoba memakai `sql.DB` tersebut untuk operasi database. Zero-value yang dibuat manual bukan handle koneksi yang dibuka melalui driver. Hasilnya tidak mewakili unit test yang stabil dan dapat menimbulkan kegagalan internal yang tidak relevan.

Jadi keamanannya bukan karena `&sql.DB{}` selalu aman. Ia aman pada test ini karena fake menjamin object tidak pernah digunakan sebagai koneksi.

## Error wrapping dengan `%w`

Helper memberi context saat `up` gagal:

```go
return fmt.Errorf(
	"run migrations: %w",
	err,
)
```

`%w` melakukan dua hal:

1. menambahkan context yang berguna bagi manusia;
2. mempertahankan error asal di dalam error chain.

Jika fake mengembalikan:

```text
practice migration failure
```

error luarnya akan terlihat seperti:

```text
run migrations: practice migration failure
```

Context `run migrations` menjelaskan operasi yang gagal. Error awal tetap tersedia untuk pemeriksaan programmatic.

Jika `%w` diganti menjadi `%v`, string mungkin masih terlihat benar, tetapi hubungan error chain hilang. Akibatnya `errors.Is` tidak dapat menemukan `upErr`.

## Mengapa memakai `errors.Is`

Test tidak membandingkan object error luar secara langsung:

```go
if err == upErr
```

Perbandingan tersebut akan `false` karena `fmt.Errorf` membuat wrapper error baru. Test memakai:

```go
if !errors.Is(err, upErr) {
```

`errors.Is` berjalan menelusuri error chain melalui mekanisme `Unwrap`. Karena helper memakai `%w`, `upErr` ditemukan di dalam wrapper.

Assertion ini membuktikan bahwa caller tingkat atas masih dapat mengenali penyebab aslinya walaupun setiap layer menambahkan context.

## Mengapa prefix pesan diperiksa terpisah

Selain identity error, test memeriksa context manusia:

```go
const wantPrefix = "run migrations: "
```

Lalu:

```go
strings.HasPrefix(got, wantPrefix)
```

Kedua assertion mempunyai tujuan berbeda:

- `errors.Is(err, upErr)` memeriksa struktur error chain;
- `strings.HasPrefix` memeriksa bahwa layer migration menambahkan context yang tepat.

Test tidak membandingkan seluruh string secara exact karena bagian setelah prefix berasal dari dependency. Jika pesan error dependency berubah, kontrak context `run migrations: ` masih dapat diuji tanpa mengunci seluruh presentasi string.

## Goose mempunyai state global

`Migrate` memanggil:

```go
goose.SetDialect("postgres")
goose.SetBaseFS(migrations)
```

Kedua setter tersebut mengatur state pada package Goose. Karena itu, memanggil beberapa konfigurasi Goose yang berbeda secara concurrent dalam satu process perlu kehati-hatian.

Test baru tidak memanggil `Migrate`. Ia langsung menguji helper:

```go
runMigrations(database, fakeUp)
```

Akibatnya test error wrapping:

- tidak mengubah dialect global Goose;
- tidak mengubah base filesystem global Goose;
- tidak memanggil `goose.Up` asli;
- tidak perlu melakukan cleanup state Goose.

Dalam application production saat ini, `Migrate` dijalankan sekali pada startup sebelum server mulai menerima request. Pola startup sekali tersebut membatasi penggunaan global setter ke fase initialization. Jika di masa depan migration dijalankan parallel, multi-database, atau berulang dengan konfigurasi berbeda dalam process yang sama, state global Goose perlu ditinjau kembali.

## Mengapa PostgreSQL dan Docker tidak diperlukan

Test baru menggunakan fake function sehingga tidak melakukan operasi eksternal:

```text
unit test
   ↓
runMigrations
   ↓
fakeUp
   ↓
error buatan
```

Test tidak:

- membuka connection pool;
- menghubungi `127.0.0.1:25432`;
- menjalankan container `practice-postgres`;
- membaca atau mengubah schema PostgreSQL;
- menjalankan SQL migration melalui Goose;
- membuat row pada `goose_db_version`.

Ini tetap unit test. Keberhasilan migration SQL pada database nyata adalah tanggung jawab integration test terpisah.

## Memformat dan menjalankan test

Format kedua file:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server

gofmt -w db/migrate.go db/migrate_test.go
```

Jalankan tiga test database boundary:

```bash
go test ./db \
  -run '^(TestAutoMigrateRejectsNilPool|TestMigrateRejectsNilDatabase|TestRunMigrationsWrapsUpError)$' \
  -count=1 \
  -v
```

Arti option yang digunakan:

- `./db` memilih package migration;
- `-run` memilih tepat tiga nama test;
- `-count=1` memaksa test dieksekusi ulang, bukan memakai cache;
- `-v` menampilkan nama dan hasil setiap test.

Setelah focused test lulus, jalankan seluruh test module:

```bash
go test ./...
```

## Hasil aktual yang diamati

Focused test menghasilkan:

```text
=== RUN   TestAutoMigrateRejectsNilPool
--- PASS: TestAutoMigrateRejectsNilPool (0.00s)
=== RUN   TestMigrateRejectsNilDatabase
--- PASS: TestMigrateRejectsNilDatabase (0.00s)
=== RUN   TestRunMigrationsWrapsUpError
--- PASS: TestRunMigrationsWrapsUpError (0.00s)
PASS
ok  	github.com/imamsyafii-dev/bbge-practice/srv-server/db	0.010s
```

Full test menghasilkan:

```text
ok  	github.com/imamsyafii-dev/bbge-practice/srv-server	(cached)
ok  	github.com/imamsyafii-dev/bbge-practice/srv-server/config	(cached)
ok  	github.com/imamsyafii-dev/bbge-practice/srv-server/db	(cached)
ok  	github.com/imamsyafii-dev/bbge-practice/srv-server/internal/app/account	(cached)
ok  	github.com/imamsyafii-dev/bbge-practice/srv-server/internal/domain/account	(cached)
ok  	github.com/imamsyafii-dev/bbge-practice/srv-server/internal/router	(cached)
ok  	github.com/imamsyafii-dev/bbge-practice/srv-server/internal/store	(cached)
?   	github.com/imamsyafii-dev/bbge-practice/srv-server/pkg/postgres	[no test files]
```

Label `(cached)` pada full test berarti Go memakai hasil valid dari package yang source dan dependency-nya tidak berubah sejak eksekusi sebelumnya. Focused test memakai `-count=1`, sehingga ketiga test benar-benar dijalankan ulang.

## Batas cakupan Step 34

Step ini membuktikan bahwa:

- helper memanggil dependency migration;
- database pointer yang sama diteruskan;
- directory `migrations` diteruskan;
- tidak ada Goose option tambahan;
- error diberi context `run migrations: `;
- error asal tetap dapat ditemukan melalui `errors.Is`.

Step ini belum membuktikan bahwa:

- connection PostgreSQL dapat dibuka;
- embedded SQL file valid untuk PostgreSQL;
- Goose dapat membuat atau membaca `goose_db_version`;
- migration benar-benar mengubah schema;
- transaction dan rollback migration bekerja;
- migration bersifat idempotent pada database nyata;
- `goose.Up` sukses ketika tidak ada migration baru.

Semua hal tersebut membutuhkan integration test atau verifikasi terhadap PostgreSQL nyata.

## Troubleshooting

### `cannot use goose.Up as gooseUpFunc`

Periksa signature `gooseUpFunc`. Parameter terakhir harus tetap variadic:

```go
...goose.OptionsFunc
```

Jangan menggantinya dengan:

```go
[]goose.OptionsFunc
```

### `cannot use fakeUp as gooseUpFunc`

Pastikan fake memiliki parameter dan return value yang sama:

```go
func(
	*sql.DB,
	string,
	...goose.OptionsFunc,
) error
```

Nama parameter boleh berbeda, tetapi type, urutan, variadic marker, dan return value harus cocok.

### `errors.Is(error, upErr) = false`

Pastikan wrapping memakai `%w`:

```go
fmt.Errorf("run migrations: %w", err)
```

Jika memakai `%v`, pesan akan terlihat mirip tetapi error chain tidak dipertahankan.

### Test panic saat memakai `&sql.DB{}`

Pastikan zero-value hanya dikirim ke `runMigrations` bersama `fakeUp`. Jangan mengirimkannya ke `Migrate` atau `goose.Up` asli.

### Test mencoba terhubung ke PostgreSQL

Pastikan test memanggil:

```go
runMigrations(database, fakeUp)
```

bukan:

```go
Migrate(database)
```

### `up was not called`

Pastikan `runMigrations` memanggil parameter `up`, bukan memanggil `goose.Up` secara hard-coded:

```go
if err := up(database, "migrations"); err != nil {
```

### Perintah test mengatakan `go.mod file not found`

Jalankan perintah dari root module:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server
```

## Ringkasan

Pada Step 34 kita menambahkan seam kecil di sekitar `goose.Up` melalui function type dan parameter injection. Production tetap memakai `goose.Up`, sedangkan test memakai `fakeUp` yang dapat memeriksa argument dan menghasilkan error secara deterministic.

Pola utamanya adalah:

```text
external function
       ↓ diberikan sebagai parameter
small private helper
       ↓ diuji dengan fake
error diberi context memakai %w
       ↓
penyebab tetap dikenali dengan errors.Is
```

Pendekatan ini menjaga unit test cepat, tidak membutuhkan Docker atau PostgreSQL, dan menghindari mutable package-global yang rawan race.

## Preview Step 35

Step 35 adalah **verifikasi akhir milestone fondasi**. Kita akan meninjau struktur server, memastikan generated code dan seluruh test konsisten, menjalankan server dengan dependency practice, lalu memverifikasi endpoint utama dari luar process.

Verifikasi akhir tersebut akan membedakan:

- unit test yang sudah melindungi setiap layer;
- integration dengan PostgreSQL dan migration nyata;
- wiring application saat startup;
- HTTP health dan account flow;
- graceful shutdown;
- kesiapan fondasi sebelum masuk ke kelompok pembelajaran berikutnya.
