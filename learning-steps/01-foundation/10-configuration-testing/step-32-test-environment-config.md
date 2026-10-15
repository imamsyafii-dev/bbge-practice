# Step 32 — Menguji konfigurasi environment

Pada step sebelumnya, lifecycle HTTP server sudah dipisahkan dan diuji. Step ini mengunci kontrak konfigurasi aplikasi melalui test untuk `config.Load()`: aplikasi memakai nilai default ketika environment variable kosong dan memakai nilai environment ketika override diberikan.

Test ini tidak menjalankan server, membuka port, menghubungi PostgreSQL, atau membutuhkan Docker.

## Tujuan

Setelah menyelesaikan step ini, kita dapat:

- menguji seluruh nilai default HTTP dan PostgreSQL;
- menguji seluruh override dengan prefix `PRACTICE_`;
- memahami isolasi environment dengan `t.Setenv`;
- memahami mengapa test yang mengubah environment tidak memakai `t.Parallel`;
- memahami perilaku `os.Getenv` terhadap variable yang tidak ada dan variable bernilai string kosong;
- membandingkan struct konfigurasi secara langsung;
- menguji kontrak public `Load()` tanpa mengikat test ke helper private.

## Prasyarat

Step 1 sampai Step 31 sudah selesai. File `config/config.go` sudah memiliki:

- `PostgresConfig`;
- `Config`;
- fungsi public `Load()`;
- helper private `envOrDefault()`;
- nilai default dengan prefix environment `PRACTICE_`.

Jalankan perintah dari module server:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server
```

## File yang dibuat

```text
srv-server/config/config_test.go
```

Dokumentasi offline step ini berada di:

```text
learning-steps/01-foundation/10-configuration-testing/step-32-test-environment-config.md
```

Tidak ada perubahan pada `config/config.go` dalam step ini.

## Isi lengkap akhir `config/config_test.go`

```go
package config

import "testing"

func TestLoadUsesDefaults(
	t *testing.T,
) {
	environmentKeys := []string{
		"PRACTICE_HTTP_ADDR",
		"PRACTICE_POSTGRES_HOST",
		"PRACTICE_POSTGRES_PORT",
		"PRACTICE_POSTGRES_USER",
		"PRACTICE_POSTGRES_PASSWORD",
		"PRACTICE_POSTGRES_DB",
		"PRACTICE_POSTGRES_SSLMODE",
	}

	for _, key := range environmentKeys {
		t.Setenv(key, "")
	}

	got := Load()

	want := Config{
		HTTPAddr: ":18080",
		Postgres: PostgresConfig{
			Host:     "127.0.0.1",
			Port:     "25432",
			User:     "practice_user",
			Password: "practice_password",
			Database: "practice_server",
			SSLMode:  "disable",
		},
	}

	if got != want {
		t.Fatalf(
			"Load() = %#v, want %#v",
			got,
			want,
		)
	}
}

func TestLoadUsesEnvironmentOverrides(
	t *testing.T,
) {
	t.Setenv(
		"PRACTICE_HTTP_ADDR",
		":28080",
	)
	t.Setenv(
		"PRACTICE_POSTGRES_HOST",
		"practice-postgres-host",
	)
	t.Setenv(
		"PRACTICE_POSTGRES_PORT",
		"35432",
	)
	t.Setenv(
		"PRACTICE_POSTGRES_USER",
		"practice_override_user",
	)
	t.Setenv(
		"PRACTICE_POSTGRES_PASSWORD",
		"practice_override_password",
	)
	t.Setenv(
		"PRACTICE_POSTGRES_DB",
		"practice_override_database",
	)
	t.Setenv(
		"PRACTICE_POSTGRES_SSLMODE",
		"require",
	)

	got := Load()

	want := Config{
		HTTPAddr: ":28080",
		Postgres: PostgresConfig{
			Host:     "practice-postgres-host",
			Port:     "35432",
			User:     "practice_override_user",
			Password: "practice_override_password",
			Database: "practice_override_database",
			SSLMode:  "require",
		},
	}

	if got != want {
		t.Fatalf(
			"Load() = %#v, want %#v",
			got,
			want,
		)
	}
}
```

## Test pertama: nilai default

`TestLoadUsesDefaults` memeriksa kontrak default aplikasi. Semua environment variable yang dibaca `Load()` terlebih dahulu diberi nilai string kosong:

```go
for _, key := range environmentKeys {
	t.Setenv(key, "")
}
```

Ini mencegah environment terminal atau mesin pengembang memengaruhi hasil test. Sesudah itu `Load()` harus menghasilkan:

```text
HTTP address       : :18080
PostgreSQL host    : 127.0.0.1
PostgreSQL port    : 25432
PostgreSQL user    : practice_user
PostgreSQL password: practice_password
PostgreSQL database: practice_server
PostgreSQL sslmode : disable
```

Test sengaja memeriksa semua field. Jika sebuah default berubah tanpa sengaja, test menunjukkan bahwa kontrak konfigurasi juga berubah.

## Test kedua: environment override

`TestLoadUsesEnvironmentOverrides` memberikan nilai berbeda untuk setiap variable:

```text
PRACTICE_HTTP_ADDR
PRACTICE_POSTGRES_HOST
PRACTICE_POSTGRES_PORT
PRACTICE_POSTGRES_USER
PRACTICE_POSTGRES_PASSWORD
PRACTICE_POSTGRES_DB
PRACTICE_POSTGRES_SSLMODE
```

Kemudian test memanggil `Load()` dan membandingkan hasilnya dengan `Config` yang berisi semua nilai override. Dengan demikian test membuktikan dua hal sekaligus:

1. nama environment variable dipetakan ke field yang benar;
2. nilai environment memiliki prioritas di atas nilai default.

Nilai override memakai nama yang jelas berbeda dari default agar kesalahan pemetaan mudah terlihat. Seluruh nama tetap memakai prefix `PRACTICE_`, sehingga tidak bertabrakan dengan konfigurasi BBGE utama.

## Cara kerja cleanup `t.Setenv`

`t.Setenv(key, value)` mengubah environment untuk proses test, lalu mendaftarkan cleanup otomatis. Setelah test selesai—baik lulus, gagal melalui `t.Fatal`, maupun berhenti karena panic—Go mengembalikan variable tersebut ke keadaan sebelumnya.

Karena cleanup otomatis, kita tidak perlu menulis pasangan manual seperti:

```go
oldValue := os.Getenv(key)
os.Setenv(key, value)
defer os.Setenv(key, oldValue)
```

Cara manual itu lebih panjang dan mudah salah, khususnya karena hanya menyimpan string tidak cukup untuk membedakan variable yang semula tidak ada dari variable yang semula ada tetapi kosong. `t.Setenv` menangani pemulihan tersebut untuk test.

Cleanup otomatis tidak berarti environment menjadi lokal untuk satu goroutine. Environment tetap merupakan state global milik proses test selama test berjalan.

## Mengapa tidak memakai `t.Parallel()`

Kedua test memodifikasi environment process. Jika test tersebut berjalan paralel, satu test dapat mengganti nilai ketika test lain sedang memanggil `Load()`. Hasilnya dapat bergantung pada timing dan menjadi flaky.

Go juga melarang pemakaian `t.Setenv` pada test paralel atau test yang memiliki parallel ancestor karena environment memengaruhi seluruh proses. Oleh karena itu test ini sengaja berjalan serial dan tidak memanggil:

```go
t.Parallel()
```

## `os.Getenv`: unset dan empty menghasilkan nilai yang sama

Helper saat ini membaca environment dengan:

```go
value := os.Getenv(key)
if value == "" {
	return fallback
}
```

`os.Getenv` hanya mengembalikan sebuah string. Hasilnya adalah `""` dalam dua keadaan:

- variable tidak diatur atau unset;
- variable diatur, tetapi nilainya string kosong.

Karena itu `envOrDefault` sengaja memperlakukan kedua keadaan tersebut sama-sama sebagai permintaan untuk memakai fallback. Inilah alasan test default dapat memakai `t.Setenv(key, "")` dan tetap menguji branch fallback.

Jika aplikasi kelak harus membedakan unset dari empty, gunakan `os.LookupEnv`, yang mengembalikan `(value string, found bool)`. Itu bukan kontrak konfigurasi pada step ini.

## Mengapa struct dapat dibandingkan langsung

Assertion memakai:

```go
if got != want {
```

Go mengizinkan struct dibandingkan jika seluruh field-nya comparable. `Config` hanya berisi:

- `HTTPAddr` dengan tipe `string`;
- `Postgres` dengan tipe `PostgresConfig`.

Seluruh field di dalam `PostgresConfig` juga bertipe `string`. String comparable, sehingga kedua struct tersebut juga comparable. Perbandingan langsung mencakup semua field tanpa library assertion tambahan.

Jika suatu saat struct ditambah slice, map, atau function, operator `==` dan `!=` tidak lagi dapat dipakai untuk keseluruhan struct. Test saat itu perlu membandingkan field secara terpisah atau memakai mekanisme perbandingan lain.

Format `%#v` pada pesan kegagalan menampilkan representasi Go yang rinci untuk `got` dan `want`, sehingga field yang berbeda lebih mudah diperiksa.

## Mengapa menguji `Load()`, bukan helper private

`envOrDefault()` adalah detail implementasi private. Kontrak yang benar-benar dipakai oleh aplikasi adalah fungsi public:

```go
func Load() Config
```

Dengan memanggil `Load()`, test mencakup alur nyata:

```text
environment variable
        ↓
envOrDefault
        ↓
pemetaan ke Config dan PostgresConfig
        ↓
hasil yang dipakai main
```

Jika test hanya memanggil `envOrDefault`, kesalahan seperti nama key yang salah, field yang tertukar, atau default yang salah di dalam `Load()` dapat lolos. Menguji API public juga membuat helper private bebas direfactor selama perilaku `Load()` tetap sama.

## Memformat dan menjalankan test

Format file test:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server
gofmt -w config/config_test.go
```

Jalankan hanya test konfigurasi yang namanya dimulai dengan `TestLoad`:

```bash
go test ./config -run '^TestLoad' -count=1 -v
```

Arti option yang dipakai:

- `./config` memilih package konfigurasi;
- `-run '^TestLoad'` memilih kedua test pada step ini;
- `-count=1` menonaktifkan pemakaian hasil test cache;
- `-v` menampilkan nama setiap test.

Setelah focused test lulus, jalankan seluruh test module:

```bash
go test ./...
```

## Hasil yang diamati

Focused test menghasilkan:

```text
=== RUN   TestLoadUsesDefaults
--- PASS: TestLoadUsesDefaults (0.00s)
=== RUN   TestLoadUsesEnvironmentOverrides
--- PASS: TestLoadUsesEnvironmentOverrides (0.00s)
PASS
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/config  0.006s
```

Full test juga lulus untuk package root, `config`, `internal/app/account`, `internal/domain/account`, `internal/router`, dan `internal/store`. Package `db` serta `pkg/postgres` belum mempunyai test dan dilaporkan sebagai `[no test files]`.

Waktu test dapat berbeda antar-mesin. Yang penting adalah status `PASS` dan seluruh package selesai tanpa error.

## Troubleshooting

### Test default menerima nilai dari mesin

Pastikan semua key yang dibaca `Load()` terdaftar pada `environmentKeys` dan dikosongkan melalui `t.Setenv` sebelum memanggil `Load()`. Jangan mengandalkan environment terminal sudah bersih.

### Test override masih menerima default

Periksa ejaan key pada test dan `config.go`. Nama bersifat case-sensitive. Pastikan juga `t.Setenv` dipanggil sebelum `got := Load()`.

### Error ketika menambahkan `t.Parallel()`

Hapus `t.Parallel()`. Test yang memakai `t.Setenv` tidak aman dijalankan paralel karena environment adalah state global proses.

### `invalid operation: got != want`

Periksa apakah `Config` atau struct di dalamnya telah mempunyai field slice, map, atau function. Tipe-tipe tersebut tidak comparable. Pada source Step 32, seluruh field masih berupa string atau struct yang seluruh field-nya string.

### Focused test tidak menemukan test

Pastikan command dijalankan dari `srv-server`, nama file berakhiran `_test.go`, nama function dimulai dengan `Test`, dan regular expression ditulis tepat:

```bash
go test ./config -run '^TestLoad' -count=1 -v
```

### Nilai kosong ternyata memakai default

Itu adalah perilaku yang disengaja pada step ini. `os.Getenv` mengembalikan `""` untuk variable unset maupun empty dan `envOrDefault` memilih fallback ketika hasilnya kosong.

## Batas Step 32

Step ini hanya menguji pemuatan konfigurasi berbasis string. Step ini belum:

- memvalidasi format address atau port;
- mengubah port menjadi integer;
- membaca file `.env` secara otomatis;
- menyembunyikan atau mengenkripsi password;
- membuka koneksi PostgreSQL;
- menguji Docker Compose;
- mengubah implementasi `config.Load()`.

Password test di sini adalah credential lokal latihan, bukan secret production. Kode aplikasi tetap tidak boleh mencetak password atau connection URL yang mengandung password ke log.

## Ringkasan

```text
config/config_test.go
├── TestLoadUsesDefaults
│   ├── menetralkan tujuh environment variable
│   └── memeriksa seluruh nilai fallback
└── TestLoadUsesEnvironmentOverrides
    ├── memberi tujuh override ber-prefix PRACTICE_
    └── memeriksa seluruh pemetaan ke Config
```

Konsep yang dipelajari:

- public behavior testing;
- environment sebagai global process state;
- cleanup otomatis melalui `t.Setenv`;
- alasan tidak menjalankan environment test secara paralel;
- perilaku `os.Getenv` untuk unset dan empty;
- perbandingan struct yang comparable;
- isolasi test dari konfigurasi mesin pengembang.

## Step 33 — Preview

Roadmap rinci Step 33 belum tersimpan pada dokumentasi yang tersedia saat Step 32 ditulis. Kelanjutan yang masuk akal adalah menguji boundary database yang masih belum mempunyai test, dimulai dari fungsi migration atau pembukaan PostgreSQL pool.

Instruksi final Step 33 akan ditentukan setelah source dan roadmap diperiksa pada awal step berikutnya. Tujuannya tetap sama: menambah test pada boundary fondasi tanpa memasukkan perubahan fitur baru.
