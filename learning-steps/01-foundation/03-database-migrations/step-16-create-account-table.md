# Step 16 — Membuat Tabel Account

Pada Step 15, server sudah menjalankan embedded migration saat startup dan database berada pada version 1. Migration pertama hanya membuat function `auto_update_at_column`; tabel bisnis belum ada. Pada langkah ini, kita menambahkan migration version 2 untuk membuat tabel `account` pertama.

Schema latihan sengaja lebih kecil daripada schema account BBGE. Tujuannya adalah mempelajari migration, constraint, index, dan trigger sebelum menambahkan kebutuhan domain yang lebih banyak.

## Membuat migration version 2

Masuk ke direktori server, lalu buat file migration berikut:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server
touch db/migrations/0002_create_account.sql
```

Isi `db/migrations/0002_create_account.sql` dengan SQL berikut:

```sql
-- +goose Up
CREATE TABLE account (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(40) NOT NULL,
    name VARCHAR(100) NOT NULL DEFAULT '',
    status SMALLINT NOT NULL DEFAULT 1,
    version BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT account_user_id_unique UNIQUE (user_id)
);

CREATE INDEX account_status_idx ON account (status);
CREATE INDEX account_version_idx ON account (version);

CREATE TRIGGER update_account_updated_at
    BEFORE UPDATE ON account
    FOR EACH ROW
    EXECUTE FUNCTION auto_update_at_column();

-- +goose Down
DROP TRIGGER IF EXISTS update_account_updated_at ON account;
DROP INDEX IF EXISTS account_version_idx;
DROP INDEX IF EXISTS account_status_idx;
DROP TABLE IF EXISTS account;
```

Pastikan file disimpan dengan newline setelah statement terakhir. PostgreSQL dan Goose tetap dapat membaca file tanpa final newline, tetapi newline di akhir file mengikuti konvensi file teks dan membuat output tool tidak menyatu dengan baris terakhir.

## Schema latihan yang disederhanakan

Schema account BBGE memiliki lebih banyak field untuk profil, short number, role, status sinkronisasi, avatar, dan kebutuhan domain lainnya. Tabel latihan hanya memakai tujuh kolom yang diperlukan untuk membangun fondasi:

- `id` adalah primary key internal bertipe `BIGSERIAL`. PostgreSQL membuat sequence dan memakai nilainya sebagai default ID baru.
- `user_id` adalah identitas account dari sisi aplikasi. Nilainya wajib ada dan dibatasi hingga 40 karakter.
- `name` adalah nama tampilan. Nilainya wajib ada dan memakai string kosong sebagai default.
- `status` menyimpan status account dalam angka kecil. Default `1` dipakai sebagai status aktif awal.
- `version` adalah version perubahan data untuk kebutuhan sinkronisasi pada langkah berikutnya. Default dimulai dari `0`.
- `created_at` mencatat waktu row dibuat dengan `CURRENT_TIMESTAMP`.
- `updated_at` dimulai dengan waktu pembuatan yang sama dan diperbarui oleh trigger ketika row berubah.

Tabel ini secara eksplisit **tidak memiliki kolom `email`**. `user_id` menjadi identitas account pada latihan ini. Data autentikasi atau contact address tidak dicampurkan ke tabel account hanya karena umum ditemukan pada contoh aplikasi lain.

## Primary key, unique constraint, dan index

Deklarasi berikut membuat primary key:

```sql
id BIGSERIAL PRIMARY KEY
```

PostgreSQL otomatis membuat unique B-tree index bernama `account_pkey` untuk primary key tersebut.

Constraint berikut memastikan satu `user_id` hanya dimiliki oleh satu row:

```sql
CONSTRAINT account_user_id_unique UNIQUE (user_id)
```

PostgreSQL juga otomatis membuat unique index bernama `account_user_id_unique`. Karena itu, migration tidak perlu membuat index kedua untuk `id` atau `user_id`.

Dua index dibuat secara eksplisit:

```sql
CREATE INDEX account_status_idx ON account (status);
CREATE INDEX account_version_idx ON account (version);
```

`account_status_idx` mendukung query yang menyaring account berdasarkan status. `account_version_idx` mendukung pembacaan perubahan berdasarkan version. Setelah migration diterapkan, total index tabel account adalah empat: dua implicit dari constraint dan dua explicit dari migration.

## Memakai strict DDL pada bagian Up

Bagian `Up` memakai `CREATE TABLE`, `CREATE INDEX`, dan `CREATE TRIGGER` tanpa `IF NOT EXISTS`. Ini disengaja.

Goose mencatat version yang sudah diterapkan dan tidak menjalankan ulang version 2 pada startup berikutnya. Jika object bernama sama sudah ada padahal version 2 belum tercatat, keadaan tersebut menandakan schema drift atau perubahan manual. Strict DDL membuat startup gagal dan memperlihatkan masalah tersebut. `IF NOT EXISTS` justru dapat menyembunyikan object lama yang bentuknya tidak sesuai dengan migration.

## Menghubungkan trigger dengan migration 0001

Trigger berikut dijalankan sebelum setiap operasi `UPDATE` pada row account:

```sql
CREATE TRIGGER update_account_updated_at
    BEFORE UPDATE ON account
    FOR EACH ROW
    EXECUTE FUNCTION auto_update_at_column();
```

Function `auto_update_at_column()` dimiliki migration `0001_create_trigger_function.sql`. Karena Goose menerapkan migration berdasarkan urutan version, function sudah tersedia sebelum migration 0002 mencoba membuat trigger. Function mengisi `NEW.updated_at` dengan `CURRENT_TIMESTAMP`, lalu mengembalikan row baru.

Migration ini tidak memerlukan `-- +goose StatementBegin` dan `-- +goose StatementEnd`. Setiap statement pada version 2 adalah DDL biasa dengan satu batas titik koma. Kedua directive tersebut diperlukan pada version 1 karena body function PL/pgSQL memiliki beberapa titik koma yang harus diperlakukan Goose sebagai satu statement.

## Menjaga urutan rollback

Bagian `Down` melepaskan object dari yang paling bergantung ke object induknya:

1. Trigger dilepas lebih dahulu karena bergantung pada tabel dan function dari version 1.
2. Index explicit dilepas dalam urutan terbalik dari pembuatannya: version lalu status.
3. Tabel account dilepas terakhir. Primary-key index dan unique index milik constraint ikut dilepas bersama tabel.

Migration 0002 tidak menghapus `auto_update_at_column()`. Function tersebut dimiliki migration 0001 dan mungkin digunakan tabel lain pada migration berikutnya. Ketika rollback dilakukan berurutan, Goose membatalkan version 2 sebelum version 1, sehingga trigger sudah hilang sebelum function milik version 1 dihapus.

## Memeriksa compile sebelum menjalankan migration

Jalankan test seluruh package:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server
go test ./...
```

Command ini memastikan migration baru cocok dengan pola `//go:embed migrations/*.sql` dan seluruh package Go tetap dapat dikompilasi. Namun, `go test ./...` tidak menjalankan `main` dan tidak memanggil `db.AutoMigrate`, sehingga database belum berubah hanya karena test berhasil.

Jalankan server untuk benar-benar menerapkan migration:

```bash
go run .
```

Karena `main.go` sudah memanggil `db.AutoMigrate`, startup akan menemukan version 2 sebagai migration yang belum diterapkan. Output akan memuat alur seperti berikut; timestamp dan durasi dapat berbeda:

```text
2026/10/15 18:30:00 connected to PostgreSQL database practice_server
2026/10/15 18:30:00 OK   0002_create_account.sql (...)
2026/10/15 18:30:00 goose: successfully migrated database to version: 2
2026/10/15 18:30:00 database migrations applied
2026/10/15 18:30:00 practice server listening on http://localhost:18080
```

Biarkan server berjalan selama verifikasi berikut dilakukan dari terminal lain.

## Memverifikasi version migration

Masuk ke direktori Compose:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server/docker
```

Periksa version terakhir yang sedang diterapkan:

```bash
docker compose exec practice-postgres \
  psql -U practice_user -d practice_server \
  -c "SELECT MAX(version_id) AS current_version FROM goose_db_version WHERE is_applied;"
```

Hasil yang diharapkan:

```text
 current_version
-----------------
               2
```

## Memverifikasi kolom dan memastikan email tidak ada

Periksa kolom berdasarkan urutan deklarasinya:

```bash
docker compose exec practice-postgres \
  psql -U practice_user -d practice_server \
  -c "SELECT string_agg(column_name, ', ' ORDER BY ordinal_position) AS columns FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'account';"
```

Hasil yang diharapkan:

```text
                              columns
--------------------------------------------------------------------
 id, user_id, name, status, version, created_at, updated_at
```

Periksa secara khusus bahwa tidak ada kolom email:

```bash
docker compose exec practice-postgres \
  psql -U practice_user -d practice_server \
  -c "SELECT COUNT(*) AS email_columns FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'account' AND column_name = 'email';"
```

Hasil yang diharapkan:

```text
 email_columns
---------------
             0
```

## Memverifikasi trigger

Periksa trigger yang terpasang pada tabel account:

```bash
docker compose exec practice-postgres \
  psql -U practice_user -d practice_server \
  -c "SELECT trigger_name, action_timing, event_manipulation FROM information_schema.triggers WHERE event_object_schema = 'public' AND event_object_table = 'account';"
```

Hasil yang diharapkan:

```text
         trigger_name          | action_timing | event_manipulation
-------------------------------+---------------+--------------------
 update_account_updated_at     | BEFORE        | UPDATE
```

## Memverifikasi semua index

Periksa index implicit dan explicit:

```bash
docker compose exec practice-postgres \
  psql -U practice_user -d practice_server \
  -c "SELECT indexname FROM pg_indexes WHERE schemaname = 'public' AND tablename = 'account' ORDER BY indexname;"
```

Hasil yang diharapkan:

```text
          indexname
-----------------------------
 account_pkey
 account_status_idx
 account_user_id_unique
 account_version_idx
```

`account_pkey` dan `account_user_id_unique` dibuat secara implicit oleh PostgreSQL. `account_status_idx` dan `account_version_idx` berasal dari statement `CREATE INDEX` pada migration.

## Memverifikasi restart yang idempotent

Hentikan server dengan `Ctrl+C`, lalu jalankan kembali:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server
go run .
```

Pada startup berikutnya, Goose mengetahui version 1 dan 2 sudah diterapkan. SQL `CREATE TABLE`, `CREATE INDEX`, dan `CREATE TRIGGER` tidak dijalankan ulang. Output akan memuat:

```text
2026/10/15 18:32:00 connected to PostgreSQL database practice_server
2026/10/15 18:32:00 goose: no migrations to run. current version: 2
2026/10/15 18:32:00 database migrations applied
2026/10/15 18:32:00 practice server listening on http://localhost:18080
```

Query version tetap menghasilkan `2`, tabel dan empat index tetap tersedia, serta startup tidak gagal akibat strict DDL. Idempotency berasal dari version tracking Goose, bukan dari `IF NOT EXISTS` pada setiap statement.

## Menjaga migration yang sudah diterapkan

Setelah version 2 diterapkan, jangan mengubah `0002_create_account.sql` untuk menambah kolom atau index baru. Goose menyimpan version dan applied state di `goose_db_version`; Goose tidak menyimpan atau membandingkan checksum isi file migration.

Jika file 0002 diubah, database lama tetap menganggap version 2 selesai dan tidak menjalankannya lagi. Database baru akan menjalankan isi 0002 yang sudah berubah. Akibatnya, kedua database dapat memiliki schema berbeda walaupun melaporkan version yang sama.

Setiap perubahan setelah Step 16 harus dibuat sebagai migration version berikutnya. Pola append-only tersebut menjaga urutan perubahan schema dapat direproduksi pada database lama maupun database baru.
