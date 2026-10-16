# Step 37 — Password registration wrapper dan policy

Step ini membentuk contract luar untuk registrasi menggunakan username dan password. Password yang pada Step 36 masih berada di inner `RegisterInput` dipindahkan ke `PasswordRegisterInput`, sehingga ownership setiap data menjadi lebih jelas.

Inner registration sekarang hanya memuat identitas device dan public identity key. Outer wrapper memuat credential login serta `request_id` yang kelak dipakai untuk mengenali retry registrasi.

## Tujuan

Setelah step ini, practice mempunyai:

- `PasswordRegisterInput` sebagai wrapper registrasi password;
- username ASCII sepanjang 3–32 byte yang dinormalisasi menjadi lowercase;
- password UTF-8 valid sepanjang 15–128 Unicode code points/runes;
- registration request ID sepanjang 16–64 byte tanpa whitespace di tepi;
- nested `RegisterInput` yang hanya bertanggung jawab atas device dan public key;
- sentinel error stabil yang dapat diperiksa dengan `errors.Is`;
- validator yang menghasilkan salinan ternormalisasi tanpa mengubah input asli;
- test untuk batas minimum, batas maksimum, normalisasi, invalid UTF-8, nested validation, username input yang tetap, dan password result yang dipertahankan.

Step ini masih murni domain contract. Belum ada hashing, database, transaction, persistence, atau perilaku retry.

## Temuan dari source `bbge-biz` aktif

Contract endpoint password production berada pada:

```text
bbge-biz/proto/proto/bbge/identity/password.proto
bbge-biz/srv-server/internal/domain/identity/password.go
bbge-biz/srv-server/internal/domain/identity/password_crypto.go
```

Secara konseptual, `PasswordRegisterRequest` production mempunyai:

```text
username
password
registration
backup
request_id
```

Alur aktif production melakukan beberapa pekerjaan:

1. menormalisasi username;
2. memvalidasi password;
3. memastikan nested registration tersedia;
4. memvalidasi encrypted identity backup terhadap public key;
5. memvalidasi `request_id`;
6. memeriksa credential yang mungkin sudah ada;
7. melanjutkan retry yang cocok tanpa membuat account kedua atau mengembalikan conflict;
8. melakukan password hashing;
9. membuat account, credential, dan device-as-session.

Step 37 hanya mengambil bagian contract dan policy dari alur itu. Pada source aktif, retry dianggap cocok bila `request_id` dan identity identifier X25519 serta Ed25519 cocok dengan data yang tersimpan; source tidak membandingkan seluruh field request. Encrypted identity backup, hashing, persistence, serta retry aktual sengaja belum diimplementasikan.

### Legacy inner password marker di production

Shared inner registration validator production masih mengharuskan field password. Ketika endpoint password modern memanggil shared registration tersebut, source aktif membuat clone dari nested registration lalu mengisi password dengan marker internal:

```go
registration.Password = PasswordAuthSource
```

Komentar source production menjelaskan bahwa marker itu hanya diperlukan oleh legacy registration validator dan tidak disimpan sebagai PIN. Artinya, inner password tersebut merupakan compatibility residue, bukan lokasi ideal untuk password login modern.

Practice tidak membawa residue itu ke desain baru. Password asli hanya berada di outer `PasswordRegisterInput`. Inner `RegisterInput` tidak mempunyai field password.

## Refactor dari Step 36 ke Step 37

Pada akhir Step 36, bentuk input masih seperti ini secara konseptual:

```text
RegisterInput
├── Password
├── DeviceID
├── DeviceName
├── DeviceModel
├── DeviceFlag
├── IdentifierX25519
└── IdentifierEd25519
```

Setelah Step 37, bentuknya menjadi:

```text
PasswordRegisterInput
├── Username
├── Password
├── RequestID
└── Registration
    ├── DeviceID
    ├── DeviceName
    ├── DeviceModel
    ├── DeviceFlag
    ├── IdentifierX25519
    └── IdentifierEd25519
```

Pemisahan ini memberi ownership yang lebih jelas:

| Data | Pemilik contract | Alasan |
|---|---|---|
| `Username` | Outer password registration | Credential login, bukan metadata device. |
| `Password` | Outer password registration | Secret login yang kelak diberikan kepada password hasher. |
| `RequestID` | Outer password registration | Identitas satu percobaan registrasi untuk idempotency. |
| Device metadata | Inner registration | Menjelaskan device yang memulai registrasi. |
| Public identity keys | Inner registration | Bagian identity/device, bukan password credential. |

Dengan bentuk ini, validator inner dapat digunakan tanpa mengetahui metode autentikasi. Metode autentikasi lain di masa depan tidak harus berpura-pura mempunyai password hanya untuk lolos dari inner validation.

## Scope dan batas Step 37

Yang dikerjakan:

- refactor ownership password;
- wrapper password registration;
- username normalization dan validation;
- password shape policy;
- request ID shape validation;
- delegasi ke nested registration validator;
- unit test domain.

Yang belum dikerjakan:

- password hashing atau verification;
- bentuk password credential yang dipersist;
- account, credential, atau device persistence;
- database migration dan SQLC;
- transaction;
- lookup berdasarkan username;
- deteksi username conflict;
- retry yang mencocokkan request ID dan identity identifier dengan data tersimpan;
- encrypted identity backup;
- proof-of-possession atau signature verification;
- HTTP request/response;
- token atau session issuance.

Walaupun `RequestID` sudah ada dan divalidasi, Step 37 **belum idempotent**. Perilaku retry membutuhkan persistence, unique constraint, transaction, serta pencocokan request ID dan identity identifier retry terhadap data yang tersimpan.

## Struktur file setelah Step 37

```text
srv-server/
└── internal/
    └── domain/
        └── identity/
            ├── password_registration.go
            ├── password_registration_test.go
            ├── registration.go
            └── registration_test.go
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
	ErrDeviceIDRequired = errors.New(
		"device id is required",
	)
	ErrIdentityIdentifiersInvalid = errors.New(
		"identity identifiers are invalid",
	)
)

type RegisterInput struct {
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

Perubahan terpenting dibanding Step 36 adalah penghapusan:

```go
Password string
```

dari `RegisterInput`, beserta `ErrPasswordRequired` dan validasi password pada inner validator. Password sekarang sepenuhnya menjadi tanggung jawab wrapper luar.

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

Fixture `validRegisterInput` tidak lagi membuat password. Test inner registration hanya menguji tanggung jawab inner registration: device ID dan panjang kedua public key.

## Isi final `password_registration.go`

```go
package identity

import (
	"errors"
	"strings"
	"unicode/utf8"
)

const (
	minLoginUsernameLength = 3
	maxLoginUsernameLength = 32
	minLoginPasswordLength = 15
	maxLoginPasswordLength = 128
	minRequestIDLength     = 16
	maxRequestIDLength     = 64
)

var (
	ErrUsernameInput = errors.New(
		"username must contain 3 to 32 ASCII letters, digits or underscores",
	)
	ErrPasswordInput = errors.New(
		"password must contain 15 to 128 characters",
	)
	ErrRegistrationRequestIDInvalid = errors.New(
		"registration request id is invalid",
	)
)

type PasswordRegisterInput struct {
	Username     string
	Password     string
	Registration RegisterInput
	RequestID    string
}

func NormalizeLoginUsername(
	value string,
) (string, error) {
	if len(value) < minLoginUsernameLength ||
		len(value) > maxLoginUsernameLength {
		return "", ErrUsernameInput
	}

	for _, character := range value {
		isLowercaseLetter :=
			character >= 'a' && character <= 'z'
		isUppercaseLetter :=
			character >= 'A' && character <= 'Z'
		isNumber :=
			character >= '0' && character <= '9'
		isUnderscore := character == '_'

		if !isLowercaseLetter &&
			!isUppercaseLetter &&
			!isNumber &&
			!isUnderscore {
			return "", ErrUsernameInput
		}
	}

	return strings.ToLower(value), nil
}

func validateLoginPassword(
	value string,
) error {
	if !utf8.ValidString(value) {
		return ErrPasswordInput
	}

	length := utf8.RuneCountInString(value)

	if length < minLoginPasswordLength ||
		length > maxLoginPasswordLength {
		return ErrPasswordInput
	}

	return nil
}

func validatePasswordRegisterInput(
	input PasswordRegisterInput,
) (PasswordRegisterInput, error) {
	username, err :=
		NormalizeLoginUsername(input.Username)
	if err != nil {
		return PasswordRegisterInput{}, err
	}

	if err := validateLoginPassword(
		input.Password,
	); err != nil {
		return PasswordRegisterInput{}, err
	}

	if len(input.RequestID) < minRequestIDLength ||
		len(input.RequestID) > maxRequestIDLength ||
		strings.TrimSpace(input.RequestID) !=
			input.RequestID {
		return PasswordRegisterInput{},
			ErrRegistrationRequestIDInvalid
	}

	if err := validateRegisterInput(
		input.Registration,
	); err != nil {
		return PasswordRegisterInput{}, err
	}

	input.Username = username

	return input, nil
}
```

## Wrapper `PasswordRegisterInput`

Wrapper menyatukan dua kelompok data:

```go
type PasswordRegisterInput struct {
	Username     string
	Password     string
	Registration RegisterInput
	RequestID    string
}
```

- `Username` dan `Password` adalah bahan password credential.
- `Registration` adalah nested value untuk device dan public identity keys.
- `RequestID` adalah identitas permintaan registrasi.

Nilai ini masih contract domain, bukan database model. Plaintext password hanya boleh hidup pada boundary proses registrasi dan kelak harus diubah menjadi hash sebelum persistence.

## Username: validasi lalu normalisasi

Username menerima karakter berikut:

```text
a-z
A-Z
0-9
_
```

Karakter spasi, tanda hubung, dan karakter non-ASCII ditolak. Setelah valid, username diubah menjadi lowercase:

```text
Practice_User123 -> practice_user123
```

Normalisasi memberi bentuk canonical sehingga perbedaan kapital tidak membuat dua credential login berbeda.

Panjang username diperiksa dengan `len`. Hal ini tepat untuk contract tersebut karena username hanya boleh berisi ASCII; setiap karakter ASCII berukuran satu byte. Pemeriksaan karakter setelahnya memastikan input non-ASCII tetap ditolak.

Username tidak di-`TrimSpace`. Jika input mempunyai whitespace di tepi, whitespace tersebut dianggap karakter invalid. Validator tidak diam-diam memperbaiki username yang salah.

## Password: valid UTF-8 dan jumlah rune

Password boleh mempunyai Unicode code points/runes. Karena itu, panjang password tidak dihitung menggunakan `len`.

Dalam Go:

```go
len("界")
```

menghitung byte UTF-8, bukan jumlah Unicode code points/runes. Policy password memakai:

```go
utf8.RuneCountInString(value)
```

agar `strings.Repeat("界", 15)` dihitung sebagai 15 runes, bukan 45 byte.

Sebelum menghitung rune, kode memastikan string merupakan UTF-8 valid:

```go
utf8.ValidString(value)
```

Policy yang dikunci Step 37 adalah:

```text
minimum 15 Unicode code points/runes
maximum 128 Unicode code points/runes
```

Password tidak di-lowercase, tidak di-trim, dan tidak dinormalisasi. Mengubah secret secara diam-diam dapat membuat nilai yang disimpan berbeda dari yang dimaksud pengguna.

Test juga menunjukkan bahwa 15 spasi lolos dari shape policy. Ini bukan rekomendasi password dan bukan penilaian kekuatan password. Step 37 hanya menyamakan aturan panjang/bentuk dari source aktif; ia belum mempunyai strength meter, breached-password check, ataupun hashing.

## Request ID: panjang byte dan whitespace

Validasi request ID memakai:

```go
len(input.RequestID)
```

Jadi batas 16–64 adalah batas **byte**, sama seperti source production aktif. Berbeda dari password, request ID adalah identifier protocol, bukan teks bebas pengguna.

Selain panjang, kode memeriksa:

```go
strings.TrimSpace(input.RequestID) == input.RequestID
```

Tujuannya bukan mengubah request ID, tetapi menolak nilai yang mempunyai whitespace di awal atau akhir. Dua request ID yang hanya berbeda karena spasi tidak boleh diam-diam dianggap sama.

Pada step ini tidak ada pembatasan alphabet request ID. Kelak client dapat memakai identifier stabil seperti UUID selama panjang dan bentuknya memenuhi contract.

## Delegasi nested validation

Setelah outer field valid, wrapper memanggil:

```go
validateRegisterInput(input.Registration)
```

Wrapper tidak menyalin ulang aturan device ID atau ukuran key. Satu aturan hanya mempunyai satu pemilik:

```text
outer validator -> username, password, request ID
inner validator -> device dan identity key
```

Jika nested registration invalid, sentinel error inner diteruskan tanpa diubah. Karena itu caller kelak masih dapat memakai `errors.Is(err, ErrDeviceIDRequired)`.

## Username input tidak dimutasi dan password result dipertahankan

Fungsi menerima `PasswordRegisterInput` by value:

```go
func validatePasswordRegisterInput(
	input PasswordRegisterInput,
) (PasswordRegisterInput, error)
```

Assignment berikut mengubah salinan lokal:

```go
input.Username = username
```

Fungsi lalu mengembalikan salinan yang sudah ternormalisasi. Variable milik caller tetap menyimpan username semula.

Contoh konseptual:

```text
input.Username  = "Practice_User"
result.Username = "practice_user"
```

Test secara eksplisit membuktikan bahwa `input.Username` milik caller tetap berisi nilai semula dan `result.Password` sama dengan password asli. Caller harus memakai result yang dikembalikan untuk memperoleh username canonical pada proses berikutnya.

Catatan: struct copy di Go bersifat shallow. Field slice pada nested registration masih menunjuk backing array yang sama. Test saat ini tidak membuktikan immutability seluruh graph input. Implementasi validator hanya membaca field lain dan slice public key tanpa memodifikasinya.

## Sentinel error

Step 37 menambahkan:

```text
ErrUsernameInput
ErrPasswordInput
ErrRegistrationRequestIDInvalid
```

Sementara nested validator tetap dapat menghasilkan:

```text
ErrDeviceIDRequired
ErrIdentityIdentifiersInvalid
```

Test memeriksa error memakai `errors.Is`, bukan membandingkan isi `err.Error()`. Ini menjaga caller tetap stabil jika nanti error diberi konteks dengan `%w`.

Error request ID dibuat khusus untuk practice. Source production saat ini mengembalikan error backup untuk request ID invalid, tetapi hal itu menggabungkan dua penyebab yang berbeda. Practice menggunakan sentinel yang menyatakan masalah sebenarnya.

## Isi final `password_registration_test.go`

```go
package identity

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeLoginUsername(
	t *testing.T,
) {
	username, err :=
		NormalizeLoginUsername("Practice_User123")
	if err != nil {
		t.Fatalf(
			"NormalizeLoginUsername() error = %v, want nil",
			err,
		)
	}

	if username != "practice_user123" {
		t.Fatalf(
			"NormalizeLoginUsername() = %q, want %q",
			username,
			"practice_user123",
		)
	}

	invalidUsernames := []string{
		"ab",
		strings.Repeat("a", 33),
		" practice_user",
		"practice-user",
		"pengguna界",
	}

	for _, value := range invalidUsernames {
		_, err := NormalizeLoginUsername(value)

		if !errors.Is(err, ErrUsernameInput) {
			t.Errorf(
				"NormalizeLoginUsername(%q) error = %v, want %v",
				value,
				err,
				ErrUsernameInput,
			)
		}
	}
}

func TestValidateLoginPassword(
	t *testing.T,
) {
	validPasswords := []string{
		strings.Repeat("a", 15),
		strings.Repeat("界", 15),
		strings.Repeat(" ", 15),
		strings.Repeat("a", 128),
	}

	for _, password := range validPasswords {
		if err := validateLoginPassword(
			password,
		); err != nil {
			t.Errorf(
				"validateLoginPassword() error = %v, want nil",
				err,
			)
		}
	}

	invalidPasswords := []string{
		strings.Repeat("a", 14),
		strings.Repeat("界", 129),
		string([]byte{0xff}),
	}

	for _, password := range invalidPasswords {
		err := validateLoginPassword(password)

		if !errors.Is(err, ErrPasswordInput) {
			t.Errorf(
				"validateLoginPassword() error = %v, want %v",
				err,
				ErrPasswordInput,
			)
		}
	}
}

func TestValidatePasswordRegisterInput(
	t *testing.T,
) {
	validInput := func() PasswordRegisterInput {
		return PasswordRegisterInput{
			Username:     "Practice_User",
			Password:     " practice-password ",
			Registration: validRegisterInput(),
			RequestID:    "practice-request-0001",
		}
	}

	tests := []struct {
		name    string
		change  func(*PasswordRegisterInput)
		wantErr error
	}{
		{
			name: "valid input",
		},
		{
			name: "minimum request ID length",
			change: func(
				input *PasswordRegisterInput,
			) {
				input.RequestID =
					strings.Repeat("r", 16)
			},
		},
		{
			name: "maximum request ID length",
			change: func(
				input *PasswordRegisterInput,
			) {
				input.RequestID =
					strings.Repeat("r", 64)
			},
		},
		{
			name: "invalid username",
			change: func(
				input *PasswordRegisterInput,
			) {
				input.Username = "practice-user"
			},
			wantErr: ErrUsernameInput,
		},
		{
			name: "invalid password",
			change: func(
				input *PasswordRegisterInput,
			) {
				input.Password =
					strings.Repeat("a", 14)
			},
			wantErr: ErrPasswordInput,
		},
		{
			name: "request ID is too short",
			change: func(
				input *PasswordRegisterInput,
			) {
				input.RequestID =
					strings.Repeat("r", 15)
			},
			wantErr: ErrRegistrationRequestIDInvalid,
		},
		{
			name: "request ID is too long",
			change: func(
				input *PasswordRegisterInput,
			) {
				input.RequestID =
					strings.Repeat("r", 65)
			},
			wantErr: ErrRegistrationRequestIDInvalid,
		},
		{
			name: "request ID has surrounding whitespace",
			change: func(
				input *PasswordRegisterInput,
			) {
				input.RequestID =
					" " + strings.Repeat("r", 16)
			},
			wantErr: ErrRegistrationRequestIDInvalid,
		},
		{
			name: "nested registration is invalid",
			change: func(
				input *PasswordRegisterInput,
			) {
				input.Registration.DeviceID = " "
			},
			wantErr: ErrDeviceIDRequired,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				input := validInput()

				if test.change != nil {
					test.change(&input)
				}

				originalUsername := input.Username
				originalPassword := input.Password

				result, err :=
					validatePasswordRegisterInput(
						input,
					)

				if test.wantErr != nil {
					if !errors.Is(err, test.wantErr) {
						t.Fatalf(
							"validatePasswordRegisterInput() error = %v, want %v",
							err,
							test.wantErr,
						)
					}

					return
				}

				if err != nil {
					t.Fatalf(
						"validatePasswordRegisterInput() error = %v, want nil",
						err,
					)
				}

				if result.Username !=
					strings.ToLower(originalUsername) {
					t.Errorf(
						"result.Username = %q, want %q",
						result.Username,
						strings.ToLower(
							originalUsername,
						),
					)
				}

				if result.Password != originalPassword {
					t.Errorf(
						"password was modified",
					)
				}

				if input.Username != originalUsername {
					t.Errorf(
						"original input was modified",
					)
				}
			},
		)
	}
}
```

## Yang dibuktikan oleh test

### `TestNormalizeLoginUsername`

Test membuktikan:

- uppercase diterima lalu diubah menjadi lowercase;
- panjang kurang dari 3 ditolak;
- panjang lebih dari 32 ditolak;
- leading space ditolak;
- tanda hubung ditolak;
- karakter non-ASCII ditolak.

### `TestValidateLoginPassword`

Test membuktikan:

- tepat 15 runes diterima;
- password Unicode dihitung berdasarkan rune;
- spasi tidak di-trim atau diubah;
- tepat 128 runes diterima;
- 14 runes ditolak;
- 129 Unicode code points/runes ditolak;
- byte sequence yang bukan UTF-8 valid ditolak.

### `TestValidatePasswordRegisterInput`

Test membuktikan:

- input lengkap diterima;
- request ID tepat 16 byte diterima;
- request ID tepat 64 byte diterima;
- username invalid diteruskan sebagai `ErrUsernameInput`;
- password invalid diteruskan sebagai `ErrPasswordInput`;
- request ID 15 dan 65 byte ditolak;
- request ID dengan surrounding whitespace ditolak;
- error nested registration diteruskan;
- result membawa username lowercase;
- password tidak dinormalisasi;
- original `Username` tetap tidak berubah dan `Password` pada result dipertahankan.

## Perintah format dan test

Dari root module:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server

gofmt -w \
  internal/domain/identity/registration.go \
  internal/domain/identity/registration_test.go \
  internal/domain/identity/password_registration.go \
  internal/domain/identity/password_registration_test.go

go test ./internal/domain/identity \
  -run '^(TestValidateRegisterInput|TestNormalizeLoginUsername|TestValidateLoginPassword|TestValidatePasswordRegisterInput)$' \
  -count=1 \
  -v

go test ./... -count=1
```

## Hasil verifikasi aktual

Focused identity test yang dijalankan setelah implementasi:

```text
=== RUN   TestNormalizeLoginUsername
--- PASS: TestNormalizeLoginUsername (0.00s)
=== RUN   TestValidateLoginPassword
--- PASS: TestValidateLoginPassword (0.00s)
=== RUN   TestValidatePasswordRegisterInput
=== RUN   TestValidatePasswordRegisterInput/valid_input
=== RUN   TestValidatePasswordRegisterInput/minimum_request_ID_length
=== RUN   TestValidatePasswordRegisterInput/maximum_request_ID_length
=== RUN   TestValidatePasswordRegisterInput/invalid_username
=== RUN   TestValidatePasswordRegisterInput/invalid_password
=== RUN   TestValidatePasswordRegisterInput/request_ID_is_too_short
=== RUN   TestValidatePasswordRegisterInput/request_ID_is_too_long
=== RUN   TestValidatePasswordRegisterInput/request_ID_has_surrounding_whitespace
=== RUN   TestValidatePasswordRegisterInput/nested_registration_is_invalid
--- PASS: TestValidatePasswordRegisterInput (0.00s)
    --- PASS: TestValidatePasswordRegisterInput/valid_input (0.00s)
    --- PASS: TestValidatePasswordRegisterInput/minimum_request_ID_length (0.00s)
    --- PASS: TestValidatePasswordRegisterInput/maximum_request_ID_length (0.00s)
    --- PASS: TestValidatePasswordRegisterInput/invalid_username (0.00s)
    --- PASS: TestValidatePasswordRegisterInput/invalid_password (0.00s)
    --- PASS: TestValidatePasswordRegisterInput/request_ID_is_too_short (0.00s)
    --- PASS: TestValidatePasswordRegisterInput/request_ID_is_too_long (0.00s)
    --- PASS: TestValidatePasswordRegisterInput/request_ID_has_surrounding_whitespace (0.00s)
    --- PASS: TestValidatePasswordRegisterInput/nested_registration_is_invalid (0.00s)
=== RUN   TestValidateRegisterInput
=== RUN   TestValidateRegisterInput/valid_input
=== RUN   TestValidateRegisterInput/device_ID_is_empty
=== RUN   TestValidateRegisterInput/X25519_identifier_has_invalid_size
=== RUN   TestValidateRegisterInput/Ed25519_identifier_has_invalid_size
--- PASS: TestValidateRegisterInput (0.00s)
    --- PASS: TestValidateRegisterInput/valid_input (0.00s)
    --- PASS: TestValidateRegisterInput/device_ID_is_empty (0.00s)
    --- PASS: TestValidateRegisterInput/X25519_identifier_has_invalid_size (0.00s)
    --- PASS: TestValidateRegisterInput/Ed25519_identifier_has_invalid_size (0.00s)
PASS
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/domain/identity
```

Full test juga lulus:

```text
ok  github.com/imamsyafii-dev/bbge-practice/srv-server
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/config
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/db
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/app/account
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/domain/account
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/domain/identity
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/router
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/store
?   github.com/imamsyafii-dev/bbge-practice/srv-server/pkg/postgres [no test files]
```

## Common pitfalls

### 1. Menaruh password kembali di inner registration

Ini mencampur credential login dengan metadata identity/device dan mengulangi residue legacy production. Password harus tetap berada di outer wrapper.

### 2. Menggunakan `strings.TrimSpace` pada password

Trim akan mengubah secret pengguna. Password harus diverifikasi persis seperti yang dikirim setelah lolos shape policy.

### 3. Menggunakan `len` untuk jumlah rune password

`len` menghitung byte. Password Unicode dapat ditolak secara tidak konsisten jika policy sebenarnya berbicara tentang Unicode code points/runes.

### 4. Mengizinkan username Unicode tetapi menghitungnya sebagai byte

Contract saat ini sengaja ASCII-only. Jika kebijakan berubah menjadi Unicode username, normalisasi, confusable characters, case folding, dan penghitung panjang harus dirancang ulang bersama-sama.

### 5. Melakukan lowercase pada password

Hanya username yang dinormalisasi. Mengubah kapitalisasi password akan merusak secret.

### 6. Mengabaikan result validator

Validator tidak mengubah `input.Username`; username canonical berada pada `result`. Proses berikutnya harus memakai `result`, bukan `input` lama. Test juga memastikan password pada result tetap sama dengan password asli.

### 7. Menganggap request ID otomatis membuat operasi idempotent

Request ID di Step 37 baru sebuah field yang valid. Idempotency baru nyata jika request disimpan, diberi constraint, dibandingkan saat retry, dan seluruh write dilakukan secara atomic.

### 8. Membandingkan teks error

Gunakan `errors.Is`. Teks error cocok untuk manusia, sedangkan sentinel error menjadi contract program.

### 9. Menyebut public key sudah aman hanya karena panjangnya benar

Validasi 32 byte hanya memeriksa bentuk. Ia belum membuktikan pemilikan private key, signature, atau binding encrypted backup.

## Ringkasan Step 37

Alur validasi yang terbentuk:

```text
PasswordRegisterInput
    -> normalize dan validate username
    -> validate plaintext password shape
    -> validate request ID shape
    -> validate nested RegisterInput
    -> return normalized copy
```

Hasil pentingnya:

- password berpindah dari inner input ke outer wrapper;
- username mempunyai bentuk canonical lowercase;
- policy password memahami Unicode dan tidak mengubah secret;
- request ID siap menjadi bahan idempotency, tetapi retry belum diterapkan;
- inner registration tetap fokus pada device dan identity key;
- contract tetap tanpa email dan tanpa client-supplied user ID.

## Berikutnya: Step 38

Step 38 membuat **password credential dan hasher boundary**.

Fokus berikutnya:

- memisahkan plaintext input dari credential yang dapat dipersist;
- mendefinisikan bentuk password credential tanpa memasukkan password mentah;
- membuat interface consumer kecil yang hanya meminta hashing; verification tetap menjadi kemampuan concrete pada Step 39 dan baru masuk ke interface verifier saat ada consumer yang membutuhkannya;
- memastikan domain service kelak bergantung pada kemampuan yang dibutuhkan, bukan implementasi hashing konkret;
- mengunci dengan test bahwa plaintext tidak masuk ke entity account atau credential persistence.

Implementasi algoritma hashing konkret belum dilakukan di Step 38; itu menjadi fokus Step 39.
