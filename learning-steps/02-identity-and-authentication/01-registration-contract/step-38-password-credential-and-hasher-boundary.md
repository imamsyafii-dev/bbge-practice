# Step 38 — Password credential dan hasher boundary

Step ini memisahkan bentuk password credential dari account dan memperkenalkan boundary kecil untuk proses hashing. Password mentah tetap menjadi input sementara proses registrasi; nilai tersebut tidak menjadi field pada `PasswordCredential`. Hasil hashing dibungkus oleh tipe `EncodedPasswordHash` agar kode domain tidak memperlakukan plaintext dan encoded hash sebagai `string` yang dapat saling dipertukarkan begitu saja.

Implementasi algoritma kriptografi belum dibuat pada step ini. Step 38 hanya mengunci bentuk data dan kontrak yang akan dipenuhi oleh Argon2id hasher pada Step 39.

## Tujuan

Setelah step ini, practice mempunyai:

- `EncodedPasswordHash`, yaitu tipe khusus untuk encoded password hash;
- constructor package-private yang menolak encoded hash kosong atau hanya whitespace;
- accessor `Encoded()` untuk mengeluarkan nilai saat boundary persistence membutuhkannya;
- `PasswordCredential` yang terpisah dari account dan tidak mempunyai field plaintext password;
- `PasswordHasher`, interface consumer kecil yang hanya meminta operasi `Hash`;
- fake hasher untuk membuktikan interface dapat diuji tanpa algoritma kriptografi nyata;
- test propagasi `context.Context`, password yang diterima, jumlah pemanggilan, dan hasil hash;
- compile-time assertion bahwa fake benar-benar memenuhi interface.

Step ini belum membuktikan bahwa sebuah string dibuat dengan algoritma password hashing yang aman. Jaminan tersebut baru datang dari implementasi concrete dan test Step 39.

## Posisi Step 38 dalam alur registrasi

Pada Step 37, password masih berada pada validated registration input:

```text
PasswordRegisterInput.Password
```

Step 38 menyiapkan perubahan bentuk berikut:

```text
plaintext password
    -> PasswordHasher.Hash(ctx, plaintext)
    -> EncodedPasswordHash
    -> PasswordCredential.PasswordHash
    -> persistence boundary mulai Step 43
```

Pemisahan pentingnya adalah:

```text
sebelum hashing                    setelah hashing
-------------------------------    --------------------------------
string plaintext sementara         EncodedPasswordHash
hanya untuk proses registrasi       aman secara tipe untuk credential
tidak boleh dipersist               kelak dikirim ke persistence
```

Kata "aman" pada tabel tersebut berarti lebih aman dari kekeliruan tipe di kode, bukan bukti keamanan kriptografi.

## Perbandingan dengan source production aktif

Source production aktif yang relevan berada pada:

```text
bbge-biz/srv-server/internal/domain/identity/password_crypto.go
bbge-biz/srv-server/internal/domain/identity/password.go
bbge-biz/srv-server/internal/store/account_password_credential.sql.go
```

Production memakai concrete `passwordHasher` yang mempunyai method internal `hash` dan `verify`. Implementasinya menggunakan Argon2id, random salt, perbandingan constant-time, pembatas concurrency untuk membatasi penggunaan memori, dukungan cancellation melalui context, dan metrik operasional. Alur registrasi memanggil `hash`, sedangkan login serta operasi credential lain memanggil `verify`.

Di persistence production, account password credential memuat data yang lebih lengkap, termasuk username, password hash, encrypted identity backup, revision, registration request ID, dan timestamps. Step 38 belum menyalin seluruh bentuk storage tersebut karena practice sedang membangun domain contract secara bertahap.

Perbedaan yang disengaja pada practice:

- encoded hash dibungkus dalam tipe domain, bukan langsung memakai `string` di `PasswordCredential`;
- interface registrasi hanya meminta `Hash`, sesuai kebutuhan consumer saat ini;
- `Verify` belum dimasukkan ke interface karena belum ada consumer login practice;
- encrypted identity backup, revision, timestamps, database model, dan operational concurrency control belum termasuk scope;
- Argon2id concrete hasher baru dibuat dan diuji pada Step 39.

Concrete type pada Step 39 boleh mempunyai kedua method `Hash` dan `Verify`. Namun interface consumer tidak harus menampilkan seluruh method concrete type. Ketika login practice membutuhkan verification, ia dapat menerima interface verifier kecil yang hanya berisi kemampuan yang dipakai oleh login.

## Keputusan desain

### 1. Credential terpisah dari account

Password credential mempunyai lifecycle dan aturan keamanan yang berbeda dari profil atau account identity. Pemisahan ini mencegah account berubah menjadi struct besar yang mencampur identity, metadata, dan rahasia autentikasi.

`PasswordCredential` berisi referensi pemilik serta bentuk data credential yang dimaksudkan untuk dipetakan ke persistence hanya setelah `PasswordHash` berasal dari concrete hasher yang dipercaya. Typed wrapper membantu membedakan bentuk data, tetapi bukan bukti keamanan kriptografi. Struct ini tidak menyimpan plaintext.

### 2. Encoded hash bukan `string` biasa

Jika field didefinisikan seperti ini:

```go
PasswordHash string
```

assignment yang salah masih valid bagi compiler:

```go
credential.PasswordHash = input.Password
```

Step 38 memakai:

```go
PasswordHash EncodedPasswordHash
```

sehingga assignment `string` langsung tidak dapat dikompilasi. Caller harus memperoleh atau membentuk nilai bertipe `EncodedPasswordHash` terlebih dahulu.

### 3. Nilai internal dibuat private

`EncodedPasswordHash` menyimpan:

```go
value string
```

Huruf kecil membuat field tersebut package-private. Kode dari package lain tidak dapat melakukan struct literal dengan mengisi `value`. Jalur konstruksi yang dimaksud adalah `newEncodedPasswordHash`.

Constructor juga dibuat package-private karena pada tahap ini ia merupakan detail domain identity. Concrete hasher Step 39 berada dalam package yang sama dan dapat menggunakannya setelah menghasilkan encoded hash.

### 4. Accessor bernama `Encoded`, bukan `String`

Boundary persistence kelak tetap memerlukan string encoded untuk disimpan. Method berikut menyediakan akses eksplisit:

```go
func (hash EncodedPasswordHash) Encoded() string
```

Nama `Encoded()` menyatakan maksud pemakaian. Tipe sengaja tidak mengimplementasikan `fmt.Stringer` melalui `String()`, sehingga formatting umum tidak otomatis memilih representasi hash sebagai string. Ini mengurangi satu jalur kebocoran tidak sengaja, walaupun tidak menggantikan disiplin logging dan redaction.

### 5. Interface hanya berisi kebutuhan consumer saat ini

Interface Step 38 adalah:

```go
type PasswordHasher interface {
    Hash(
        ctx context.Context,
        plaintext string,
    ) (EncodedPasswordHash, error)
}
```

Consumer registrasi hanya perlu mengubah plaintext menjadi encoded hash. Ia tidak perlu memverifikasi password. Menambahkan `Verify` sekarang akan memaksa fake registrasi mengimplementasikan method yang tidak pernah dipakai.

Ini menerapkan interface segregation dalam bentuk idiomatik Go: interface kecil didefinisikan berdasarkan kebutuhan consumer, bukan sebagai katalog seluruh kemampuan implementation.

Pada Step 39, concrete Argon2id hasher boleh memiliki:

```text
Hash(...)
Verify(...)
```

Tetapi keputusan itu tidak mengubah interface `PasswordHasher`. Interface verifier baru diperlukan saat consumer login benar-benar dibuat. Dengan begitu registration service dan login service dapat meminta dependency minimum masing-masing.

## Struktur file setelah Step 38

```text
srv-server/
└── internal/
    └── domain/
        └── identity/
            ├── password_credential.go
            ├── password_credential_test.go
            ├── password_registration.go
            ├── password_registration_test.go
            ├── registration.go
            └── registration_test.go
```

## Isi final `password_credential.go`

```go
package identity

import (
	"context"
	"errors"
	"strings"
)

var ErrPasswordHashInvalid = errors.New(
	"encoded password hash is invalid",
)

type EncodedPasswordHash struct {
	value string
}

func newEncodedPasswordHash(
	value string,
) (EncodedPasswordHash, error) {
	if strings.TrimSpace(value) == "" {
		return EncodedPasswordHash{},
			ErrPasswordHashInvalid
	}

	return EncodedPasswordHash{
		value: value,
	}, nil
}

func (hash EncodedPasswordHash) Encoded() string {
	return hash.value
}

type PasswordCredential struct {
	UserID                string
	Username              string
	PasswordHash          EncodedPasswordHash
	RegistrationRequestID string
}

type PasswordHasher interface {
	Hash(
		ctx context.Context,
		plaintext string,
	) (EncodedPasswordHash, error)
}
```

## `EncodedPasswordHash` secara rinci

### Sentinel error

Constructor mengembalikan:

```go
ErrPasswordHashInvalid
```

untuk string kosong maupun string yang seluruhnya whitespace. Caller dapat memeriksanya dengan `errors.Is`.

Validasi memakai `strings.TrimSpace` hanya untuk menentukan apakah encoded hash mempunyai isi. Nilai yang valid tidak di-trim sebelum disimpan. Contohnya, bila input bukan kosong tetapi mempunyai spasi tepi, constructor saat ini menyimpan nilai persis yang diberikan. Format concrete Step 39 kelak harus menghasilkan encoding canonical tanpa spasi.

### Constructor mengembalikan zero value pada error

Jalur error mengembalikan:

```go
EncodedPasswordHash{}
```

Nilai tersebut mempunyai `value == ""`. Caller harus memeriksa error sebelum memakai hasil. Pola ini konsisten dengan banyak constructor Go yang mengembalikan zero value bersama error.

### Accessor tidak melakukan transformasi

`Encoded()` hanya mengembalikan nilai yang tersimpan. Ia tidak melakukan hashing, encoding ulang, validation ulang, atau redaction. Karena itu method ini harus dipakai secara sengaja pada boundary yang memang memerlukan encoded representation, terutama persistence.

## Batas perlindungan compile-time

Tipe khusus memberi perlindungan yang berguna:

- plaintext `string` tidak dapat langsung di-assign ke `PasswordHash`;
- signature `PasswordHasher.Hash` menjanjikan hasil bertipe encoded hash;
- test dan call site lebih jelas membedakan plaintext dari nilai yang akan dipersist.

Namun perlindungan tersebut mempunyai batas penting:

- nama tipe tidak membuktikan bahwa isinya benar-benar hash;
- constructor saat ini hanya menolak empty/whitespace-only, bukan memvalidasi format Argon2id;
- kode dalam package `identity` masih dapat membuat `EncodedPasswordHash{value: "..."}` secara langsung;
- zero value `EncodedPasswordHash{}` tetap dapat dibuat;
- implementation `PasswordHasher` yang salah dapat membungkus plaintext dan mengembalikannya;
- `Encoded()` tetap membuka string lengkap, sehingga logging yang tidak aman masih dapat membocorkannya.

Jadi tipe ini adalah guardrail desain dan compile-time, bukan security proof. Step 39 harus membuktikan implementation konkret: algoritma, parameter, salt, perbedaan hasil dengan plaintext, serta verification benar/salah.

## Field `PasswordCredential`

```go
type PasswordCredential struct {
    UserID                string
    Username              string
    PasswordHash          EncodedPasswordHash
    RegistrationRequestID string
}
```

Makna masing-masing field:

| Field | Peran |
|---|---|
| `UserID` | Identity account pemilik credential. Generator server-side baru dibuat pada Step 40. |
| `Username` | Username canonical yang sudah dinormalisasi oleh contract Step 37. |
| `PasswordHash` | Encoded hash bertipe khusus, bukan plaintext password. |
| `RegistrationRequestID` | Identifier request registrasi yang kelak mendukung retry idempotent. |

Struct ini belum mempunyai constructor. Pada Step 38, test membuatnya langsung untuk mengunci bentuk data. Domain service Step 42 kelak menjadi tempat orchestration yang menggabungkan validated input, generated user ID, dan hasil hasher.

Field yang sengaja tidak ada:

- plaintext password;
- email;
- client-supplied user ID;
- encrypted identity backup;
- credential revision;
- created/updated timestamps;
- database tag atau SQL-specific type;
- public/private identity key;
- device atau session state.

Ketiadaan plaintext adalah invariant desain utama. Ketiadaan field lain menjaga entity practice tetap sesuai scope saat ini, bukan klaim bahwa production tidak membutuhkannya.

## `PasswordHasher` dan context

Method `Hash` menerima `context.Context` sebagai parameter pertama:

```go
Hash(ctx context.Context, plaintext string)
```

Password hashing dapat mahal dalam CPU dan memori. Production juga memakai context untuk memeriksa cancellation sebelum dan setelah derivation. Boundary practice meneruskan context sejak awal agar implementation concrete serta caller dapat mempertahankan cancellation dan deadline.

Interface tidak menjanjikan bahwa semua implementation dapat menghentikan primitive kriptografi tepat di tengah eksekusi. Ia tetap menyediakan channel standar untuk cancellation, deadline, dan metadata request yang memang perlu diteruskan.

Password diterima persis seperti input. Boundary tidak melakukan `TrimSpace`, lowercase, ataupun normalisasi. Password dengan spasi yang sudah lolos policy Step 37 harus di-hash apa adanya.

## Fake hasher dan compile-time assertion

Test memakai fake stateful:

```go
type fakePasswordHasher struct {
    result           EncodedPasswordHash
    err              error
    calls            int
    receivedContext  context.Context
    receivedPassword string
}
```

Fake tersebut tidak menghitung hash. Ia mencatat input lalu mengembalikan hasil/error yang sudah dikonfigurasi. Ini tepat untuk test consumer karena test dapat memeriksa interaksi tanpa menanggung biaya atau randomness algoritma konkret.

Assertion berikut dijalankan oleh compiler:

```go
var _ PasswordHasher = (*fakePasswordHasher)(nil)
```

Maknanya:

- target interface adalah `PasswordHasher`;
- tipe yang diperiksa adalah pointer `*fakePasswordHasher`;
- `nil` tidak membuat instance dan tidak memanggil method;
- build gagal bila method set pointer tersebut tidak memenuhi interface.

Pointer dipakai karena method `Hash` mempunyai pointer receiver agar dapat menaikkan `calls` dan menyimpan argumen yang diterima.

## Apa yang dibuktikan test boundary

`TestPasswordHasherBoundary` membuat context dengan typed key dan value khusus. Setelah memanggil interface, test membuktikan:

1. hasil yang dikembalikan sama dengan encoded hash yang dikonfigurasi;
2. `Hash` dipanggil tepat satu kali;
3. plaintext diteruskan persis, termasuk spasi di awal dan akhir;
4. context yang diterima fake masih membawa value yang sama.

Test context value bukan anjuran untuk menyimpan data sembarang di context. Ia dipakai sebagai penanda sederhana bahwa context tidak diganti dengan `context.Background()` di boundary.

## Isi final `password_credential_test.go`

```go
package identity

import (
	"context"
	"errors"
	"testing"
)

func TestNewEncodedPasswordHash(
	t *testing.T,
) {
	tests := []struct {
		name    string
		value   string
		want    string
		wantErr error
	}{
		{
			name:  "valid encoded hash",
			value: "practice-encoded-password-hash",
			want:  "practice-encoded-password-hash",
		},
		{
			name:    "empty encoded hash",
			value:   "",
			wantErr: ErrPasswordHashInvalid,
		},
		{
			name:    "spaces only",
			value:   "   ",
			wantErr: ErrPasswordHashInvalid,
		},
		{
			name:    "whitespace only",
			value:   "\t\n",
			wantErr: ErrPasswordHashInvalid,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				hash, err :=
					newEncodedPasswordHash(
						test.value,
					)

				if test.wantErr != nil {
					if !errors.Is(err, test.wantErr) {
						t.Fatalf(
							"newEncodedPasswordHash() error = %v, want %v",
							err,
							test.wantErr,
						)
					}

					return
				}

				if err != nil {
					t.Fatalf(
						"newEncodedPasswordHash() error = %v, want nil",
						err,
					)
				}

				if hash.Encoded() != test.want {
					t.Errorf(
						"Encoded() = %q, want %q",
						hash.Encoded(),
						test.want,
					)
				}
			},
		)
	}
}

func TestPasswordCredentialUsesEncodedHash(
	t *testing.T,
) {
	hash, err := newEncodedPasswordHash(
		"practice-encoded-password-hash",
	)
	if err != nil {
		t.Fatalf(
			"newEncodedPasswordHash() error = %v",
			err,
		)
	}

	credential := PasswordCredential{
		UserID:                "practice-user-001",
		Username:              "practice_user",
		PasswordHash:          hash,
		RegistrationRequestID: "practice-request-0001",
	}

	if credential.UserID != "practice-user-001" {
		t.Errorf(
			"UserID = %q, want %q",
			credential.UserID,
			"practice-user-001",
		)
	}

	if credential.Username != "practice_user" {
		t.Errorf(
			"Username = %q, want %q",
			credential.Username,
			"practice_user",
		)
	}

	if credential.PasswordHash.Encoded() !=
		"practice-encoded-password-hash" {
		t.Errorf(
			"PasswordHash = %q, want %q",
			credential.PasswordHash.Encoded(),
			"practice-encoded-password-hash",
		)
	}

	if credential.RegistrationRequestID !=
		"practice-request-0001" {
		t.Errorf(
			"RegistrationRequestID = %q, want %q",
			credential.RegistrationRequestID,
			"practice-request-0001",
		)
	}
}

type fakePasswordHasher struct {
	result           EncodedPasswordHash
	err              error
	calls            int
	receivedContext  context.Context
	receivedPassword string
}

func (
	hasher *fakePasswordHasher,
) Hash(
	ctx context.Context,
	plaintext string,
) (EncodedPasswordHash, error) {
	hasher.calls++
	hasher.receivedContext = ctx
	hasher.receivedPassword = plaintext

	return hasher.result, hasher.err
}

var _ PasswordHasher = (*fakePasswordHasher)(nil)

type passwordHasherContextKey struct{}

func TestPasswordHasherBoundary(
	t *testing.T,
) {
	expectedHash, err := newEncodedPasswordHash(
		"practice-fake-encoded-hash",
	)
	if err != nil {
		t.Fatalf(
			"newEncodedPasswordHash() error = %v",
			err,
		)
	}

	ctx := context.WithValue(
		context.Background(),
		passwordHasherContextKey{},
		"practice-context",
	)
	plaintext := " practice-password "

	fake := &fakePasswordHasher{
		result: expectedHash,
	}

	var hasher PasswordHasher = fake

	result, err := hasher.Hash(
		ctx,
		plaintext,
	)
	if err != nil {
		t.Fatalf(
			"Hash() error = %v, want nil",
			err,
		)
	}

	if result.Encoded() != expectedHash.Encoded() {
		t.Errorf(
			"Hash() = %q, want %q",
			result.Encoded(),
			expectedHash.Encoded(),
		)
	}

	if fake.calls != 1 {
		t.Errorf(
			"Hash() calls = %d, want 1",
			fake.calls,
		)
	}

	if fake.receivedPassword != plaintext {
		t.Errorf(
			"received password = %q, want %q",
			fake.receivedPassword,
			plaintext,
		)
	}

	if fake.receivedContext.Value(
		passwordHasherContextKey{},
	) != "practice-context" {
		t.Errorf(
			"context value was not forwarded",
		)
	}
}
```

## Cakupan test

### `TestNewEncodedPasswordHash`

Test table-driven ini mengunci:

- nilai non-empty dipertahankan dan tersedia melalui `Encoded()`;
- string kosong ditolak;
- spasi saja ditolak;
- whitespace tab/newline saja ditolak;
- sentinel diperiksa dengan `errors.Is`.

Test belum memvalidasi grammar encoded Argon2id karena implementation dan format konkret baru menjadi tanggung jawab Step 39.

### `TestPasswordCredentialUsesEncodedHash`

Test ini membentuk credential dengan encoded hash yang valid lalu memeriksa semua field. Nilai tambah utamanya tetap terjadi pada compile time: field `PasswordHash` hanya menerima `EncodedPasswordHash`, bukan `string` biasa.

### `TestPasswordHasherBoundary`

Test ini membuktikan kontrak consumer tanpa menguji algoritma konkret. Fake mengembalikan hasil deterministik sehingga test cepat dan tidak flakey.

Field `err` sudah tersedia pada fake untuk test consumer di step berikutnya, walaupun test Step 38 hanya menjalankan success path.

## Format dan perintah verifikasi

Dari `practice/srv-server`, format file Go:

```bash
gofmt -w \
  internal/domain/identity/password_credential.go \
  internal/domain/identity/password_credential_test.go
```

Jalankan focused test:

```bash
go test ./internal/domain/identity \
  -run '^(TestNewEncodedPasswordHash|TestPasswordCredentialUsesEncodedHash|TestPasswordHasherBoundary)$' \
  -count=1 \
  -v
```

Jalankan seluruh suite:

```bash
go test ./... -count=1
```

Jalankan race test package identity:

```bash
go test -race ./internal/domain/identity -count=1
```

Pada mesin verifikasi, shell sempat membawa `GOROOT` versi lama. Agar hasil memakai binary Homebrew dan cache yang cocok, verifikasi aktual dijalankan dengan bentuk eksplisit berikut:

```bash
env -u GOROOT \
  GOCACHE=/private/tmp/bbge-practice-go-cache \
  /opt/homebrew/bin/go test ./internal/domain/identity \
  -run '^(TestNewEncodedPasswordHash|TestPasswordCredentialUsesEncodedHash|TestPasswordHasherBoundary)$' \
  -count=1 \
  -v

env -u GOROOT \
  GOCACHE=/private/tmp/bbge-practice-go-cache \
  /opt/homebrew/bin/go test ./... -count=1

env -u GOROOT \
  GOCACHE=/private/tmp/bbge-practice-go-race-cache \
  /opt/homebrew/bin/go test -race \
  ./internal/domain/identity \
  -count=1
```

Path dan environment override tersebut khusus untuk mesin verifikasi. Jika `go version` dan `GOROOT` shell sudah konsisten, command sederhana sebelumnya tetap merupakan command normal untuk latihan.

## Hasil verifikasi aktual

Focused test lulus:

```text
=== RUN   TestNewEncodedPasswordHash
=== RUN   TestNewEncodedPasswordHash/valid_encoded_hash
=== RUN   TestNewEncodedPasswordHash/empty_encoded_hash
=== RUN   TestNewEncodedPasswordHash/spaces_only
=== RUN   TestNewEncodedPasswordHash/whitespace_only
--- PASS: TestNewEncodedPasswordHash (0.00s)
    --- PASS: TestNewEncodedPasswordHash/valid_encoded_hash (0.00s)
    --- PASS: TestNewEncodedPasswordHash/empty_encoded_hash (0.00s)
    --- PASS: TestNewEncodedPasswordHash/spaces_only (0.00s)
    --- PASS: TestNewEncodedPasswordHash/whitespace_only (0.00s)
=== RUN   TestPasswordCredentialUsesEncodedHash
--- PASS: TestPasswordCredentialUsesEncodedHash (0.00s)
=== RUN   TestPasswordHasherBoundary
--- PASS: TestPasswordHasherBoundary (0.00s)
PASS
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/domain/identity  0.011s
```

Seluruh suite lulus:

```text
ok  github.com/imamsyafii-dev/bbge-practice/srv-server                           0.017s
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/config                    0.009s
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/db                        0.023s
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/app/account      0.021s
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/domain/account   0.018s
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/domain/identity  0.024s
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/router           0.029s
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/store            0.012s
?   github.com/imamsyafii-dev/bbge-practice/srv-server/pkg/postgres              [no test files]
```

Race test identity lulus:

```text
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/domain/identity  1.013s
```

Durasi dapat berbeda pada mesin atau run berikutnya. Status `PASS` dan package yang diuji adalah bagian hasil yang penting.

## Kesalahan umum

### 1. Menaruh plaintext pada `PasswordCredential`

Jangan menambahkan:

```go
Password string
```

ke credential. Plaintext hanya diperlukan sementara untuk hashing dan tidak boleh ikut ke entity yang akan dipersist.

### 2. Menggunakan `string` untuk `PasswordHash`

Ini menghilangkan pemisahan compile-time dan membuat assignment plaintext menjadi terlalu mudah. Pertahankan tipe `EncodedPasswordHash` sampai boundary persistence benar-benar memerlukan encoded string.

### 3. Menganggap constructor membuktikan keamanan kriptografi

Constructor hanya menolak nilai kosong/whitespace-only. Ia belum memvalidasi format, algoritma, parameter, salt, atau asal nilai.

### 4. Menambahkan `Verify` ke `PasswordHasher` terlalu dini

Registrasi hanya membutuhkan `Hash`. Interface yang lebih lebar membuat fake dan consumer bergantung pada kemampuan yang tidak digunakan. Buat interface verifier saat login benar-benar memerlukannya.

### 5. Memotong whitespace password

Test sengaja mengirim:

```text
 practice-password 
```

dan memastikan fake menerima nilai persis. Jangan memakai `strings.TrimSpace` pada plaintext password.

### 6. Mengganti context dengan `context.Background()`

Hal itu memutus cancellation, deadline, dan metadata request. Teruskan context caller ke dependency.

### 7. Menambahkan `String()` untuk kenyamanan logging

Encoded hash bukan data yang perlu muncul di log biasa. Gunakan accessor eksplisit hanya pada boundary yang membutuhkannya.

### 8. Menganggap private field membuat nilai mustahil dipalsukan

Privacy field hanya membatasi package lain. Kode dalam package yang sama, zero value, atau implementation interface yang salah tetap dapat menghasilkan nilai tidak valid. Test concrete implementation masih wajib.

## Scope dan batas Step 38

Yang dikerjakan:

- bentuk typed encoded password hash;
- validasi minimum encoded value non-empty;
- credential terpisah dari account;
- interface hashing untuk consumer registrasi;
- fake dan compile-time interface assertion;
- unit test data shape serta forwarding boundary.

Yang belum dikerjakan:

- implementasi Argon2id;
- random salt dan encoded Argon2id format;
- verification password benar/salah;
- constant-time comparison;
- concurrency/memory bound hashing;
- persistence, database migration, dan SQLC;
- generated user ID;
- registration domain service;
- idempotent retry terhadap data tersimpan;
- encrypted identity backup;
- proof-of-possession atau signature verification;
- HTTP, token, dan session issuance.

## Ringkasan

Step 38 mengunci boundary antara plaintext password dan credential yang dapat dipersist:

- plaintext tetap berupa input sementara;
- hasil hashing mempunyai tipe `EncodedPasswordHash`;
- internal value private dan dibuka secara eksplisit melalui `Encoded()`;
- `PasswordCredential` tidak mempunyai field plaintext;
- registrasi bergantung pada interface `PasswordHasher` yang hanya berisi `Hash`;
- verification tidak dipaksakan ke interface sebelum ada consumer;
- fake membuktikan result, call count, plaintext preservation, dan context forwarding;
- tipe khusus membantu compiler, tetapi belum membuktikan keamanan algoritma.

## Berikutnya: Step 39

Step 39 membuat **implementasi dan test password hashing**.

Fokus berikutnya:

- membuat concrete Argon2id hasher yang memenuhi `PasswordHasher`;
- menghasilkan salt acak dan encoded hash yang memuat parameter yang diperlukan;
- memastikan hash tidak sama dengan plaintext dan dua hash password sama tidak identik karena salt;
- memverifikasi password benar dan menolak password salah atau encoded hash malformed;
- mempertahankan password persis tanpa trimming;
- menguji cancellation serta batas operasional yang dipilih untuk practice;
- membiarkan `Verify` sebagai kemampuan concrete, lalu memperkenalkan interface verifier kecil hanya ketika consumer login membutuhkannya.
