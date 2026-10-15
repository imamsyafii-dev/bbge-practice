# Step 5 — Menyiapkan PostgreSQL dengan Docker Compose

Buat direktori `docker` di dalam project server:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server
mkdir -p docker
```

Buat `docker/docker-compose.yml` dengan isi berikut:

```yaml
name: practice-srv-server

services:
  practice-postgres:
    container_name: practice-postgres
    image: postgres:18.3
    restart: unless-stopped
    ports:
      - "${PRACTICE_POSTGRES_PORT:-25432}:5432"
    environment:
      POSTGRES_USER: ${PRACTICE_POSTGRES_USER:-practice_user}
      POSTGRES_PASSWORD: ${PRACTICE_POSTGRES_PASSWORD:-practice_password}
      POSTGRES_DB: ${PRACTICE_POSTGRES_DB:-practice_server}
    volumes:
      - practice-postgres-data:/var/lib/postgresql
    networks:
      - practice-network
    healthcheck:
      test:
        [
          "CMD-SHELL",
          "pg_isready -U ${PRACTICE_POSTGRES_USER:-practice_user} -d ${PRACTICE_POSTGRES_DB:-practice_server}",
        ]
      interval: 3s
      timeout: 5s
      retries: 20

volumes:
  practice-postgres-data:
    name: practice-postgres-data

networks:
  practice-network:
    name: practice-network
```

Sintaks `${NAMA_VARIABLE:-nilai_bawaan}` memakai environment variable jika tersedia dan memakai nilai setelah `:-` sebagai fallback jika variable tidak diatur atau kosong. Karena itu, konfigurasi ini memakai port host `25432`, user `practice_user`, password `practice_password`, dan database `practice_server` secara default. Port container tetap `5432`.

Entri `volumes` dan `networks` di dalam service menghubungkan container ke volume dan network. Bagian top-level `volumes` dan `networks` mendeklarasikan resource tersebut. Properti `name` memberi nama Docker yang eksplisit, yaitu `practice-postgres-data` dan `practice-network`, agar tidak otomatis diberi prefix nama project.

Validasi konfigurasi dari direktori `docker`:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server/docker
docker compose config
```

Perintah tersebut harus berhasil dan menampilkan konfigurasi yang sudah di-resolve. Jangan jalankan `docker compose up` pada langkah ini. Jika container dari latihan sebelumnya masih ada, penanganannya dilakukan pada Step 6.
