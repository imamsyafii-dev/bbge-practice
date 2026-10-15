# Step 33 — Menguji nil guard pada migration

Step ini memulai kelompok **database boundary testing**. Kita menguji perilaku paling awal pada fungsi migration: dependency database yang `nil` harus ditolak dengan error yang jelas sebelum kode mencoba memakai pgx, `database/sql`, atau Goose.

Test ini tidak membuka koneksi database, tidak menjalankan Docker, dan tidak mengubah implementation `db/migrate.go`.

## Tujuan

Setelah menyelesaikan step ini, kita dapat:

- menguji guard clause pada `AutoMigrate` dan `Migrate`;
- memastikan dependency `nil` menghasilkan error, bukan panic;
- memahami type inference ketika literal `nil` dikirim sebagai argument;
- memahami mengapa `err` harus diperiksa sebelum memanggil `err.Error()`;
- memahami kelebihan dan tradeoff perbandingan exact error message;
- memahami bahwa panic otomatis menggagalkan test Go;
- membedakan nil pointer dari zero-value object;
- mengenali batas antara unit test boundary dan integration test migration.

## Prasyarat

Step 1 sampai Step 32 sudah selesai. File `db/migrate.go` sudah memiliki dua fungsi public:

```go
func AutoMigrate(pool *pgxpool.Pool) error
func Migrate(database *sql.DB) error
```

Keduanya sudah mempunyai guard clause untuk menolak argument `nil`.

Jalankan perintah dari module server:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server
```

## File yang dibuat

```text
srv-server/db/migrate_test.go
```

Dokumentasi offline step ini berada di:

```text
learning-steps/01-foundation/11-database-boundary-testing/step-33-test-migration-nil-guards.md
```

Tidak ada perubahan pada `db/migrate.go` dalam step ini.

## Isi lengkap akhir `db/migrate_test.go`

```go
package db

import "testing"

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
```

Blok di atas adalah isi lengkap file test pada workspace setelah `gofmt`.

## Alur `AutoMigrate(nil)`

Test pertama menjalankan:

```go
err := AutoMigrate(nil)
```

Alurnya adalah:

```text
AutoMigrate(nil)
        ↓
pool == nil
        ↓
return error "postgres pool is nil"
        ↓
stdlib.OpenDBFromPool tidak dipanggil
        ↓
Migrate tidak dipanggil
```

Guard ini penting karena `AutoMigrate` biasanya menerima pool PostgreSQL yang sudah dibuat oleh application composition. Jika dependency itu tidak tersedia, fungsi berhenti di boundary dengan pesan yang lebih mudah dipahami daripada melanjutkan ke adapter dan berpotensi panic.

Test memeriksa dua kontrak:

1. error harus non-`nil`;
2. pesan error harus tepat `postgres pool is nil`.

## Alur `Migrate(nil)`

Test kedua menjalankan:

```go
err := Migrate(nil)
```

Alurnya adalah:

```text
Migrate(nil)
        ↓
database == nil
        ↓
return error "sql database is nil"
        ↓
goose.SetDialect tidak dipanggil
        ↓
goose.SetBaseFS dan goose.Up tidak dipanggil
```

Dengan berhenti sebelum operasi Goose, test ini tetap deterministic dan tidak membutuhkan PostgreSQL. Ia hanya menguji kontrak input pada boundary migration, bukan keberhasilan migration itu sendiri.

## Mengapa literal `nil` langsung dapat dipakai

Literal `nil` sendiri tidak memiliki satu tipe pointer tertentu. Compiler menentukan tipe yang dibutuhkan dari parameter fungsi pada tempat literal itu dipakai.

Pada pemanggilan:

```go
AutoMigrate(nil)
```

parameter `AutoMigrate` mengharapkan `*pgxpool.Pool`, sehingga `nil` dipahami sebagai nil pointer bertipe `*pgxpool.Pool`.

Pada pemanggilan:

```go
Migrate(nil)
```

parameter `Migrate` mengharapkan `*sql.DB`, sehingga `nil` dipahami sebagai nil pointer bertipe `*sql.DB`.

Karena konteks tipenya sudah jelas dari signature, test tidak perlu menulis variable tambahan seperti:

```go
var pool *pgxpool.Pool
var database *sql.DB
```

Kedua bentuk valid, tetapi literal `nil` lebih ringkas untuk kasus ini.

## Mengapa cek `err == nil` dilakukan lebih dahulu

Test memakai urutan:

```go
if err == nil {
	t.Fatal("...")
}

if got := err.Error(); got != want {
```

Method `Error()` hanya aman dipanggil setelah kita mengetahui `err` non-`nil`. Jika implementation salah dan mengembalikan `nil`, pemanggilan langsung berikut akan menyebabkan panic:

```go
got := err.Error()
```

Dengan assertion non-`nil` lebih dahulu, kegagalan menghasilkan pesan yang sengaja dibuat oleh test:

```text
AutoMigrate(nil) error = nil, want non-nil
```

Pesan tersebut lebih tepat daripada stack trace nil-pointer dereference. `t.Fatal` juga langsung menghentikan test saat itu, sehingga baris `err.Error()` tidak diteruskan.

## Exact error comparison dan tradeoff-nya

Test membandingkan string error secara exact:

```go
if got := err.Error(); got != want {
```

Keuntungannya:

- pesan error menjadi bagian kontrak yang terkunci;
- typo atau pesan yang terlalu umum langsung terdeteksi;
- cocok untuk guard sederhana yang saat ini membuat error tanpa wrapping;
- tidak membutuhkan package assertion tambahan.

Tradeoff-nya:

- perubahan wording yang sebenarnya tidak mengubah perilaku akan menggagalkan test;
- test terikat pada presentasi string, bukan hanya kategori error;
- jika error nantinya diberi context melalui wrapping, string lengkapnya ikut berubah.

Untuk error yang perlu dikenali secara programmatic, alternatif yang lebih stabil adalah sentinel error dan `errors.Is`. Contohnya secara konsep:

```go
var ErrNilDatabase = errors.New("sql database is nil")
```

Lalu test dapat memakai `errors.Is(err, ErrNilDatabase)`. Step ini belum menambah sentinel karena tujuannya hanya menguji kontrak source yang sudah ada, tanpa mengubah production code.

## Panic otomatis membuat test gagal

Test tidak menulis `defer`, `recover`, atau assertion khusus untuk panic. Jika `AutoMigrate(nil)` atau `Migrate(nil)` panic, test runner Go otomatis:

- menandai test sebagai gagal;
- menampilkan pesan panic;
- menampilkan stack trace lokasi panic.

Karena kontrak yang diinginkan adalah **mengembalikan error**, bukan panic, perilaku bawaan ini sudah cukup. Menambahkan `recover` justru menambah kode dan dapat menyembunyikan panic jika assertion-nya ditulis keliru.

`recover` berguna saat panic memang merupakan kontrak yang sengaja ingin diuji. Itu bukan kasus pada migration boundary ini.

## Mengapa tidak menggunakan zero-value `pgxpool.Pool`

Nil pointer berbeda dari pointer menuju object zero-value. Contoh berikut bukan input yang sama dengan `nil`:

```go
pool := &pgxpool.Pool{}
AutoMigrate(pool)
```

`pool == nil` akan bernilai `false`, sehingga fungsi melewati guard dan menyerahkan object internal yang belum diinisialisasi ke adapter pgx. Tipe `pgxpool.Pool` harus dibuat melalui API constructor pgx agar invariant internalnya valid. Membuat zero-value hanya untuk test dapat menghasilkan panic atau perilaku internal yang bukan kontrak public.

Tujuan Step 33 adalah menguji cabang `pool == nil`, jadi input paling tepat memang `nil`.

## Mengapa tidak menggunakan zero-value `sql.DB`

Hal serupa berlaku untuk:

```go
database := &sql.DB{}
Migrate(database)
```

Pointer tersebut non-`nil`, tetapi bukan handle database yang dibuka melalui `sql.Open` atau adapter yang valid. Jika diberikan ke Goose, eksekusi dapat mencapai method internal `database/sql` dengan state yang tidak mewakili koneksi nyata.

Menggunakan zero-value untuk memaksa kegagalan akan mengikat test ke detail internal library dan dapat menghasilkan panic yang tidak relevan. Itu tidak menguji nil guard `database == nil`.

Jika kelak ingin menguji kegagalan Goose tanpa database nyata, production code memerlukan seam yang terkontrol—misalnya dependency function kecil—atau memakai SQL mock yang sesuai. Hal tersebut dipisahkan ke step berikutnya agar setiap step tetap kecil.

## Mengapa Docker dan PostgreSQL tidak diperlukan

Kedua fungsi kembali tepat pada guard clause sebelum operasi eksternal terjadi:

```text
nil input
   ↓
guard clause
   ↓
error dikembalikan
```

Akibatnya test tidak:

- membaca Docker Compose;
- menghubungi port PostgreSQL `25432`;
- membuat connection pool;
- menjalankan query;
- menjalankan file SQL melalui Goose;
- mengubah schema atau data.

Ini adalah unit test cepat untuk defensive boundary. PostgreSQL aktif maupun mati tidak memengaruhi hasilnya.

## Memformat dan menjalankan test

Format file:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server
gofmt -w db/migrate_test.go
```

Jalankan hanya dua test Step 33:

```bash
go test ./db \
  -run '^(TestAutoMigrateRejectsNilPool|TestMigrateRejectsNilDatabase)$' \
  -count=1 \
  -v
```

Arti option yang dipakai:

- `./db` memilih package migration;
- `-run` memilih tepat dua nama test tersebut;
- `-count=1` memastikan hasil tidak diambil dari test cache;
- `-v` menampilkan nama dan status setiap test.

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

Label `(cached)` pada full test berarti Go menggunakan hasil valid dari test sebelumnya untuk package yang source dan dependency-nya belum berubah. Focused test dijalankan dengan `-count=1`, sehingga dua test Step 33 benar-benar dieksekusi ulang.

Durasi dan pemakaian cache dapat berbeda antar-mesin. Yang penting adalah status `PASS` dan tidak ada package yang gagal.

## Troubleshooting

### `undefined: AutoMigrate` atau `undefined: Migrate`

Pastikan `migrate_test.go` memakai:

```go
package db
```

dan berada di folder `srv-server/db`, satu package dengan `migrate.go`.

### Focused test mengatakan `[no tests to run]`

Periksa nama function dan regular expression. Nama harus tepat:

```text
TestAutoMigrateRejectsNilPool
TestMigrateRejectsNilDatabase
```

Jalankan command dari root `srv-server`, bukan dari folder workspace `bbge`.

### Test panic pada `err.Error()`

Pastikan pemeriksaan `if err == nil` berada sebelum pemanggilan `err.Error()` dan menggunakan `t.Fatal`, bukan hanya mencetak pesan lalu meneruskan test.

### Pesan error berbeda

Bandingkan nilai `want` dengan guard clause di `db/migrate.go`. Step ini sengaja memakai exact comparison, jadi kapitalisasi, spasi, dan wording harus sama.

### Test mencoba menghubungi PostgreSQL

Pastikan argument yang dikirim benar-benar literal `nil`. Jangan menggantinya dengan `&pgxpool.Pool{}` atau `&sql.DB{}`. Keduanya non-`nil` dan akan melewati guard yang sedang diuji.

### `go: cannot find main module`

Pastikan terminal berada di:

```bash
/Users/mac/Documents/bbge/practice/srv-server
```

Folder tersebut berisi `go.mod`.

## Batas Step 33

Step ini membuktikan bahwa dua nil guard bekerja sesuai kontrak. Step ini belum membuktikan bahwa:

- `stdlib.OpenDBFromPool` berhasil membuat adapter dari pool yang valid;
- dialect Goose benar-benar tersetel ke PostgreSQL;
- embedded filesystem berisi migration yang benar;
- `goose.Up` berhasil membaca dan menjalankan file SQL;
- schema `account`, trigger, index, atau `goose_db_version` terbentuk;
- error dari pgx, `database/sql`, atau Goose dibungkus dengan context yang benar;
- migration dapat berjalan idempotent pada PostgreSQL nyata.

Semua hal tersebut membutuhkan seam test tambahan atau integration test dengan database nyata. Karena itu kelulusan Step 33 tidak boleh disebut sebagai bukti bahwa SQL migration telah berhasil dijalankan.

## Ringkasan

```text
db/migrate_test.go
├── TestAutoMigrateRejectsNilPool
│   ├── kirim nil sebagai *pgxpool.Pool
│   ├── pastikan error non-nil
│   └── pastikan pesan "postgres pool is nil"
└── TestMigrateRejectsNilDatabase
    ├── kirim nil sebagai *sql.DB
    ├── pastikan error non-nil
    └── pastikan pesan "sql database is nil"
```

Konsep yang dipelajari:

- defensive guard pada database boundary;
- type inference untuk literal `nil`;
- assertion non-`nil` sebelum memakai error;
- exact error comparison dan tradeoff-nya;
- perilaku panic pada Go test;
- perbedaan nil pointer dan pointer ke zero-value object;
- batas unit test tanpa dependency eksternal.

## Step 34 — Preview

Step berikutnya tetap kecil: menyiapkan seam yang terkontrol untuk menguji bahwa kegagalan proses migration dibungkus dengan context, misalnya prefix `run migrations:`, tanpa memerlukan PostgreSQL aktif. Kita akan memisahkan dependency yang perlu dipalsukan secukupnya, lalu menguji error dengan `errors.Is` agar underlying cause tetap dapat dikenali.

Detail implementation Step 34 akan ditentukan setelah source migration dan roadmap diperiksa kembali. Step tersebut tidak akan menggantikan integration test; keberhasilan eksekusi SQL pada PostgreSQL nyata tetap merupakan pengujian terpisah.
