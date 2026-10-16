# Step 40 — Server-generated user identity

Step ini membuat concrete generator untuk `user_id` yang sepenuhnya dikendalikan server. Generator membaca 16 byte dari `crypto/rand.Reader`, lalu mengubahnya menjadi 32 karakter lowercase hexadecimal. Registration input tetap tidak mempunyai field `UserID`, sehingga client tidak mempunyai slot domain untuk memilih identity account.

Implementasi practice sengaja sederhana: hasilnya adalah opaque random identifier, bukan UUIDv8 production. Step ini juga belum mempunyai interface `UserIDGenerator`, fake generator untuk registration service, atau wiring ke service. Interface consumer-owned dan behavioral service test baru tepat dibuat pada Step 42 ketika registration service benar-benar menjadi consumer.

## Tujuan dan scope

Setelah Step 40, practice mempunyai:

- concrete `RandomUserIDGenerator`;
- constructor yang memasang `crypto/rand.Reader`;
- method `Generate(context.Context) (string, error)`;
- output 32 karakter lowercase hexadecimal dari 128 bit random;
- private `io.Reader` sebagai test seam;
- `io.ReadFull` agar seluruh buffer 16 byte harus terisi walaupun reader melakukan partial read;
- pemeriksaan context sebelum dan sesudah random read;
- sentinel error untuk nil receiver atau reader yang tidak dikonfigurasi;
- wrapped reader error yang tetap dapat diperiksa dengan `errors.Is`;
- test deterministik untuk output, fresh randomness, error, configuration, dan cancellation;
- reflection test yang memastikan dua registration input tidak mempunyai field bernama `UserID`;
- hanya standard library, tanpa dependency module baru.

Step ini tidak membuat registration service, persistence, database constraint practice, retry collision, HTTP contract, ataupun consumer interface. Ia menyediakan concrete primitive yang kelak dikonsumsi Step 42.

## Hubungan dengan Step 36–42

Urutan desainnya adalah:

```text
Step 36  RegisterInput tanpa user_id
    ↓
Step 37  PasswordRegisterInput membungkus RegisterInput, tetap tanpa user_id
    ↓
Step 38  PasswordCredential + Hash-only PasswordHasher boundary
    ↓
Step 39  Concrete Argon2IDPasswordHasher
    ↓
Step 40  Concrete RandomUserIDGenerator
    ↓
Step 41  Device-as-session entity dan ownership
    ↓
Step 42  Registration domain service
         + consumer-owned UserIDGenerator interface
         + fake generator untuk behavioral service test
         + orkestrasi hasher, generated user ID, credential, dan device
```

Step 36 dan Step 37 sudah mencegah client mengirim `UserID` melalui bentuk input domain. Step 40 mempertahankan invariant struktural itu dan menyediakan generator concrete. Namun baru Step 42 yang dapat membuktikan secara behavioral bahwa service benar-benar memanggil generator dan meneruskan hasilnya ke account yang dibuat.

Step 41 lebih dahulu mendefinisikan device-as-session karena registration service Step 42 harus membentuk ownership account dan device secara bersamaan. Persistence serta unique constraint practice baru dibangun mulai Step 43.

## Bentuk alur Step 40

```text
NewRandomUserIDGenerator()
    -> random = crypto/rand.Reader

Generate(ctx)
    -> pastikan receiver dan random reader tersedia
    -> periksa ctx.Err()
    -> io.ReadFull(reader, buffer 16 byte)
    -> periksa ctx.Err() lagi
    -> hex.EncodeToString(buffer)
    -> string 32 karakter lowercase hexadecimal
```

Tidak ada input client pada method `Generate`. Caller hanya menyediakan context. Random bytes dan hasil ID ditentukan di sisi server.

## Bukti production yang dibandingkan

Source production aktif yang relevan adalah:

```text
bbge-biz/srv-server/internal/domain/identity/service.go
bbge-biz/srv-server/pkg/redis/user_id.go
bbge-biz/srv-server/pkg/helper/tools.go
bbge-biz/proto/proto/bbge/identity/identity.proto
bbge-biz/proto/proto/bbge/identity/password.proto
bbge-biz/srv-server/db/migrations/0002_create_account.sql
```

### 1. Production mengalokasikan internal UID dengan Redis `INCR`

Di `internal/domain/identity/service.go`, `registerAccount` memanggil `generateInternalUID()`. Helper tersebut memanggil:

```go
rediskit.GetNextUserID(
    context.Background(),
    helper.MaxBit37UserID,
)
```

`pkg/redis/user_id.go` memakai atomic Redis operation:

```go
universalClient.Incr(ctx, GlobalUserIDKey)
```

Key-nya adalah `global:user:id`, diinisialisasi ke `1000000` bila belum ada, dan hasil increment diperiksa terhadap batas 37 bit `(1 << 37) - 1`.

Alokasi Redis tersebut terjadi saat closure transaction registration berjalan, tetapi bukan bagian dari rollback database. Nomor yang sudah di-increment tidak otomatis kembali bila transaction database kemudian gagal. `generateInternalUID` production juga memakai `context.Background()`, bukan request context; lesson tidak mengklaim allocation itu dibatalkan bersama request.

### 2. Production membentuk custom UUIDv8

Setelah memperoleh `internalUID`, `registerAccount` memanggil:

```go
userID := helper.GenerateInternalUserIDUUIDv8(
    internalUID,
)
```

Production kemudian menyimpan `userID` dan `internalUID` sebagai dua field account yang berkaitan. `pkg/helper/tools.go` menyusun custom 128-bit UUIDv8 dari:

- 48 bit Unix timestamp dalam millisecond;
- nibble version `8`;
- 8 random bit;
- 37 bit internal user ID;
- UUID variant bits;
- 29 random business bits.

Hasil byte diubah melalui `uuid.UUID(id).String()`, lalu seluruh hyphen dibuang. Output akhirnya 32 karakter lowercase hexadecimal. Production juga mempunyai extractor yang dapat membaca kembali internal UID dan field lain dari layout tersebut.

Ini bukan sekadar UUID random. Format production membawa struktur dan mempunyai coupling dengan Redis internal UID serta helper extraction.

### 3. Request production tidak membawa `user_id`

Canonical `RegisterRequest` di `proto/proto/bbge/identity/identity.proto` mempunyai field registration seperti device dan identity identifiers, tetapi tidak mempunyai `user_id`. `PasswordRegisterRequest` di `password.proto` membawa username, password, nested registration, backup, dan request ID; wrapper tersebut juga tidak mempunyai `user_id`.

Jadi production membuat identity server-side. Response boleh mengembalikan account yang mempunyai `user_id`, tetapi itu berbeda dari menerima pilihan ID dalam registration request.

### 4. Database production mengunci uniqueness

Migration `db/migrations/0002_create_account.sql` mendefinisikan:

```sql
user_id VARCHAR(40) NOT NULL,
internal_uid BIGINT NOT NULL DEFAULT 0,
...
CONSTRAINT account_user_id_unique UNIQUE (user_id)
```

Migration itu juga membuat partial unique index untuk `internal_uid` ketika nilainya bukan nol:

```sql
CREATE UNIQUE INDEX IF NOT EXISTS account_internal_uid_unique
    ON account (internal_uid)
    WHERE internal_uid != 0;
```

Schema production tidak mempunyai `CHECK` yang memaksa `user_id` selalu 32 lowercase hex atau UUIDv8. Bentuk tersebut merupakan aturan generator aplikasi, sedangkan database mengunci panjang maksimum, non-null, dan uniqueness.

## Perbedaan practice dan production

| Aspek | Production aktif | Practice Step 40 |
|---|---|---|
| Sumber identity | Redis internal UID + random fields + timestamp | 16 byte random dari `crypto/rand.Reader` |
| Format | Custom UUIDv8 tanpa hyphen | Opaque lowercase hexadecimal |
| Panjang output | 32 karakter | 32 karakter |
| Informasi embedded | Timestamp, version, variant, internal UID, business bits | Tidak ada field production yang sengaja di-embed |
| Internal UID | Dialokasikan dengan Redis `INCR` | Tidak ada |
| Extraction | Helper production dapat mengekstrak field UUIDv8 | Tidak didukung; hasil yang lolos secara kebetulan tidak bermakna |
| Database uniqueness | `account.user_id` unique | Belum ada persistence pada Step 40 |
| Dependency | `github.com/google/uuid` dan Redis infrastructure | Standard library saja |

Kesamaan panjang dan alfabet tidak membuat kedua format kompatibel. Random ID practice tidak sengaja menyetel atau memvalidasi UUIDv8 version bits, variant bits, maupun layout field production, sehingga tidak ada jaminan parser production akan menerimanya. Karena bytes practice acak, kombinasi version `8` dan variant yang diperiksa production dapat cocok secara kebetulan dengan peluang sekitar `1/16 × 1/4 = 1/64`. Bila pemeriksaan itu lolos dan extraction berlanjut, timestamp, internal UID, serta business fields yang terbaca hanyalah interpretasi kebetulan atas random bits dan tidak mempunyai makna production. Karena itu ID practice tetap bukan format yang production-compatible dan tidak boleh dipakai untuk menguji semantics extractor production.

Penyederhanaan ini disengaja agar Step 40 berfokus pada ownership: server membuat opaque identity, client tidak memilihnya. Porting data atau interoperabilitas dengan production berada di luar scope.

## Mengapa 16 byte dan 32 hexadecimal

Konstanta practice adalah:

```go
const userIDRandomByteLength = 16
```

Enam belas byte sama dengan 128 bit. Setiap byte menghasilkan dua digit hexadecimal:

```text
16 byte × 2 hex character per byte = 32 character
```

`encoding/hex.EncodeToString` memakai alfabet lowercase `0-9a-f`, sehingga output deterministic untuk byte input yang sama. Contoh fixture:

```text
00 01 02 03 04 05 06 07 08 09 0a 0b 0c 0d 0e 0f
    ↓
000102030405060708090a0b0c0d0e0f
```

Nilai tersebut diperlakukan opaque. Caller tidak boleh mencoba membaca timestamp, shard, sequence, environment, atau ownership dari karakter ID.

## Mengapa tidak memakai prefix `practice_`

Generator sengaja tidak menambahkan prefix environment:

- identity tetap opaque dan tidak membawa deployment label;
- format practice tetap 32 karakter dan muat dalam batas production `VARCHAR(40)`;
- string `practice_` mempunyai 9 karakter, sehingga `practice_` + 32 hex menjadi 41 karakter dan melewati batas tersebut;
- memendekkan random payload hanya demi prefix akan mengurangi ruang random tanpa kebutuhan domain;
- prefix bukan pengganti database unique constraint atau environment isolation.

Jika environment perlu dibedakan, configuration, database boundary, dan deployment ownership adalah tempat yang lebih tepat daripada mengubah bentuk identity account secara ad hoc.

## Mengapa cukup standard library

Step 40 hanya membutuhkan:

```go
context
crypto/rand
encoding/hex
errors
fmt
io
```

Practice tidak sedang membuat UUID RFC tertentu. Karena itu library UUID eksternal justru dapat menyiratkan semantics version/variant yang tidak dimiliki output ini. `crypto/rand.Reader`, `io.ReadFull`, dan `hex.EncodeToString` sudah memenuhi contract opaque 128-bit random ID.

Tidak ada perubahan `go.mod` atau `go.sum` pada Step 40.

## `io.Reader` sebagai boundary kecil

Field concrete generator adalah:

```go
type RandomUserIDGenerator struct {
    random io.Reader
}
```

`io.Reader` adalah interface standard library dengan method konseptual:

```go
Read(buffer []byte) (numberOfBytes int, err error)
```

Production constructor memasang `rand.Reader`. Test dalam package `identity` yang sama dapat mengganti private field `random` dengan `bytes.Reader` atau reader buatan. Dengan begitu test tidak bergantung pada output random nyata dan dapat memicu error secara deterministik.

Field tetap private. Package lain tidak dapat mengisi `random` melalui struct literal. Public path normal adalah `NewRandomUserIDGenerator`, sehingga caller production memperoleh `crypto/rand.Reader` dan tidak diberi API untuk memasang pseudo-random reader yang lemah.

## `userIDReaderFunc` dan adapter `Read`

Test mendefinisikan named function type:

```go
type userIDReaderFunc func(
    buffer []byte,
) (int, error)
```

Function type tersebut belum otomatis memenuhi `io.Reader`, karena interface membutuhkan method bernama `Read`. Adapter berikut menambahkan method itu:

```go
func (
    function userIDReaderFunc,
) Read(
    buffer []byte,
) (int, error) {
    return function(buffer)
}
```

Sekarang test dapat mengubah inline function menjadi `userIDReaderFunc` dan memasangnya sebagai `io.Reader`. Adapter dipakai untuk:

- mengembalikan sentinel reader error;
- mencatat apakah reader sempat dipanggil;
- mengisi buffer lalu membatalkan context selama read.

Type ini hanya ada di `_test.go`; ia bukan API production.

## Mengapa memakai `io.ReadFull`

Contract `io.Reader.Read` mengizinkan partial read. Satu pemanggilan `Read` boleh mengisi kurang dari panjang buffer tanpa berarti generator sudah mempunyai entropy yang cukup.

Implementation memakai:

```go
io.ReadFull(generator.random, randomBytes[:])
```

`io.ReadFull` terus membaca sampai tepat 16 byte terkumpul atau reader mengembalikan error. Jika stream berhenti sebelum penuh, hasil seperti `io.ErrUnexpectedEOF` menjadi error dan ID kosong dikembalikan. Generator tidak meng-encode buffer setengah terisi sebagai ID.

Test saat ini tidak mempunyai subtest khusus reader yang sukses melalui beberapa partial read. Jaminan pengulangan read berasal dari contract `io.ReadFull`; test deterministik mengunci output full read dan error forwarding.

## Constructor dan private reader test seam

Constructor production sangat kecil:

```go
func NewRandomUserIDGenerator() *RandomUserIDGenerator {
    return &RandomUserIDGenerator{
        random: rand.Reader,
    }
}
```

Constructor tidak menerima reader dari caller. Ini menjaga API production sederhana dan mencegah randomness lemah disuntikkan secara tidak sengaja. Same-package test sengaja mempunyai akses ke private field untuk membentuk generator deterministik:

```go
generator := &RandomUserIDGenerator{
    random: bytes.NewReader(randomInput),
}
```

Ini adalah test seam pada infrastructure primitive, bukan `UserIDGenerator` domain interface. Interface yang akan dipakai registration service belum dibuat pada Step 40.

## Context sebelum dan sesudah random read

`Generate` memeriksa `ctx.Err()` sebelum membaca. Context yang sudah canceled menghasilkan `context.Canceled` tanpa memanggil reader.

Setelah `io.ReadFull` selesai, `ctx.Err()` diperiksa lagi. Bila context dibatalkan saat synchronous reader sedang bekerja, bytes tidak diubah menjadi ID dan method mengembalikan cancellation.

Batas pentingnya: `io.Reader.Read` tidak menerima context. Jika reader benar-benar block, pemeriksaan ini tidak dapat menghentikannya di tengah operasi. Cancellation baru diamati setelah `io.ReadFull` kembali. `crypto/rand.Reader` biasanya sesuai untuk operasi kecil ini, tetapi contract method tetap tidak menjanjikan preemption atas arbitrary reader.

Random buffer adalah local fixed array. Bila post-read context check gagal, function mengembalikan string kosong. Step 40 tidak menambahkan explicit `clear` karena generated random ID bukan secret credential; output yang sukses memang akan dipakai sebagai public account identifier.

## Error configuration dan wrapping

Nil receiver dan generator tanpa reader mengembalikan:

```go
ErrUserIDGeneratorNotConfigured
```

Guard dilakukan sebelum dereference, sehingga method pada nil pointer tidak panic.

Reader error dibungkus:

```go
fmt.Errorf("generate user ID: %w", err)
```

Prefix memberi context operasi, sedangkan `%w` mempertahankan error chain. Caller atau test dapat memakai:

```go
errors.Is(err, errUserIDRandomRead)
```

Step 40 tidak mengubah reader failure menjadi sentinel generik karena root cause masih berguna untuk diagnosis. Error dan empty ID harus selalu diperiksa bersama; caller tidak boleh memakai hasil bila error non-nil.

## Randomness, collision, dan database uniqueness

Dua successful call membaca 16 byte baru masing-masing. Ruang 128-bit membuat collision acak sangat kecil, tetapi randomness bukan bukti uniqueness matematis. Test fresh randomness hanya menunjukkan dua fixture berurutan menghasilkan dua ID berbeda; ia tidak membuktikan semua output masa depan unik.

Boundary persistence tetap harus mempunyai unique constraint. Production sudah mempunyai `UNIQUE (user_id)`. Practice baru menambahkan schema dan aturan conflict pada Step 43–49. Saat itu collision atau duplicate insert harus diperlakukan sebagai conflict persistence yang eksplisit; generator Step 40 sendiri tidak melakukan query database dan tidak mempunyai retry loop.

Jangan melakukan pola check-then-insert tanpa constraint karena dua request concurrent tetap dapat lolos pemeriksaan yang sama. Database uniqueness adalah final arbiter pada persistence boundary.

## Structural ownership test dan batasnya

`TestRegistrationInputsDoNotExposeUserID` menggunakan reflection:

```go
inputType.FieldByName("UserID")
```

Test memeriksa `RegisterInput` dan `PasswordRegisterInput`. Jika salah satu kelak menambahkan exported field exact bernama `UserID`, test gagal. Ini mengunci keputusan bahwa registration input domain saat ini tidak memberikan slot langsung bagi client-controlled user ID.

Batas test tersebut:

- hanya memeriksa field Go bernama exact `UserID`;
- tidak mendeteksi nama alternatif seperti `ChosenIdentity`;
- tidak menguji JSON/protobuf transport field atau custom decoder;
- tidak membuktikan handler menolak payload unknown field;
- tidak membuktikan service memanggil generator;
- tidak membuktikan hasil generator diteruskan ke account/persistence;
- tidak menjalankan concurrent registration atau duplicate collision.

Behavioral proof baru ditambahkan pada Step 42: registration service akan menerima consumer-owned generator interface dan fake yang merekam pemanggilan serta mengembalikan ID tetap. HTTP contract tanpa `user_id` baru diuji pada Step 52–53.

## Belum ada interface `UserIDGenerator`

Step 40 sengaja hanya membuat concrete type:

```go
*RandomUserIDGenerator
```

Belum ada deklarasi:

```go
type UserIDGenerator interface { ... }
```

Ini konsisten dengan prinsip consumer-owned interface di Go. Registration service adalah consumer yang memerlukan kemampuan `Generate`; karena service baru dibuat pada Step 42, interface kecil dan fake behavioral juga dibuat di sana. Step 40 tidak menebak contract consumer lebih awal dan tidak mengklaim `io.Reader` sebagai pengganti interface domain tersebut.

Signature concrete sudah membentuk kemampuan yang kemungkinan dikonsumsi nanti:

```go
Generate(context.Context) (string, error)
```

Namun dokumentasi ini tidak menjanjikan nama atau lokasi interface sebelum Step 42 benar-benar menguncinya lewat consumer dan test.

## Isi final `user_id_generator.go`

Listing berikut harus sama persis dengan source setelah `gofmt`.

```go
package identity

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

const userIDRandomByteLength = 16

var ErrUserIDGeneratorNotConfigured = errors.New(
	"user ID generator is not configured",
)

type RandomUserIDGenerator struct {
	random io.Reader
}

func NewRandomUserIDGenerator() *RandomUserIDGenerator {
	return &RandomUserIDGenerator{
		random: rand.Reader,
	}
}

func (
	generator *RandomUserIDGenerator,
) Generate(
	ctx context.Context,
) (string, error) {
	if generator == nil ||
		generator.random == nil {
		return "",
			ErrUserIDGeneratorNotConfigured
	}

	if err := ctx.Err(); err != nil {
		return "", err
	}

	var randomBytes [userIDRandomByteLength]byte

	if _, err := io.ReadFull(
		generator.random,
		randomBytes[:],
	); err != nil {
		return "",
			fmt.Errorf(
				"generate user ID: %w",
				err,
			)
	}

	if err := ctx.Err(); err != nil {
		return "", err
	}

	return hex.EncodeToString(
		randomBytes[:],
	), nil
}
```

## Isi final `user_id_generator_test.go`

Listing berikut harus sama persis dengan source setelah `gofmt`.

```go
package identity

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"
)

var errUserIDRandomRead = errors.New(
	"practice random reader failed",
)

type userIDReaderFunc func(
	buffer []byte,
) (int, error)

func (
	function userIDReaderFunc,
) Read(
	buffer []byte,
) (int, error) {
	return function(buffer)
}

func TestNewRandomUserIDGenerator(
	t *testing.T,
) {
	generator := NewRandomUserIDGenerator()

	if generator == nil {
		t.Fatal(
			"NewRandomUserIDGenerator() returned nil",
		)
	}

	if generator.random == nil {
		t.Fatal(
			"NewRandomUserIDGenerator() did not configure random reader",
		)
	}
}

func TestRandomUserIDGeneratorGenerate(
	t *testing.T,
) {
	randomInput := []byte{
		0x00, 0x01, 0x02, 0x03,
		0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b,
		0x0c, 0x0d, 0x0e, 0x0f,
	}

	generator := &RandomUserIDGenerator{
		random: bytes.NewReader(randomInput),
	}

	userID, err := generator.Generate(
		context.Background(),
	)
	if err != nil {
		t.Fatalf(
			"Generate() error = %v, want nil",
			err,
		)
	}

	const wantUserID = "000102030405060708090a0b0c0d0e0f"

	if userID != wantUserID {
		t.Errorf(
			"Generate() = %q, want %q",
			userID,
			wantUserID,
		)
	}
}

func TestRandomUserIDGeneratorGenerateUsesFreshRandomness(
	t *testing.T,
) {
	randomInput := append(
		bytes.Repeat(
			[]byte{0x11},
			userIDRandomByteLength,
		),
		bytes.Repeat(
			[]byte{0x22},
			userIDRandomByteLength,
		)...,
	)

	generator := &RandomUserIDGenerator{
		random: bytes.NewReader(randomInput),
	}

	firstUserID, err := generator.Generate(
		context.Background(),
	)
	if err != nil {
		t.Fatalf(
			"first Generate() error = %v, want nil",
			err,
		)
	}

	secondUserID, err := generator.Generate(
		context.Background(),
	)
	if err != nil {
		t.Fatalf(
			"second Generate() error = %v, want nil",
			err,
		)
	}

	const wantFirstUserID = "11111111111111111111111111111111"
	const wantSecondUserID = "22222222222222222222222222222222"

	if firstUserID != wantFirstUserID {
		t.Errorf(
			"first Generate() = %q, want %q",
			firstUserID,
			wantFirstUserID,
		)
	}

	if secondUserID != wantSecondUserID {
		t.Errorf(
			"second Generate() = %q, want %q",
			secondUserID,
			wantSecondUserID,
		)
	}

	if firstUserID == secondUserID {
		t.Fatal(
			"successive Generate() calls returned the same user ID",
		)
	}
}

func TestRandomUserIDGeneratorRejectsMissingConfiguration(
	t *testing.T,
) {
	tests := []struct {
		name      string
		generator *RandomUserIDGenerator
	}{
		{
			name:      "nil receiver",
			generator: nil,
		},
		{
			name:      "nil random reader",
			generator: &RandomUserIDGenerator{},
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				_, err := test.generator.Generate(
					context.Background(),
				)

				if !errors.Is(
					err,
					ErrUserIDGeneratorNotConfigured,
				) {
					t.Fatalf(
						"Generate() error = %v, want %v",
						err,
						ErrUserIDGeneratorNotConfigured,
					)
				}
			},
		)
	}
}

func TestRandomUserIDGeneratorForwardsReaderError(
	t *testing.T,
) {
	generator := &RandomUserIDGenerator{
		random: userIDReaderFunc(
			func(
				_ []byte,
			) (int, error) {
				return 0, errUserIDRandomRead
			},
		),
	}

	userID, err := generator.Generate(
		context.Background(),
	)

	if userID != "" {
		t.Errorf(
			"Generate() user ID = %q, want empty",
			userID,
		)
	}

	if !errors.Is(
		err,
		errUserIDRandomRead,
	) {
		t.Fatalf(
			"Generate() error = %v, want wrapped %v",
			err,
			errUserIDRandomRead,
		)
	}
}

func TestRandomUserIDGeneratorHonorsCanceledContext(
	t *testing.T,
) {
	t.Run(
		"before random read",
		func(t *testing.T) {
			ctx, cancel :=
				context.WithCancel(
					context.Background(),
				)
			cancel()

			readerCalled := false
			generator := &RandomUserIDGenerator{
				random: userIDReaderFunc(
					func(
						buffer []byte,
					) (int, error) {
						readerCalled = true

						return len(buffer), nil
					},
				),
			}

			userID, err := generator.Generate(ctx)

			if !errors.Is(
				err,
				context.Canceled,
			) {
				t.Fatalf(
					"Generate() error = %v, want %v",
					err,
					context.Canceled,
				)
			}

			if userID != "" {
				t.Errorf(
					"Generate() user ID = %q, want empty",
					userID,
				)
			}

			if readerCalled {
				t.Fatal(
					"Generate() read randomness after context cancellation",
				)
			}
		},
	)

	t.Run(
		"during random read",
		func(t *testing.T) {
			ctx, cancel :=
				context.WithCancel(
					context.Background(),
				)

			generator := &RandomUserIDGenerator{
				random: userIDReaderFunc(
					func(
						buffer []byte,
					) (int, error) {
						for index := range buffer {
							buffer[index] =
								byte(index)
						}

						cancel()

						return len(buffer), nil
					},
				),
			}

			userID, err := generator.Generate(ctx)

			if !errors.Is(
				err,
				context.Canceled,
			) {
				t.Fatalf(
					"Generate() error = %v, want %v",
					err,
					context.Canceled,
				)
			}

			if userID != "" {
				t.Errorf(
					"Generate() user ID = %q, want empty",
					userID,
				)
			}
		},
	)
}

func TestRegistrationInputsDoNotExposeUserID(
	t *testing.T,
) {
	tests := []struct {
		name  string
		input any
	}{
		{
			name:  "register input",
			input: RegisterInput{},
		},
		{
			name:  "password register input",
			input: PasswordRegisterInput{},
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				inputType := reflect.TypeOf(
					test.input,
				)

				if _, found :=
					inputType.FieldByName(
						"UserID",
					); found {
					t.Fatalf(
						"%s exposes client-controlled UserID",
						inputType.Name(),
					)
				}
			},
		)
	}
}
```

## Penjelasan setiap test

### `TestNewRandomUserIDGenerator`

Memastikan constructor mengembalikan pointer non-nil dan private reader sudah dikonfigurasi. Test tidak membandingkan identity object `rand.Reader`; contract yang dikunci adalah tersedianya reader sehingga generator siap dipakai.

### `TestRandomUserIDGeneratorGenerate`

Memasang `bytes.Reader` berisi byte `0x00` sampai `0x0f`. Hasil harus persis `000102030405060708090a0b0c0d0e0f`. Test ini mengunci jumlah input, urutan byte, lowercase hex, ketiadaan hyphen, dan deterministic encoding—bukan kualitas entropy production.

### `TestRandomUserIDGeneratorGenerateUsesFreshRandomness`

Reader berisi dua blok 16 byte: seluruh byte `0x11`, lalu seluruh byte `0x22`. Dua pemanggilan `Generate` harus mengonsumsi blok berbeda dan menghasilkan dua ID exact yang berbeda. Ini membuktikan method membaca lagi pada setiap call, tetapi bukan statistical randomness test atau global uniqueness proof.

### `TestRandomUserIDGeneratorRejectsMissingConfiguration`

Dua subtest mencakup nil receiver dan concrete value dengan nil reader. Keduanya harus mengembalikan `ErrUserIDGeneratorNotConfigured`. Guard mencegah panic dan membedakan configuration failure dari entropy-source failure.

### `TestRandomUserIDGeneratorForwardsReaderError`

`userIDReaderFunc` mengembalikan sentinel `errUserIDRandomRead`. Hasil ID harus kosong dan `errors.Is` harus menemukan sentinel di dalam wrapped error. Test ini mengunci `%w`, bukan exact full error string.

### `TestRandomUserIDGeneratorHonorsCanceledContext`

Subtest `before random read` membatalkan context lebih dahulu, memastikan `context.Canceled`, ID kosong, dan reader sama sekali tidak dipanggil. Subtest `during random read` mengisi buffer lalu memanggil `cancel`; post-read check harus membuang hasil dan mengembalikan `context.Canceled`.

Nama `during random read` menggambarkan cancellation dipicu dari callback reader. Karena read synchronous, test tidak membuktikan blocked read dapat diinterupsi; callback tetap return sebelum generator melihat cancellation.

### `TestRegistrationInputsDoNotExposeUserID`

Reflection memeriksa `RegisterInput{}` serta `PasswordRegisterInput{}` dan gagal bila exact exported field `UserID` ditemukan. Ini structural guard untuk bentuk input practice saat ini. Test tidak menggantikan behavioral registration service test Step 42 atau HTTP DTO test Step 52–53.

### `errUserIDRandomRead` dan `userIDReaderFunc`

Keduanya merupakan test fixture/helper, bukan production API dan bukan test terpisah. Sentinel dipakai untuk `errors.Is`; function adapter membuat inline function memenuhi `io.Reader` tanpa struct fake yang lebih besar.

## Workflow implementasi dan verifikasi

Source diformat lebih dahulu:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server

gofmt -w \
  internal/domain/identity/user_id_generator.go \
  internal/domain/identity/user_id_generator_test.go
```

Focused test command:

```bash
env -u GOROOT \
  GOCACHE=/private/tmp/bbge-step40-go-cache \
  /opt/homebrew/bin/go test ./internal/domain/identity \
  -run '^(TestNewRandomUserIDGenerator|TestRandomUserIDGeneratorGenerate|TestRandomUserIDGeneratorGenerateUsesFreshRandomness|TestRandomUserIDGeneratorRejectsMissingConfiguration|TestRandomUserIDGeneratorForwardsReaderError|TestRandomUserIDGeneratorHonorsCanceledContext|TestRegistrationInputsDoNotExposeUserID)$' \
  -count=1 \
  -v
```

Verifikasi module, race detector, dan vet:

```bash
env -u GOROOT \
  GOCACHE=/private/tmp/bbge-step40-go-cache \
  /opt/homebrew/bin/go test ./... -count=1

env -u GOROOT \
  GOCACHE=/private/tmp/bbge-step40-go-cache \
  /opt/homebrew/bin/go test -race \
  ./internal/domain/identity -count=1

env -u GOROOT \
  GOCACHE=/private/tmp/bbge-step40-go-cache \
  /opt/homebrew/bin/go vet ./...
```

## Hasil verifikasi aktual

Seluruh package selesai sukses pada latest run:

```text
ok  github.com/imamsyafii-dev/bbge-practice/srv-server                          0.017s
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/config                   0.015s
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/db                       0.016s
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/app/account     0.014s
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/domain/account  0.016s
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/domain/identity 0.281s
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/router          0.034s
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/store           0.017s
?   github.com/imamsyafii-dev/bbge-practice/srv-server/pkg/postgres             [no test files]
```

Identity package dengan race detector selesai sukses:

```text
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/domain/identity  3.155s
```

`go vet ./...` selesai tanpa diagnostic. Durasi adalah hasil run tersebut, bukan benchmark randomness atau target performance generator.

## Batas dan hal yang sengaja belum dikerjakan

Step 40 tidak mencakup:

- `UserIDGenerator` interface;
- fake generator untuk consumer registration service;
- behavioral proof bahwa service memakai generated ID;
- registration service dan dependency wiring;
- device-as-session entity;
- database schema atau unique constraint practice;
- collision retry policy;
- Redis internal UID allocation;
- production custom UUIDv8 generation atau extraction;
- compatibility/migration antara practice ID dan production ID;
- HTTP request decoding atau unknown-field rejection;
- persistence, transaction, dan rollback behavior;
- metrics, tracing, atau entropy-source health monitoring;
- distributed coordination antarprocess.

Keberhasilan test Step 40 membuktikan contract concrete generator dan bentuk input yang diperiksa. Ia tidak membuktikan keamanan atau uniqueness end-to-end sistem registration.

## Pitfall umum

1. Menambahkan `UserID` ke request agar client dapat memilih identity. Ownership harus tetap di server.
2. Menyebut output practice sebagai UUIDv8 hanya karena panjangnya 32 hex. Version, variant, dan layout production tidak ada.
3. Menganggap 128-bit randomness menjamin tidak pernah collision. Database unique constraint tetap diperlukan.
4. Menggunakan `math/rand`, timestamp saja, counter process-local, atau predictable seed untuk identity.
5. Memanggil `Read` sekali dan menerima partial buffer. Gunakan `io.ReadFull` bila contract membutuhkan seluruh 16 byte.
6. Mengabaikan error `io.ReadFull` lalu meng-encode buffer parsial atau zero-filled.
7. Menggunakan `%v` alih-alih `%w` lalu mengharapkan `errors.Is` tetap menemukan reader error.
8. Menganggap context dapat menghentikan arbitrary blocked `io.Reader`. Step 40 hanya memeriksa sebelum dan setelah read.
9. Mengekspor reader injection pada API tanpa kebutuhan, sehingga caller production dapat memasang randomness lemah.
10. Menganggap reflection test membuktikan handler dan service aman dari client-controlled ID. Ia hanya memeriksa exact field Go.
11. Membuat interface di package implementation sebelum consumer ada. Interface `UserIDGenerator` ditunda sampai Step 42.
12. Menambahkan prefix `practice_`: prefix 9 karakter ditambah 32 hex menjadi 41 karakter, melewati `VARCHAR(40)` production dan membocorkan environment ke identity.
13. Mencoba mengekstrak internal UID dari random ID practice. Tidak ada internal UID yang di-embed.
14. Mengklaim Redis increment production ikut rollback bersama PostgreSQL transaction. Keduanya adalah state boundary berbeda.
15. Menambahkan library UUID eksternal padahal Step 40 tidak membutuhkan UUID semantics.

## Recap

Step 40 mengunci primitive kecil berikut:

```text
server-controlled generation
16 bytes from crypto/rand.Reader
128 random bits
32 lowercase hexadecimal characters
opaque, no embedded fields
io.ReadFull for complete input
private io.Reader test seam
context check before and after read
wrapped reader errors
registration input has no UserID field
no UserIDGenerator interface yet
```

Production memakai desain lebih kaya: Redis `INCR` menghasilkan internal UID, custom UUIDv8 meng-embed internal UID bersama timestamp dan random fields, lalu PostgreSQL menjaga uniqueness. Practice tidak meniru layout tersebut dan tidak kompatibel dengannya; practice hanya mengambil invariant ownership bahwa server menciptakan `user_id`.

## Preview Step 41

Step 41 mendefinisikan device-as-session beserta ownership terhadap account, refresh token, status, issued-at, dan expired-at. Setelah generator account dan bentuk device tersedia, Step 42 dapat membangun registration domain service, mendefinisikan consumer-owned `UserIDGenerator` interface, serta menguji orkestrasi dengan fake generator, fake hasher, dan entity device tanpa persistence nyata.
