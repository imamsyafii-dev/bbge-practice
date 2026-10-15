# Step 14 — Membuat Embedded Migration Runner

Pada Step 13, project sudah menambahkan Goose sebagai dependency, tetapi belum memiliki file migration atau kode yang menjalankannya. Pada langkah ini, kita membuat migration runner dan migration pertama yang disematkan ke dalam binary Go.

Step ini belum menghubungkan runner ke `main.go`. Kode akan dikompilasi dan diuji, tetapi belum mengubah database ketika server dijalankan.

## Membuat direktori dan file migration

Masuk ke direktori server, lalu buat package database dan direktori migration:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server
mkdir -p db/migrations
touch db/migrate.go
touch db/migrations/0001_create_trigger_function.sql
```

Nama `0001_create_trigger_function.sql` memakai prefix angka agar Goose mengetahui urutan migration. Migration dengan versi lebih kecil dijalankan lebih dahulu.

## Membuat migration runner

Isi `db/migrate.go` dengan kode berikut:

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

	if err := goose.Up(
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

## Menyematkan migration ke dalam binary

Directive berikut dibaca oleh compiler Go:

```go
//go:embed migrations/*.sql
var migrations embed.FS
```

`//go:embed` harus diletakkan tepat sebelum variable yang menerima hasil embed. Jangan menambahkan baris kosong di antara directive dan deklarasi variable. Path `migrations/*.sql` bersifat relatif terhadap direktori package `db`, sehingga cocok dengan file di `db/migrations`.

`embed.FS` adalah filesystem read-only yang menjadi bagian dari binary hasil build. Aplikasi tidak perlu mencari direktori migration dari current working directory ketika runtime. Selama file SQL sudah ada pada saat build, Goose dapat membacanya dari variable `migrations`.

Package `embed` diimpor sebagai `embed`, bukan blank import, karena tipe `embed.FS` dipakai secara langsung pada deklarasi variable.

## Memisahkan AutoMigrate dan Migrate

Runner memiliki dua lapisan:

1. `AutoMigrate` menerima `*pgxpool.Pool`, yaitu tipe connection pool yang sudah dipakai server.
2. `Migrate` menerima `*sql.DB`, yaitu handle database dari package `database/sql` yang dibutuhkan Goose.

Pemisahan ini menjaga tanggung jawab setiap fungsi tetap jelas. `AutoMigrate` menangani adapter dari pgxpool, sedangkan `Migrate` hanya mengatur dan menjalankan Goose. Fungsi `Migrate` juga dapat dipakai secara terpisah pada test atau tool yang sudah memiliki `*sql.DB`.

Kedua fungsi memeriksa argument `nil` sebelum memakainya:

```go
if pool == nil {
	return fmt.Errorf("postgres pool is nil")
}
```

dan:

```go
if database == nil {
	return fmt.Errorf("sql database is nil")
}
```

Nil check menghasilkan error yang jelas dan mencegah panic akibat dereference pointer kosong.

## Menjembatani pgxpool dan database/sql

Goose bekerja dengan `*sql.DB`, sedangkan server sudah memakai `*pgxpool.Pool`. Fungsi berikut membuat adapter `database/sql` di atas pool yang sama:

```go
sqlDB := stdlib.OpenDBFromPool(pool)
defer sqlDB.Close()
```

`stdlib.OpenDBFromPool` berasal dari adapter standard library milik pgx. Menutup `sqlDB` hanya menutup adapter `database/sql`; tindakan tersebut tidak menutup `pgxpool.Pool` yang diberikan oleh pemanggil. Kepemilikan pool tetap berada di `main`, yang menutupnya melalui `defer database.Close()`.

Karena `AutoMigrate` membuat adapter, fungsi tersebut juga bertanggung jawab menutup adapter setelah `Migrate` selesai. Inilah alasan `defer sqlDB.Close()` berada di `AutoMigrate`, bukan di pemanggil.

## Mengatur dan menjalankan Goose

`Migrate` melakukan tiga hal utama. Pertama, runner memilih dialect PostgreSQL:

```go
if err := goose.SetDialect("postgres"); err != nil {
	return fmt.Errorf(
		"set goose dialect: %w",
		err,
	)
}
```

Dialect memberi tahu Goose cara berkomunikasi dengan database target dan mengelola tabel versi migration.

Kedua, embedded filesystem diberikan kepada Goose:

```go
goose.SetBaseFS(migrations)
```

Ketiga, semua migration `Up` yang belum pernah diterapkan dijalankan dari direktori `migrations`:

```go
if err := goose.Up(
	database,
	"migrations",
); err != nil {
	return fmt.Errorf(
		"run migrations: %w",
		err,
	)
}
```

Error dibungkus dengan `%w`, sehingga pemanggil mendapat konteks operasi seperti `set goose dialect` atau `run migrations` tanpa kehilangan error asli untuk pemeriksaan lebih lanjut.

## Membuat migration pertama

Isi `db/migrations/0001_create_trigger_function.sql` dengan SQL berikut:

```sql
-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION auto_update_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION IF EXISTS auto_update_at_column();
```

Simpan file SQL dengan newline di akhir file. PostgreSQL dan Goose tetap dapat membaca statement tanpa newline tersebut, tetapi final newline mengikuti konvensi file teks dan mencegah output tool berikutnya menyatu pada baris terakhir.

## Memahami directive Goose

`-- +goose Up` menandai statement yang dijalankan ketika migration diterapkan. Pada migration pertama, bagian ini membuat atau mengganti function PostgreSQL bernama `auto_update_at_column`.

Function tersebut dirancang untuk dipanggil oleh trigger tabel pada migration berikutnya. Ketika sebuah row diperbarui, function mengisi `NEW.updated_at` dengan waktu database saat ini, lalu mengembalikan row baru agar proses update dapat dilanjutkan.

`-- +goose Down` menandai statement rollback. Jika migration ini dibatalkan setelah semua object yang bergantung kepadanya sudah dilepas, Goose menghapus function dengan:

```sql
DROP FUNCTION IF EXISTS auto_update_at_column();
```

`-- +goose StatementBegin` dan `-- +goose StatementEnd` membungkus definition PL/pgSQL sebagai satu statement. Isi function memiliki beberapa tanda titik koma. Tanpa kedua directive tersebut, parser migration dapat mengira titik koma di dalam body function sebagai batas beberapa statement SQL terpisah.

## Memformat, merapikan dependency, dan menguji

Format source code Go:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server
gofmt -w db/migrate.go
```

Sekarang source code sudah mengimpor Goose, sehingga `go mod tidy` aman dijalankan:

```bash
go mod tidy
```

Setelah command tersebut, `github.com/pressly/goose/v3 v3.27.0` berpindah dari dependency `// indirect` menjadi dependency langsung di `go.mod`. `go.sum` juga disesuaikan dengan dependency transitif yang benar-benar dibutuhkan oleh build dan test.

Jalankan test seluruh package:

```bash
go test ./...
```

Command ini memastikan package `db`, embedded path, import pgx adapter, dan Goose dapat dikompilasi. Test ini belum menjalankan migration terhadap PostgreSQL karena tidak ada kode yang memanggil `AutoMigrate`.

## Batas scope Step 14

Step ini hanya membuat migration runner dan function PostgreSQL pertama. `main.go` tetap sama seperti Step 12, sehingga `go run .` masih hanya membuka connection pool dan HTTP server. Belum ada tabel versi Goose, function `auto_update_at_column`, atau perubahan schema lain yang dibuat di database oleh runtime aplikasi.

Pada langkah berikutnya, `AutoMigrate` akan dihubungkan ke alur startup setelah PostgreSQL berhasil dibuka. Langkah tersebut juga akan menjalankan server dan memverifikasi bahwa migration tercatat serta function benar-benar tersedia di database.
