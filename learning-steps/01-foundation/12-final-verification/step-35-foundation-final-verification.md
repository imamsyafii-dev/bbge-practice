# Step 35 — Verifikasi akhir milestone fondasi

Step 35 tidak menambahkan fitur baru. Step ini membuktikan bahwa hasil Step 1–34 dapat direproduksi, dibangun, dijalankan, dan dihentikan dengan benar sebagai satu sistem utuh.

Verifikasi dibagi menjadi checkpoint agar perubahan keadaan mudah dipahami dan lokasi kegagalan dapat diketahui dengan jelas.

## Tujuan

Setelah step ini, kita membuktikan bahwa:

- Docker Compose practice valid dan terisolasi dari service BBGE utama;
- generated code SQLC dapat dibuat ulang tanpa mengubah workspace;
- seluruh test, race test, dan build lulus;
- PostgreSQL practice sehat dan migration Goose berjalan pada database nyata;
- schema account, index, trigger, dan function benar-benar terbentuk;
- migration tidak dijalankan ulang pada startup kedua;
- integration test store bekerja dengan transaction dan rollback;
- request HTTP mengalir dari router sampai PostgreSQL;
- `SIGINT` dan `SIGTERM` memicu graceful shutdown;
- cleanup hanya menyentuh resource practice milik latihan.

## Prasyarat dan konfigurasi

Step 1 sampai Step 34 sudah selesai. Go, Docker Desktop/Compose, `curl`, `lsof`, dan `diff` tersedia.

Root module:

```text
/Users/mac/Documents/bbge/practice/srv-server
```

Resource default practice:

```text
Compose project : practice-srv-server
container       : practice-postgres
image           : postgres:18.3
host port       : 25432
container port  : 5432
database        : practice_server
user            : practice_user
password        : practice_password
volume          : practice-postgres-data
network         : practice-network
```

HTTP verification memakai port `18082`. Port `18081` sengaja dihindari karena sebelumnya pernah menghasilkan:

```text
listen tcp :18081: bind: address already in use
```

Nomor port dapat diganti jika diperlukan, tetapi semua command dalam satu sesi harus memakai port yang sama.

## Scope keselamatan

Step ini hanya memakai resource berprefix `practice` dan file sementara di `/private/tmp`. Step ini tidak mengubah remote Git, service, container, database, atau autentikasi BBGE utama. Source server juga tidak diubah.

Urutan checkpoint:

```text
35A  reproducibility, test, race test, dan build
35B  preflight dan startup PostgreSQL practice
35C  migration nyata, lifecycle, restart, dan integration test
35D  HTTP end-to-end dan persistence
35E  graceful shutdown dengan SIGTERM
35F  cleanup terarah
```

---

## Checkpoint 35A — Reproducibility

Checkpoint ini belum menjalankan server dan belum mengubah database.

### 35A.1 Validasi Docker Compose

```bash
cd /Users/mac/Documents/bbge/practice/srv-server

docker compose \
  -f docker/docker-compose.yml \
  config --quiet
```

Jika valid, command selesai tanpa output.

### 35A.2 Generate SQLC di folder sementara

Jangan menimpa `internal/store` hanya untuk perbandingan. Salin input generator ke folder sementara:

```bash
PRACTICE_SQLC_VERIFY_DIR="$(
  mktemp -d /private/tmp/practice-step35-sqlc.XXXXXX
)"

echo "$PRACTICE_SQLC_VERIFY_DIR"

cp -R \
  db \
  dev \
  sqlc.yaml \
  "$PRACTICE_SQLC_VERIFY_DIR/"
```

Simpan variable tersebut di shell yang sama karena dipakai lagi saat cleanup.

```bash
cd "$PRACTICE_SQLC_VERIFY_DIR"

./dev/sqlc
```

Script mengunci SQLC pada `v1.31.1`, membaca salinan schema/query/config, lalu menulis hasil generate ke folder sementara. Ini tidak menimpa generated store di workspace. Namun, jika binary SQLC belum tersedia atau versinya berbeda, `./dev/sqlc` dapat menjalankan `go install` dan memasang versi tersebut ke Go bin milik pengguna.

### 35A.3 Bandingkan tepat empat file generated

```bash
cd /Users/mac/Documents/bbge/practice/srv-server

for file in \
  account.sql.go \
  db.go \
  models.go \
  querier.go
do
  diff -u \
    "internal/store/$file" \
    "$PRACTICE_SQLC_VERIFY_DIR/internal/store/$file"
done
```

Hasil yang benar: tidak ada output.

> **Penting:** whole-folder diff secara sah melaporkan `account_postgres_test.go` sebagai file tambahan. File itu dibuat manual untuk integration test dan bukan keluaran SQLC. Generator hanya memiliki empat output yang dibandingkan di atas: `account.sql.go`, `db.go`, `models.go`, dan `querier.go`.

Command berikut berguna untuk investigasi, tetapi perbedaan file test manual tersebut bukan generated drift:

```bash
diff -ru \
  internal/store \
  "$PRACTICE_SQLC_VERIFY_DIR/internal/store"
```

Hasil aktual: keempat file generated identik.

### 35A.4 Test, race detector, dan build

```bash
go test ./...
go test -race ./...
go build ./...
```

Hasil aktual:

- seluruh unit test lulus;
- integration test PostgreSQL dilewati pada run normal karena belum diaktifkan;
- race detector tidak menemukan data race;
- seluruh package berhasil dibangun;
- `go build ./...` selesai tanpa output.

Durasi test dapat berbeda antar-komputer.

---

## Checkpoint 35B — PostgreSQL practice

### 35B.1 Preflight port database

```bash
docker ps \
  --filter publish=25432 \
  --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'
```

Aman dilanjutkan jika tidak ada container atau yang muncul hanya `practice-postgres`. Jika nama lain muncul, jangan hentikan container tersebut hanya untuk membebaskan port.

### 35B.2 Menyalakan dan memeriksa database

```bash
cd /Users/mac/Documents/bbge/practice/srv-server

docker compose \
  -f docker/docker-compose.yml \
  up -d --wait practice-postgres

docker compose \
  -f docker/docker-compose.yml \
  ps
```

Periksa PostgreSQL dari dalam container:

```bash
docker exec \
  practice-postgres \
  pg_isready \
  -U practice_user \
  -d practice_server
```

Hasil aktual:

```text
/var/run/postgresql:5432 - accepting connections
```

Container `practice-postgres` berstatus `healthy` dengan mapping `25432 -> 5432`.

---

## Checkpoint 35C — Migration dan lifecycle nyata

### 35C.1 Membuat binary sementara

Gunakan binary hasil build, bukan `go run`, agar signal dikirim langsung ke proses server dan tidak tertahan oleh proses Go sebagai parent.

```bash
cd /Users/mac/Documents/bbge/practice/srv-server

go build \
  -o /private/tmp/practice-srv-server-step35 \
  .
```

Preflight port HTTP:

```bash
lsof -nP \
  -iTCP:18082 \
  -sTCP:LISTEN
```

Hasil aman: tidak ada output. Jangan menghentikan proses yang tidak dimiliki sesi practice.

### 35C.2 Startup pertama

Di terminal server:

```bash
PRACTICE_HTTP_ADDR=:18082 \
  /private/tmp/practice-srv-server-step35
```

Alur log:

```text
connected to PostgreSQL database practice_server
database migrations applied
practice server listening on http://localhost:18082
```

Timestamp dapat berbeda. Pada database baru Goose menjalankan migration `0001` dan `0002`; pada database yang sudah dimigrasikan Goose dapat melaporkan tidak ada migration baru. Keduanya valid jika version akhir `2` dan schema sesuai.

Uji health dari terminal lain:

```bash
curl -i \
  http://127.0.0.1:18082/health
```

Hasil aktual:

```text
HTTP/1.1 200 OK
Content-Type: application/json

{"status":"ok"}
```

Header tambahan, timestamp, dan urutan header dapat berbeda.

### 35C.3 Memeriksa hasil migration

```bash
docker exec \
  practice-postgres \
  psql \
  -U practice_user \
  -d practice_server \
  -v ON_ERROR_STOP=1 \
  -c "SELECT version_id, is_applied FROM goose_db_version ORDER BY id;"
```

Hasil aktual mempunyai row version `0`, `1`, dan `2`, semuanya applied. Version `0` adalah baseline internal Goose, version `1` berasal dari migration function trigger, dan version `2` berasal dari migration tabel account.

Tabel aktual:

```text
account
goose_db_version
```

Empat index aktual pada `account`:

```text
account_pkey
account_status_idx
account_user_id_unique
account_version_idx
```

`account_pkey` berasal dari primary key, `account_user_id_unique` dari unique constraint, dan dua index lain dibuat eksplisit.

Object lain:

```text
trigger  : update_account_updated_at
function : auto_update_at_column
```

Function mengisi `updated_at` dengan waktu saat ini sebelum row account diperbarui.

### 35C.4 Graceful shutdown dengan SIGINT

Di terminal server, tekan `Control+C`. Ini mengirim `SIGINT`; `signal.NotifyContext` membatalkan application context dan lifecycle memanggil `Shutdown` dengan timeout.

Log aktual:

```text
shutdown signal received
practice server stopped
```

Pastikan port dilepas:

```bash
lsof -nP \
  -iTCP:18082 \
  -sTCP:LISTEN
```

Hasil aktual: tidak ada output.

### 35C.5 Startup kedua dan idempotency Goose

```bash
PRACTICE_HTTP_ADDR=:18082 \
  /private/tmp/practice-srv-server-step35
```

Startup kedua berhasil. Goose melaporkan keadaan setara dengan:

```text
no migrations to run. current version: 2
```

Version `1` dan `2` masing-masing tetap hanya memiliki satu row applied. Goose membaca `goose_db_version` dan tidak menjalankan ulang migration yang sudah tercatat.

Ini membuktikan idempotency pada **migration runner**, bukan bahwa seluruh raw SQL aman dieksekusi manual berulang kali. Contohnya, `CREATE TABLE account` tanpa `IF NOT EXISTS` akan gagal jika SQL mentah dipanggil lagi di luar mekanisme versioning Goose.

### 35C.6 Integration test PostgreSQL

Biarkan server kedua hidup. Dari terminal lain:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server

PRACTICE_RUN_POSTGRES_TESTS=1 \
  go test ./internal/store \
  -run '^TestAccountQueriesAgainstPostgres$' \
  -count=1 \
  -v
```

Hasil aktual:

```text
=== RUN   TestAccountQueriesAgainstPostgres
--- PASS: TestAccountQueriesAgainstPostgres
PASS
```

Test membuka database nyata, memulai transaction, membuat dan membaca account melalui generated query, melakukan rollback, lalu memastikan row tidak tersimpan.

Rollback tidak mengembalikan sequence PostgreSQL. Nilai `BIGSERIAL` yang sempat dialokasikan dapat terlewati walaupun row di-rollback. Karena itu celah ID bukan kegagalan rollback.

---

## Checkpoint 35D — HTTP end-to-end

Server startup kedua tetap hidup. Alur yang dibuktikan:

```text
HTTP request
  -> router/handler
  -> application use case
  -> domain service
  -> sqlc store
  -> PostgreSQL
```

### 35D.1 Membersihkan hanya data target

Agar POST dapat diulang dengan aman, hapus hanya row Step 35:

```bash
docker exec \
  practice-postgres \
  psql \
  -U practice_user \
  -d practice_server \
  -v ON_ERROR_STOP=1 \
  -c "DELETE FROM account WHERE user_id = 'practice-step-35-final';"
```

Command ini tidak mengosongkan tabel dan tidak menyentuh account lain.

### 35D.2 Health endpoint

```bash
curl -i \
  http://127.0.0.1:18082/health
```

Hasil aktual adalah `200 OK` dengan body:

```json
{"status":"ok"}
```

### 35D.3 Membuat account

```bash
curl -i \
  -X POST \
  http://127.0.0.1:18082/accounts \
  -H 'Content-Type: application/json' \
  -d '{
    "user_id": "practice-step-35-final",
    "name": "Final Practice User"
  }'
```

Hasil aktual POST:

```text
HTTP/1.1 201 Created
```

Account aktual:

```text
id      : 6
user_id : practice-step-35-final
name    : Final Practice User
status  : 1
version : 0
```

ID `6` adalah hasil sesi ini, bukan nilai yang harus di-hardcode. ID, timestamp, durasi, dan PID dapat berbeda pada run berikutnya.

### 35D.4 Mengambil account

```bash
curl -i \
  http://127.0.0.1:18082/accounts/practice-step-35-final
```

Hasil aktual GET adalah `200 OK` dan mengembalikan account yang sama.

### 35D.5 Membuktikan persistence

```bash
docker exec \
  practice-postgres \
  psql \
  -U practice_user \
  -d practice_server \
  -v ON_ERROR_STOP=1 \
  -c "
    SELECT
      id,
      user_id,
      name,
      status,
      version
    FROM account
    WHERE user_id = 'practice-step-35-final';
  "
```

Hasil aktual menunjukkan tepat satu row dengan `id=6`, `user_id=practice-step-35-final`, `name=Final Practice User`, `status=1`, dan `version=0`.

Account practice tidak memakai email. Contract latihan saat ini hanya memakai field yang dibutuhkan alur account tersebut.

---

## Checkpoint 35E — SIGTERM

`SIGINT` sudah diuji melalui `Control+C`. Sekarang uji signal lain yang didaftarkan server.

Cari PID proses yang benar-benar listen:

```bash
lsof -nP \
  -iTCP:18082 \
  -sTCP:LISTEN
```

Ambil PID dari kolom kedua, lalu:

```bash
kill -TERM PID_YANG_TAMPIL
```

Jangan memakai PID contoh; PID berubah setiap kali proses dijalankan.

Terminal server menampilkan:

```text
shutdown signal received
practice server stopped
```

Setelah prompt kembali, periksa exit code di terminal server:

```bash
echo $?
```

Hasil aktual:

```text
0
```

Pastikan port sudah dilepas:

```bash
lsof -nP \
  -iTCP:18082 \
  -sTCP:LISTEN
```

Hasil aktual: tidak ada output. Ini membuktikan `SIGTERM` diterima, graceful shutdown selesai, proses keluar normal, dan listener tidak tertinggal.

Graceful shutdown memberi request aktif kesempatan selesai sampai timeout. Jika `Shutdown` gagal, lifecycle Step 31 memiliki fallback `Close`.

---

## Checkpoint 35F — Cleanup terarah

### 35F.1 Menghapus hanya row E2E

```bash
docker exec \
  practice-postgres \
  psql \
  -U practice_user \
  -d practice_server \
  -v ON_ERROR_STOP=1 \
  -c "
    DELETE FROM account
    WHERE user_id = 'practice-step-35-final';
  "
```

Hasil aktual:

```text
DELETE 1
```

### 35F.2 Menghentikan hanya Compose practice

```bash
cd /Users/mac/Documents/bbge/practice/srv-server

docker compose \
  -f docker/docker-compose.yml \
  down
```

Jangan tambahkan `-v`. Tanpa `-v`, container/network Compose dihentikan atau dihapus, tetapi volume data dipertahankan.

```bash
docker ps \
  --filter name=practice-postgres \
  --format 'table {{.Names}}\t{{.Status}}'

docker volume ls \
  --filter name=practice-postgres-data
```

Hasil aktual: tidak ada container `practice-postgres` yang berjalan, sedangkan volume `practice-postgres-data` tetap tersedia. Migration history dan data practice lain tetap dapat dipakai.

### 35F.3 Menghapus binary sementara

```bash
rm -f \
  /private/tmp/practice-srv-server-step35
```

Hasil aktual: binary sementara sudah tidak ada.

### 35F.4 Menghapus salinan SQLC dengan guard

```bash
echo "$PRACTICE_SQLC_VERIFY_DIR"
```

Validasi bahwa path merupakan anak langsung `/private/tmp` dan basename mengikuti pola `practice-step35-sqlc.*` sebelum menghapusnya:

```bash
PRACTICE_SQLC_VERIFY_PARENT="$(
  dirname -- "$PRACTICE_SQLC_VERIFY_DIR"
)"
PRACTICE_SQLC_VERIFY_NAME="$(
  basename -- "$PRACTICE_SQLC_VERIFY_DIR"
)"

if [ "$PRACTICE_SQLC_VERIFY_PARENT" != "/private/tmp" ]; then
  echo "unexpected temp parent; directory was not removed"
else
  case "$PRACTICE_SQLC_VERIFY_NAME" in
    practice-step35-sqlc.?*)
      rm -rf -- "$PRACTICE_SQLC_VERIFY_DIR"
      ;;
    *)
      echo "unexpected temp name; directory was not removed"
      ;;
  esac
fi
```

Pemeriksaan `dirname` memastikan target berada tepat di bawah `/private/tmp`, bukan di subdirectory lain yang kebetulan memiliki prefix serupa. Pemeriksaan `basename` mengharuskan nama memiliki prefix yang dibuat oleh `mktemp` dan setidaknya satu karakter suffix. Nilai kosong juga gagal pada kedua pemeriksaan sehingga `rm -rf` tidak dijalankan. Variable shell tidak otomatis berpindah ke terminal baru; jika nilainya kosong, jangan melakukan penghapusan.

State akhir aktual:

```text
server process                 : stopped
HTTP port 18082                : released
practice-postgres container    : stopped/removed by Compose
practice-postgres-data volume  : preserved
Step 35 E2E account            : removed
temporary server binary        : removed
temporary SQLC directory       : removed
BBGE services and databases    : unchanged
```

---

## Mengapa dibagi menjadi checkpoint?

- 35A gagal: masalah source, generator, test, race, atau build;
- 35B gagal: masalah Docker, port, healthcheck, atau PostgreSQL;
- 35C gagal: masalah startup, migration, schema, lifecycle, atau store;
- 35D gagal: masalah wiring HTTP sampai database;
- 35E gagal: masalah signal atau shutdown;
- 35F gagal: masalah cleanup dan kepemilikan resource.

Setiap checkpoint memastikan precondition checkpoint berikutnya.

## Troubleshooting

### Port HTTP sudah dipakai

```bash
lsof -nP \
  -iTCP:18082 \
  -sTCP:LISTEN
```

Identifikasi prosesnya. Jangan langsung menjalankan `kill` bila proses bukan binary practice milik sesi ini. Pilih port practice lain dan pakai konsisten jika perlu.

### Port PostgreSQL sudah dipakai

```bash
docker ps \
  --filter publish=25432 \
  --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'
```

Jika pemiliknya bukan `practice-postgres`, jangan menghentikannya. Selesaikan konflik kepemilikan atau ubah `PRACTICE_POSTGRES_PORT` secara konsisten.

### PostgreSQL belum sehat

```bash
docker compose \
  -f docker/docker-compose.yml \
  ps

docker logs practice-postgres
```

Periksa healthcheck, credential, port, dan volume. Jangan menghapus volume sebagai langkah pertama.

### Whole-folder SQLC diff menunjukkan file tambahan

Jika hanya `internal/store/account_postgres_test.go`, itu bukan generated drift. Ulangi diff hanya pada empat file generated.

### Integration test tampil `SKIP`

```bash
PRACTICE_RUN_POSTGRES_TESTS=1 \
  go test ./internal/store \
  -run '^TestAccountQueriesAgainstPostgres$' \
  -count=1 \
  -v
```

Environment opt-in harus berada pada command yang sama dan PostgreSQL practice harus aktif.

### POST conflict atau duplicate user

Hapus hanya row target `practice-step-35-final`, lalu ulangi POST. Jangan memakai `TRUNCATE` karena scope-nya lebih luas.

### ID bukan `6`

Itu normal. Rollback menghapus row, tetapi tidak mengembalikan sequence. Periksa bahwa ID positif serta `user_id`, `name`, `status`, `version`, dan persistence sesuai.

### Startup kedua mencoba menjalankan migration lagi

```bash
docker exec \
  practice-postgres \
  psql \
  -U practice_user \
  -d practice_server \
  -c "SELECT version_id, is_applied FROM goose_db_version ORDER BY id;"
```

Pastikan aplikasi memakai database dan volume practice yang sama. Jangan menambahkan history migration secara manual untuk menyamarkan masalah.

## Arsitektur fondasi akhir

```text
environment configuration
  -> PostgreSQL pool
  -> embedded Goose migrations
  -> sqlc Queries
  -> account domain service
  -> account application use case
  -> HTTP account handler/router
  -> net/http server lifecycle
  -> graceful shutdown
```

Lapisan verifikasi yang tersedia:

```text
configuration tests
migration boundary tests
PostgreSQL store integration test
domain service tests
application use-case tests
HTTP handler/router tests
dependency composition tests
HTTP server lifecycle tests
manual end-to-end verification
```

## Kemampuan yang dipelajari

- Go module, package, function, pointer, interface, dan dependency injection;
- HTTP server/handler dengan `net/http`;
- environment configuration berprefix `PRACTICE_`;
- Docker Compose, volume, network, healthcheck, dan isolasi resource;
- PostgreSQL pool, embedded Goose migration, dan version history;
- SQLC generation serta generated query/store;
- integration test dengan transaction dan rollback;
- pemisahan store, domain service, application use case, dan router;
- wiring dependency pada composition root;
- fake implementation, error wrapping `%w`, dan `errors.Is`;
- channel, `SIGINT`, `SIGTERM`, graceful shutdown, dan fallback close;
- test, race detector, build, end-to-end check, preflight, dan cleanup aman.

## Batas milestone fondasi

Lulus Step 35 membuktikan fondasi satu modul account, bukan seluruh `srv-server` BBGE. Fase lanjutan masih mencakup:

- identity/account workflow yang lebih lengkap;
- authentication, token, principal, dan middleware;
- friend, channel, group, dan message;
- Redis dan realtime state;
- WuKongIM/webhook dan LiveKit;
- background worker serta transaction/version allocation;
- observability, deployment production, dan client mobile.

Fase berikutnya menambah satu kelompok kemampuan di atas fondasi ini. Boundary antar-layer perlu dipertahankan agar modul baru tetap dapat diuji terpisah.

## Scope commit Step 35

Step ini tidak mengubah source server. File yang ditambahkan hanya:

```text
learning-steps/01-foundation/12-final-verification/step-35-foundation-final-verification.md
```

Commit dari root repository practice:

```bash
cd /Users/mac/Documents/bbge/practice

git add \
  learning-steps/01-foundation/12-final-verification/step-35-foundation-final-verification.md

git commit -m "step 35: verify foundation milestone"
git push origin main
```

Dengan verifikasi ini, milestone **01-foundation, Step 1–35** selesai.
