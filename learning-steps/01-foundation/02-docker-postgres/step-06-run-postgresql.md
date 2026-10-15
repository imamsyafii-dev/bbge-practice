# Step 6 — Menjalankan dan Memverifikasi PostgreSQL

Mulai dari direktori Docker server:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server/docker
```

## Reset resource latihan lama

> **Peringatan:** perintah berikut bersifat destruktif. Container akan dihapus, seluruh data PostgreSQL di volume latihan akan hilang, dan network latihan akan dihapus. Jalankan hanya jika memang ingin mengulang latihan dari kondisi bersih. Jangan mengganti target dengan nama resource milik project lain.

Hapus container `practice-postgres`, lalu hapus volume dan network latihan versi lama (ber-prefix nama project) maupun versi baru (bernama eksplisit):

```bash
docker stop practice-postgres
docker rm practice-postgres
docker volume rm practice_postgres_data practice-postgres-data
docker network rm practice_network practice-network
```

## Menjalankan PostgreSQL

```bash
docker compose up -d practice-postgres
```

Periksa statusnya:

```bash
docker compose ps
```

Ulangi `docker compose ps` sampai service `practice-postgres` berstatus `healthy`. Kemudian pastikan PostgreSQL menerima koneksi:

```bash
docker compose exec practice-postgres pg_isready -U practice_user -d practice_server
```

Hasilnya harus memuat `accepting connections`.

## Memverifikasi database dan user

```bash
docker compose exec practice-postgres \
  psql -U practice_user -d practice_server \
  -c "SELECT current_database(), current_user;"
```

Nilai yang diharapkan:

```text
 current_database | current_user
------------------+---------------
 practice_server  | practice_user
```

Resource yang berjalan adalah container `practice-postgres`, volume `practice-postgres-data`, dan network `practice-network`. PostgreSQL memakai port `5432` di dalam container dan dapat diakses dari host melalui port `25432`.
