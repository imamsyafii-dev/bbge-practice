# Step 36 — Contract registrasi identity

Step ini memulai fase identity dan authentication dengan contract registrasi practice yang kecil dan dapat diuji tanpa infrastructure. Contract ini adalah penyederhanaan yang diekstrak dari shared inner registration validation di `bbge-biz`; ia **bukan** contract lengkap password registration production yang aktif.

## Tujuan

Setelah step ini, kita mempunyai:

- tipe input registrasi milik domain practice;
- validasi password dan device ID wajib;
- validasi panjang public key X25519 dan Ed25519;
- sentinel error yang dapat diperiksa memakai `errors.Is`;
- table-driven test untuk jalur valid dan setiap input invalid.

Step ini sengaja belum membuat database, migration, repository, password hash, application use case, router, HTTP endpoint, access token, atau refresh token.

## Prasyarat

- Milestone `01-foundation` Step 1–35 sudah selesai.
- Module Go berada di `practice/srv-server`.
- Seluruh test fondasi masih lulus.

## Temuan dari `bbge-biz` aktif

Contract protobuf production masih memuat field legacy seperti `name` dan `email`. Akan tetapi, alur registrasi aktif tidak memakai nilai keduanya sebagai identity yang dipilih client:

- `user_id` dibuat oleh server dari internal UID;
- email compatibility dibuat oleh server dari `user_id`, bukan dari request;
- nama awal dibuat oleh server, bukan dari `req.name`;
- validator domain aktif memeriksa password, device ID, public key X25519, dan public key Ed25519;
- router juga memverifikasi panjang key serta signed device payload sebelum memanggil use case.

Jadi, keberadaan field pada protobuf atau database tidak otomatis berarti field tersebut masih menjadi input bisnis aktif. Practice menghilangkan email dari contract baru agar tidak mengajarkan residue legacy sebagai requirement baru.

Namun, endpoint password modern aktif bukan sekadar `RegisterInput` pada step ini. Secara konseptual, `/identity/password/register` memakai wrapper yang memuat:

- `username` sebagai login identifier;
- `password` yang divalidasi lalu di-hash sebelum persistence;
- nested registration untuk metadata device dan public identity keys;
- encrypted identity backup yang terikat pada kedua public key;
- `request_id` untuk mengenali retry setelah registrasi sudah committed;
- idempotent retry: request yang sama dapat melanjutkan account yang sudah dibuat, sedangkan request yang tidak cocok menjadi conflict.

Router production juga mewajibkan transport aman, rate limiting, serta proof-of-possession melalui signed device payload. Karena Step 36 hanya mengunci bagian shared inner validation, fitur wrapper dan hashing dipelajari bertahap pada step berikutnya. Proof-of-possession/signature verification dan encrypted identity backup berada di luar Fase 02.

Rujukan source lokal ketika step ini dibuat:

```text
bbge-biz/proto/proto/bbge/identity/identity.proto
bbge-biz/proto/proto/bbge/identity/password.proto
bbge-biz/srv-server/internal/domain/identity/service.go
bbge-biz/srv-server/internal/domain/identity/password.go
bbge-biz/srv-server/internal/router/identity.go
bbge-biz/srv-server/internal/router/identity_password.go
```

## Ownership field `RegisterInput`

| Field | Pemilik nilai | Makna pada Step 36 |
|---|---|---|
| `Password` | Client | Secret sementara untuk registrasi; hashing dipelajari setelah contract ini stabil. |
| `DeviceID` | Client/device | Identifier device yang mengajukan registrasi. |
| `DeviceName` | Client/device | Metadata device; belum diberi aturan wajib. |
| `DeviceModel` | Client/device | Metadata model; belum diberi aturan wajib. |
| `DeviceFlag` | Client/device | Kategori/platform device; validasi nilai dipelajari bersama model device. |
| `IdentifierX25519` | Client/device | Public key 32 byte untuk key agreement. |
| `IdentifierEd25519` | Client/device | Public key 32 byte untuk signature verification. |

Field yang sengaja tidak ada:

- `Email`: bukan identity registrasi practice.
- `UserID`: harus dibuat server, bukan dipilih client.
- `Name`: profile awal tidak dijadikan credential; source aktif juga membuat nama awal di server.
- Password hash: bukan input dari client dan kelak menjadi credential persistence terpisah.
- Username dan registration request ID: milik wrapper password registration, bukan shared inner input pada Step 36; diperkenalkan pada Step 37.
- Token/transient session ID: harus diterbitkan server setelah registrasi berhasil.

Model production aktif menggunakan **device-as-session**, bukan tabel session umum terpisah. Record device dimiliki account dan membawa refresh token, session status, issued-at, serta expired-at. Jika access token kelak memiliki `session_id`, ID tersebut dapat menjadi claim transient tanpa berarti ada entity session yang dipersist terpisah.

## File yang dibuat

```text
srv-server/internal/domain/identity/registration.go
srv-server/internal/domain/identity/registration_test.go
```

## Isi final `registration.go`

```go
package identity

import (
	"crypto/ed25519"
	"errors"
	"strings"
)

const x25519PublicKeySize = 32

var (
	ErrPasswordRequired = errors.New(
		"password is required",
	)
	ErrDeviceIDRequired = errors.New(
		"device id is required",
	)
	ErrIdentityIdentifiersInvalid = errors.New(
		"identity identifiers are invalid",
	)
)

type RegisterInput struct {
	Password          string
	DeviceID          string
	DeviceName        string
	DeviceModel       string
	DeviceFlag        int32
	IdentifierX25519  []byte
	IdentifierEd25519 []byte
}

func validateRegisterInput(
	input RegisterInput,
) error {
	if strings.TrimSpace(input.Password) == "" {
		return ErrPasswordRequired
	}

	if strings.TrimSpace(input.DeviceID) == "" {
		return ErrDeviceIDRequired
	}

	if len(input.IdentifierX25519) !=
		x25519PublicKeySize ||
		len(input.IdentifierEd25519) !=
			ed25519.PublicKeySize {
		return ErrIdentityIdentifiersInvalid
	}

	return nil
}
```

## Mengapa public key memakai `[]byte`?

Key pada wire protocol diterima sebagai bytes, bukan teks biasa. `[]byte` menjaga representasi binary dan memungkinkan panjang key diperiksa langsung dengan `len`.

Pada step ini validasi hanya memeriksa bentuk paling dasar:

```text
X25519 public key  = 32 byte
Ed25519 public key = 32 byte
```

Konstanta X25519 ditulis lokal karena standard library Go tidak menyediakan konstanta public-key-size X25519 dengan bentuk yang dipakai di sini. Untuk Ed25519, production menggunakan konstanta resmi:

```go
ed25519.PublicKeySize
```

Menggunakan konstanta package crypto menghindari magic number dan menyatakan maksud kode secara eksplisit.

Pemeriksaan panjang belum membuktikan bahwa key benar-benar dibuat secara kriptografis, dimiliki oleh pengirim, atau cocok dengan signature. Proof-of-possession dan verifikasi signed payload berada di luar Step 36 dan di luar scope Fase 02. Encrypted identity backup juga tidak dibangun pada fase ini. Karena itu latihan tidak boleh dianggap memiliki keamanan registrasi yang setara dengan production.

## Mengapa memakai `strings.TrimSpace`?

String seperti berikut tidak boleh dianggap password atau device ID yang valid:

```text
""
"   "
"\t"
"\n"
```

`strings.TrimSpace` membuang whitespace di awal dan akhir untuk keperluan pemeriksaan kosong. Fungsi ini tidak mengubah nilai asli di dalam `RegisterInput` karena input diterima by value dan tidak di-assign kembali.

Password belum dinormalisasi. Untuk password, normalisasi diam-diam dapat mengubah secret pengguna. Step ini hanya menolak nilai yang seluruhnya whitespace; kebijakan panjang dan hashing dibahas pada step berikutnya.

## Sentinel error dan `errors.Is`

Ketiga error dibuat satu kali pada level package:

```go
var ErrPasswordRequired = errors.New("password is required")
```

Nilai stabil ini dapat diperiksa oleh layer lain:

```go
if errors.Is(err, ErrPasswordRequired) {
	// map ke response yang sesuai
}
```

`errors.Is` tetap bekerja jika pada masa mendatang error diberi konteks dengan `%w`. Membandingkan teks `err.Error()` akan lebih rapuh karena teks konteks dapat berubah.

## Isi final `registration_test.go`

```go
package identity

import (
	"errors"
	"testing"
)

func TestValidateRegisterInput(
	t *testing.T,
) {
	tests := []struct {
		name    string
		change  func(*RegisterInput)
		wantErr error
	}{
		{
			name: "valid input",
		},
		{
			name: "password is empty",
			change: func(input *RegisterInput) {
				input.Password = "   "
			},
			wantErr: ErrPasswordRequired,
		},
		{
			name: "device ID is empty",
			change: func(input *RegisterInput) {
				input.DeviceID = "\t"
			},
			wantErr: ErrDeviceIDRequired,
		},
		{
			name: "X25519 identifier has invalid size",
			change: func(input *RegisterInput) {
				input.IdentifierX25519 = make(
					[]byte,
					x25519PublicKeySize-1,
				)
			},
			wantErr: ErrIdentityIdentifiersInvalid,
		},
		{
			name: "Ed25519 identifier has invalid size",
			change: func(input *RegisterInput) {
				input.IdentifierEd25519 = make(
					[]byte,
					ed25519PublicKeySizeForTest-1,
				)
			},
			wantErr: ErrIdentityIdentifiersInvalid,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				input := validRegisterInput()

				if test.change != nil {
					test.change(&input)
				}

				err := validateRegisterInput(input)

				if test.wantErr == nil {
					if err != nil {
						t.Fatalf(
							"validateRegisterInput() error = %v, want nil",
							err,
						)
					}

					return
				}

				if !errors.Is(err, test.wantErr) {
					t.Fatalf(
						"validateRegisterInput() error = %v, want %v",
						err,
						test.wantErr,
					)
				}
			},
		)
	}
}

const ed25519PublicKeySizeForTest = 32

func validRegisterInput() RegisterInput {
	return RegisterInput{
		Password:    "practice-password",
		DeviceID:    "practice-device-001",
		DeviceName:  "Practice Phone",
		DeviceModel: "Practice Model",
		DeviceFlag:  1,
		IdentifierX25519: make(
			[]byte,
			x25519PublicKeySize,
		),
		IdentifierEd25519: make(
			[]byte,
			ed25519PublicKeySizeForTest,
		),
	}
}
```

## Struktur table-driven test

Satu tabel mewakili semua skenario dengan tiga field:

- `name` menjadi nama subtest;
- `change` mengubah satu bagian dari input valid;
- `wantErr` menyatakan sentinel error yang diharapkan.

Setiap subtest dimulai dari fixture valid:

```go
input := validRegisterInput()
```

Kemudian fungsi `change` hanya merusak satu kondisi:

```go
change: func(input *RegisterInput) {
	input.DeviceID = "\t"
}
```

Parameter berupa `*RegisterInput` diperlukan karena fungsi tersebut harus mengubah fixture milik subtest. Tanpa pointer, fungsi menerima salinan struct dan perubahan tidak terlihat oleh pemanggil.

Pada kasus `valid input`, `change` dan `wantErr` memakai zero value `nil`. Karena itu test memastikan hasilnya juga `nil`. Pada kasus invalid, test memakai `errors.Is`, bukan membandingkan string error.

Test tidak memakai `t.Parallel()`. Saat ini sebenarnya setiap fixture berdiri sendiri, tetapi eksekusi serial membuat tahap awal lebih mudah dibaca dan tidak memberi asumsi concurrency sebelum dependency berikutnya diketahui.

## Perintah format dan test

```bash
cd /Users/mac/Documents/bbge/practice/srv-server

gofmt -w \
  internal/domain/identity/registration.go \
  internal/domain/identity/registration_test.go

go test ./internal/domain/identity \
  -run '^TestValidateRegisterInput$' \
  -count=1 \
  -v

go test ./...
```

## Hasil verifikasi aktual

Focused test yang dijalankan pada Step 36:

```text
=== RUN   TestValidateRegisterInput
=== RUN   TestValidateRegisterInput/valid_input
=== RUN   TestValidateRegisterInput/password_is_empty
=== RUN   TestValidateRegisterInput/device_ID_is_empty
=== RUN   TestValidateRegisterInput/X25519_identifier_has_invalid_size
=== RUN   TestValidateRegisterInput/Ed25519_identifier_has_invalid_size
--- PASS: TestValidateRegisterInput (0.00s)
    --- PASS: TestValidateRegisterInput/valid_input (0.00s)
    --- PASS: TestValidateRegisterInput/password_is_empty (0.00s)
    --- PASS: TestValidateRegisterInput/device_ID_is_empty (0.00s)
    --- PASS: TestValidateRegisterInput/X25519_identifier_has_invalid_size (0.00s)
    --- PASS: TestValidateRegisterInput/Ed25519_identifier_has_invalid_size (0.00s)
PASS
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/domain/identity
```

Full test `go test ./...` juga lulus untuk package root, config, database, account domain/application, identity domain, router, store, dan PostgreSQL helper package tanpa test.

## Batas Step 36

Lulusnya test ini hanya membuktikan validasi contract in-memory. Ia belum membuktikan:

- password mempunyai kebijakan panjang yang cukup;
- password telah di-hash atau aman untuk disimpan;
- public key dimiliki oleh client;
- device flag valid;
- account atau device-as-session berhasil disimpan;
- operasi registrasi atomic;
- token dapat diterbitkan atau diverifikasi;
- endpoint HTTP dapat dipanggil.

Jangan menyimpan `RegisterInput.Password` langsung ke tabel account atau credential.

## Troubleshooting

### `undefined: ed25519`

Pastikan production file mengimpor:

```go
import "crypto/ed25519"
```

### Test valid malah mendapat `ErrIdentityIdentifiersInvalid`

Pastikan fixture membuat kedua slice sepanjang 32 byte. `make([]byte, 32)` menghasilkan slice berisi zero bytes, tetapi panjangnya valid untuk test bentuk. Itu bukan key nyata untuk production.

### Kasus whitespace tidak gagal

Pastikan pengecekan menggunakan:

```go
strings.TrimSpace(value) == ""
```

bukan hanya `value == ""`.

### Perubahan pada `change` tidak berpengaruh

Pastikan tipe callback adalah:

```go
func(*RegisterInput)
```

dan dipanggil dengan alamat fixture:

```go
test.change(&input)
```

### `errors.Is` menghasilkan `false`

Kembalikan sentinel error package yang sama. Jika error diberi konteks, gunakan `%w`, bukan `%v`.

## Step berikutnya

Step 37 memperkenalkan wrapper password registration practice: **username**, nested registration, **registration request ID**, aturan retry idempotent, dan password policy. Encrypted identity backup tetap di luar fase. Setelah contract stabil, Step 38 membuat password credential dan hasher boundary, kemudian Step 39 mengimplementasikan serta menguji hashing sebelum registrasi boleh dipersist.
