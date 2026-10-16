# Step 39 — Argon2id password hashing

Step ini mengisi boundary `PasswordHasher` dari Step 38 dengan implementasi concrete Argon2id. Password plaintext diubah menjadi encoded hash yang menyimpan algoritma, versi, parameter, random salt, dan derived key. Concrete hasher juga menyediakan verification untuk membuktikan password benar diterima serta password salah ditolak, tetapi interface registrasi tetap sekecil kebutuhan consumer: hanya `Hash`.

Implementasi ini adalah latihan yang sengaja dibatasi. Ia meniru keputusan penting dari source production aktif, tetapi tidak boleh disebut sebagai bukti keamanan production lengkap atau pengganti threat model, audit kriptografi, rate limiting lintas process, monitoring, dan pengelolaan credential end-to-end.

## Tujuan dan scope

Setelah Step 39, practice mempunyai:

- concrete `Argon2IDPasswordHasher` yang memenuhi interface `PasswordHasher`;
- Argon2id v=19 dengan parameter yang dikunci: `m=19456` KiB, `t=2`, dan `p=1`;
- random salt 16 byte dari `crypto/rand` dan derived key 32 byte;
- encoding tetap bergaya PHC dengan raw standard Base64 tanpa padding;
- parser ketat yang hanya menerima satu format dan satu set parameter;
- verification dengan `subtle.ConstantTimeCompare` untuk dua byte slice key berukuran tetap;
- dummy salt dan dummy expected key agar malformed hash tetap mencapai pekerjaan Argon2 yang bounded;
- semaphore `chan struct{}` yang membatasi jumlah KDF yang berjalan bersamaan dan menolak beban berlebih tanpa antrean;
- pemeriksaan context sebelum serta setelah KDF;
- test untuk constructor, hashing, verification, parser, dummy work, kapasitas, cancellation, dan nil receiver;
- dependency langsung `golang.org/x/crypto v0.49.0`.

Step ini tidak menambahkan persistence, registration service, login use case, HTTP endpoint, rate limiter terdistribusi, pepper, parameter migration, metrics, maupun konfigurasi Argon2 melalui environment.

## Posisi dalam alur belajar

Step 38 sudah menyediakan:

```text
PasswordHasher.Hash(ctx, plaintext) -> EncodedPasswordHash
```

Step 39 menambahkan implementasi nyata:

```text
plaintext password
    -> random salt 16 byte
    -> Argon2id(password, salt, t=2, m=19456 KiB, p=1)
    -> derived key 32 byte
    -> fixed PHC-style encoding
    -> EncodedPasswordHash
```

Verification melakukan arah berikut:

```text
EncodedPasswordHash
    -> strict fixed-format parser
    -> salt + expected key, atau dummy salt + dummy key
plaintext candidate
    -> Argon2id dengan parameter lokal yang dikunci
    -> actual key
actual key + expected key
    -> constant-time byte comparison
    -> valid && matched
```

Hash baru dipersist pada step persistence berikutnya. Step ini hanya membuat dan memverifikasi nilainya di domain identity.

## Perbandingan dengan source production aktif

Source production yang dibandingkan berada di:

```text
bbge-biz/srv-server/internal/domain/identity/password_crypto.go
bbge-biz/srv-server/internal/domain/identity/password.go
```

Practice mengikuti production pada keputusan inti berikut:

- memakai `argon2.IDKey` atau Argon2id;
- memakai Argon2 version 19 dalam encoding;
- memakai `t=2`, `m=19456` KiB, `p=1`, salt 16 byte, dan key 32 byte;
- mengambil salt dari `crypto/rand`;
- memakai raw standard Base64;
- membandingkan derived key dengan `subtle.ConstantTimeCompare`;
- membatasi jumlah pekerjaan Argon2 per process dengan buffered channel;
- menolak ketika seluruh slot sedang digunakan, bukan membuat antrean tak terbatas;
- memeriksa context sebelum dan setelah derivation;
- tetap menjalankan bounded KDF work untuk stored hash yang malformed.

Perbedaan practice yang disengaja:

- concrete type dan constructor diekspor sebagai `Argon2IDPasswordHasher` dan `NewArgon2IDPasswordHasher` untuk latihan;
- hasil hash memakai typed wrapper `EncodedPasswordHash`, bukan `string` langsung;
- constructor menolak concurrency nol atau negatif;
- method memeriksa receiver nil dan channel yang belum dikonfigurasi;
- parser practice memeriksa panjang encoded total terlebih dahulu dan mengharuskan struktur canonical yang persis;
- error pembentukan salt diberi context `generate password salt`;
- byte slice plaintext, salt, expected key, dan derived key dibersihkan secara best-effort dengan `clear`;
- practice belum mempunyai Prometheus histogram, gauge, dan counter milik production;
- practice belum tersambung ke registration/login service atau persistence production.

Parser practice yang lebih ketat bukan parser PHC umum. Ia sengaja tidak menerima variasi parameter, urutan parameter berbeda, versi lain, algoritma lain, padding Base64, maupun field tambahan.

## Mengapa Argon2id

Keluarga Argon2 mempunyai beberapa varian:

- Argon2d memakai akses memori yang bergantung pada data. Sifat ini kuat terhadap serangan komputasi paralel tertentu, tetapi pola aksesnya lebih berisiko terhadap side-channel pada environment yang relevan.
- Argon2i memakai akses memori yang tidak bergantung pada data untuk mengurangi risiko side-channel, dengan trade-off kerja tambahan untuk resistance tertentu.
- Argon2id menggabungkan pendekatan keduanya. Bagian awal memakai strategi Argon2i, lalu bagian berikutnya memakai strategi Argon2d.

Step ini memilih Argon2id karena sesuai dengan source production aktif dan merupakan pilihan hybrid untuk password hashing. Pemilihan nama algoritma saja tidak cukup: keamanan serta kapasitas operasional tetap bergantung pada parameter, resource limit, rate limiting, penyimpanan credential, deployment, dan kemampuan melakukan upgrade parameter.

## Parameter yang dikunci

Konstanta implementasi adalah:

```go
const (
    passwordHashTime        uint32 = 2
    passwordHashMemory      uint32 = 19456
    passwordHashParallelism uint8  = 1
    passwordHashSaltLength         = 16
    passwordHashKeyLength          = 32
)
```

Maknanya:

| Parameter | Nilai | Makna |
|---|---:|---|
| `t` / time | 2 | Jumlah pass/iteration Argon2. Ini bukan durasi dalam detik. |
| `m` / memory | 19456 KiB | Memori KDF sekitar 19 MiB untuk satu derivation. |
| `p` / parallelism | 1 | Satu lane Argon2. Ini berbeda dari jumlah request yang boleh bersamaan. |
| salt | 16 byte | Nilai random baru untuk setiap pemanggilan `Hash`. |
| key | 32 byte | Panjang derived key yang dibandingkan saat verification. |

Konversi memorinya:

```text
19456 KiB / 1024 = 19 MiB
```

Parameter ini sama dengan source production saat Step 39 ditulis. Ia dikunci di kode practice, bukan dibaca dari encoded input, sehingga stored value tidak dapat meminta alokasi seperti `m=999999999`.

## Bentuk encoded hash

Prefix exact yang dihasilkan adalah:

```text
$argon2id$v=19$m=19456,t=2,p=1$
```

Nilai lengkap berbentuk:

```text
$argon2id$v=19$m=19456,t=2,p=1$<salt-base64-raw>$<key-base64-raw>
```

Pecahannya:

| Bagian | Nilai atau arti |
|---|---|
| field pertama | Kosong karena string dimulai dengan `$`. |
| algorithm | `argon2id`. |
| version | `v=19`. |
| parameters | `m=19456,t=2,p=1`. |
| salt | 16 byte yang di-encode raw standard Base64. |
| key | 32 byte yang di-encode raw standard Base64. |

`base64.RawStdEncoding` memakai alfabet standard Base64 tetapi tidak menambahkan padding `=`. Akibatnya, salt 16 byte menjadi 22 karakter dan key 32 byte menjadi 43 karakter. Parser menolak padded Base64 sehingga contract storage memakai bentuk raw-unpadded yang tetap.

Ada batas kecil yang perlu dicatat: code memakai `base64.RawStdEncoding.DecodeString`, bukan `base64.RawStdEncoding.Strict().DecodeString`. Jadi parser mengunci alfabet, padding, panjang decoded, field, dan parameter, tetapi tidak meminta pemeriksaan strict atas unused trailing bits Base64. Istilah "parser ketat" dalam lesson ini merujuk pada contract format tetap tersebut, bukan klaim bahwa decoder menerima tepat satu bit-level representation untuk setiap byte sequence.

Format ini bergaya PHC karena menyimpan identitas algoritma, versi, parameter, salt, dan hasil dalam field yang dipisahkan `$`. Namun fungsi `parseEncodedPasswordHash` bukan general-purpose PHC decoder; ia adalah parser contract tetap untuk satu format Step 39.

## Salt dari `crypto/rand`

Setiap `Hash` membuat buffer 16 byte lalu mengisinya melalui:

```go
rand.Read(salt)
```

`crypto/rand`, bukan `math/rand`, digunakan karena salt memerlukan randomness kriptografis. Salt tidak perlu dirahasiakan dan disimpan bersama hash. Tujuannya adalah membuat password sama menghasilkan encoded hash berbeda serta mencegah precomputation tunggal berlaku langsung untuk semua credential.

Test memang memastikan dua hash untuk plaintext yang sama tidak identik. Test tersebut memberi bukti perilaku pada eksekusi itu, bukan pembuktian matematis atas kualitas seluruh sumber entropy platform.

## Parser ketat dan perlindungan parameter hostile

`parseEncodedPasswordHash` memulai dengan dummy salt 16 byte dan dummy expected key 32 byte. Sebelum decode, parser menghitung panjang encoded yang persis diharapkan dari prefix, salt, separator, dan key.

Input baru dianggap valid bila seluruh syarat berikut terpenuhi:

- panjang total persis sama dengan format canonical;
- hasil `strings.Split(value, "$")` mempunyai tepat enam bagian;
- bagian pertama kosong;
- algorithm persis `argon2id`;
- version persis `v=19`;
- parameter persis `m=19456,t=2,p=1`;
- salt serta key valid menurut `base64.RawStdEncoding`;
- decoded salt tepat 16 byte;
- decoded expected key tepat 32 byte.

Karena implementation selalu memanggil `argon2.IDKey` dengan konstanta lokal, parameter dari stored string tidak pernah diteruskan sebagai angka ke KDF. Contoh hostile berikut ditolak:

```text
$argon2id$v=19$m=999999999,t=99,p=255$x$x
```

Test membedakan dua lapisan penolakan. `argon2zz`, `v=18`, dan `m=19457,t=2,p=1` sengaja mempunyai panjang yang sama dengan field valid, sehingga lolos dari total-length precheck dan benar-benar menguji guard algorithm, version, serta fixed parameters. Hostile parameters yang jauh lebih panjang menguji bahwa total-length precheck menolak input sebelum parser mencoba memperlakukannya sebagai parameter KDF.

Tanpa contract ini, parser yang terlalu permisif dapat membuat attacker-controlled stored value meminta resource ekstrem. Perlindungan Step 39 hanya mencakup path parser ini; ia bukan pengganti validasi pada boundary storage atau kontrol akses yang lebih luas.

## Dummy work pada malformed hash

Jika parsing gagal, `Verify` tidak langsung return `false`. Parser mengembalikan dummy salt dan dummy expected key dengan panjang yang benar, lalu `Verify` tetap memanggil `derive`:

```text
malformed encoded hash
    -> dummy salt 16 byte + dummy key 32 byte
    -> bounded Argon2id work
    -> constant-time key comparison
    -> valid == false
    -> hasil akhir false
```

Tujuannya adalah menghindari perbedaan paling kasar antara jalur hash valid yang mahal dan malformed hash yang langsung selesai. Test membuktikan malformed value tetap mencoba mengambil slot derivation: ketika slot sengaja dipenuhi, `Verify` menghasilkan `ErrPasswordHasherBusy`.

Ini tidak membuktikan semua jalur mempunyai timing yang identik. Parsing, allocation, decoding, scheduling, cache, runtime, serta kondisi mesin masih dapat menimbulkan variasi. Dummy work adalah hardening terbatas, bukan janji perfect constant-time behavior untuk seluruh fungsi.

## Perbandingan constant-time

Actual key dan expected key dibandingkan dengan:

```go
subtle.ConstantTimeCompare(actual, expected) == 1
```

Kedua slice selalu ditargetkan sepanjang 32 byte: parser valid menghasilkan key 32 byte, sedangkan parser invalid menghasilkan dummy key 32 byte. Primitive ini menghindari early exit berdasarkan byte pertama yang berbeda pada perbandingan key.

Jaminannya harus disebut secara sempit: perbandingan dua byte slice berukuran sama memakai constant-time primitive. Seluruh method `Verify`, termasuk parsing, Base64 decoding, Argon2 execution, context, dan scheduling, tidak otomatis menjadi perfectly constant-time.

Hasil akhirnya memakai:

```go
return valid && matched, nil
```

Jadi kecocokan terhadap dummy zero key tidak dapat membuat malformed hash diterima.

## Semaphore `chan struct{}` dan batas memori

Struct concrete menyimpan:

```go
type Argon2IDPasswordHasher struct {
    slots chan struct{}
}
```

`slots` adalah field bertipe buffered channel yang membawa empty struct. Empty struct tidak membawa payload; keberadaan elemennya hanya menandai satu slot sedang dipakai.

Constructor membuat channel dengan kapasitas `concurrency`:

```go
slots: make(chan struct{}, concurrency)
```

`derive` mencoba mengirim marker:

```go
select {
case hasher.slots <- struct{}{}:
    defer func() {
        <-hasher.slots
    }()
default:
    return nil, ErrPasswordHasherBusy
}
```

Jika slot tersedia, pengiriman berhasil dan deferred receive melepasnya ketika fungsi selesai. Jika semua slot penuh, `default` langsung mengembalikan busy. Tidak ada antrean tunggu internal.

Setiap KDF dikonfigurasi sekitar 19 MiB. Perkiraan batas memori KDF aktif adalah:

```text
concurrency=1 -> sekitar 19 MiB
concurrency=2 -> sekitar 38 MiB
concurrency=4 -> sekitar 76 MiB
```

Ini adalah perkiraan memori untuk pekerjaan Argon2 aktif, bukan batas keras bagi seluruh process. Runtime, goroutine, input/output buffer, library, allocator overhead, dan komponen aplikasi lain tetap memakai memori. Nilai concurrency harus dipilih berdasarkan memory budget dan pola traffic deployment nyata.

`p=1` dan `concurrency` berbeda. `p=1` adalah parallelism internal satu derivation, sedangkan kapasitas channel adalah jumlah derivation yang boleh berjalan serentak dalam instance hasher pada satu process.

## Method receiver `hasher`

Signature berikut:

```go
func (
    hasher *Argon2IDPasswordHasher,
) Hash(
    ctx context.Context,
    plaintext string,
) (EncodedPasswordHash, error)
```

terdiri dari:

```text
(hasher *Argon2IDPasswordHasher)  method receiver
Hash                              nama method
(ctx, plaintext)                  parameter input
(EncodedPasswordHash, error)      hasil
```

`hasher` bukan parameter bisnis seperti `ctx` atau `plaintext`; ia adalah receiver yang menghubungkan method `Hash` ke instance `Argon2IDPasswordHasher`. Melalui receiver tersebut, method mengakses `hasher.slots` yang dimiliki instance.

Receiver memakai pointer agar seluruh pemanggilan pada instance yang sama berbagi channel semaphore yang sama dan agar nil receiver dapat dideteksi. Nil receiver atau instance dengan `slots == nil` menghasilkan `ErrPasswordHasherNotConfigured`, bukan panic.

## Context sebelum dan setelah KDF

`derive` memeriksa:

```go
if err := ctx.Err(); err != nil {
    return nil, err
}
```

sebelum mengambil slot dan sebelum menjalankan Argon2. Setelah `argon2.IDKey` selesai, context diperiksa lagi. Jika request dibatalkan selama computation, hasil dibersihkan lalu cancellation dikembalikan.

Batas pentingnya: `argon2.IDKey` tidak menerima context. Setelah pemanggilan dimulai, code Step 39 tidak dapat menghentikannya di tengah KDF. Cancellation baru diamati lagi sesudah KDF selesai. Context membantu menghindari pekerjaan yang belum dimulai dan mencegah hasil yang sudah tidak dibutuhkan diteruskan, tetapi bukan preemption primitive untuk Argon2.

## Pembersihan byte slice

Implementasi memakai `clear` pada salt, plaintext bytes, derived key, parsed expected key, dan buffer dummy. Ini merupakan usaha mengurangi lamanya material sensitif berada dalam byte slice yang masih dapat dijangkau.

Batasnya:

- input `plaintext` berasal dari immutable Go `string`, sehingga konversi dan `clear(plaintextBytes)` tidak menghapus string asal;
- compiler, runtime, allocator, dan copy internal dapat memengaruhi keberadaan salinan;
- salt tidak rahasia, walaupun buffer tetap dibersihkan secara konsisten;
- Step 39 tidak menjanjikan secure memory locking atau penghapusan fisik yang dapat diaudit.

Karena itu `clear` disebut best-effort hygiene, bukan jaminan complete secret erasure.

## Interface Hash-only dan concrete Verify

Compile-time assertion:

```go
var _ PasswordHasher = (*Argon2IDPasswordHasher)(nil)
```

membuktikan pointer concrete memenuhi interface Step 38. Interface tersebut tetap:

```go
type PasswordHasher interface {
    Hash(
        ctx context.Context,
        plaintext string,
    ) (EncodedPasswordHash, error)
}
```

Concrete type mempunyai kemampuan tambahan:

```go
Verify(ctx, plaintext, encoded) (bool, error)
```

Registrasi hanya membutuhkan `Hash`, jadi interface registrasi tidak diperbesar. `Verify` diuji melalui concrete type untuk memvalidasi implementation. Ketika login practice dibuat, consumer login dapat mendefinisikan interface verifier kecil miliknya sendiri. Ini menjaga interface berbasis kebutuhan consumer, bukan daftar semua method implementation.

## Isi final `password_hasher.go`

Listing berikut harus sama persis dengan source setelah `gofmt`.

```go
package identity

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	passwordHashTime        uint32 = 2
	passwordHashMemory      uint32 = 19456
	passwordHashParallelism uint8  = 1
	passwordHashSaltLength         = 16
	passwordHashKeyLength          = 32

	passwordHashPrefix = "$argon2id$v=19$m=19456,t=2,p=1$"
)

var (
	ErrPasswordHasherConcurrencyInvalid = errors.New(
		"password hasher concurrency must be positive",
	)
	ErrPasswordHasherNotConfigured = errors.New(
		"password hasher is not configured",
	)
	ErrPasswordHasherBusy = errors.New(
		"password hasher is busy",
	)
)

type Argon2IDPasswordHasher struct {
	slots chan struct{}
}

func NewArgon2IDPasswordHasher(
	concurrency int,
) (*Argon2IDPasswordHasher, error) {
	if concurrency <= 0 {
		return nil,
			ErrPasswordHasherConcurrencyInvalid
	}

	return &Argon2IDPasswordHasher{
		slots: make(
			chan struct{},
			concurrency,
		),
	}, nil
}

var _ PasswordHasher = (*Argon2IDPasswordHasher)(nil)

func (
	hasher *Argon2IDPasswordHasher,
) Hash(
	ctx context.Context,
	plaintext string,
) (EncodedPasswordHash, error) {
	if err := hasher.ensureConfigured(); err != nil {
		return EncodedPasswordHash{}, err
	}

	if err := ctx.Err(); err != nil {
		return EncodedPasswordHash{}, err
	}

	salt := make(
		[]byte,
		passwordHashSaltLength,
	)
	defer clear(salt)

	if _, err := rand.Read(salt); err != nil {
		return EncodedPasswordHash{},
			fmt.Errorf(
				"generate password salt: %w",
				err,
			)
	}

	key, err := hasher.derive(
		ctx,
		plaintext,
		salt,
	)
	if err != nil {
		return EncodedPasswordHash{}, err
	}
	defer clear(key)

	encoded :=
		passwordHashPrefix +
			base64.RawStdEncoding.EncodeToString(
				salt,
			) +
			"$" +
			base64.RawStdEncoding.EncodeToString(
				key,
			)

	return newEncodedPasswordHash(encoded)
}

func (
	hasher *Argon2IDPasswordHasher,
) Verify(
	ctx context.Context,
	plaintext string,
	encoded EncodedPasswordHash,
) (bool, error) {
	if err := hasher.ensureConfigured(); err != nil {
		return false, err
	}

	salt, expected, valid :=
		parseEncodedPasswordHash(
			encoded.Encoded(),
		)
	defer clear(salt)
	defer clear(expected)

	actual, err := hasher.derive(
		ctx,
		plaintext,
		salt,
	)
	if err != nil {
		return false, err
	}
	defer clear(actual)

	matched :=
		subtle.ConstantTimeCompare(
			actual,
			expected,
		) == 1

	return valid && matched, nil
}

func (
	hasher *Argon2IDPasswordHasher,
) ensureConfigured() error {
	if hasher == nil || hasher.slots == nil {
		return ErrPasswordHasherNotConfigured
	}

	return nil
}

func (
	hasher *Argon2IDPasswordHasher,
) derive(
	ctx context.Context,
	plaintext string,
	salt []byte,
) ([]byte, error) {
	if err := hasher.ensureConfigured(); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	select {
	case hasher.slots <- struct{}{}:
		defer func() {
			<-hasher.slots
		}()
	default:
		return nil, ErrPasswordHasherBusy
	}

	plaintextBytes := []byte(plaintext)
	defer clear(plaintextBytes)

	result := argon2.IDKey(
		plaintextBytes,
		salt,
		passwordHashTime,
		passwordHashMemory,
		passwordHashParallelism,
		passwordHashKeyLength,
	)

	if err := ctx.Err(); err != nil {
		clear(result)

		return nil, err
	}

	return result, nil
}

func parseEncodedPasswordHash(
	value string,
) (
	salt []byte,
	expected []byte,
	valid bool,
) {
	dummySalt := make(
		[]byte,
		passwordHashSaltLength,
	)
	dummyExpected := make(
		[]byte,
		passwordHashKeyLength,
	)

	expectedLength :=
		len(passwordHashPrefix) +
			base64.RawStdEncoding.EncodedLen(
				passwordHashSaltLength,
			) +
			1 +
			base64.RawStdEncoding.EncodedLen(
				passwordHashKeyLength,
			)

	if len(value) != expectedLength {
		return dummySalt, dummyExpected, false
	}

	parts := strings.Split(value, "$")

	if len(parts) != 6 ||
		parts[0] != "" ||
		parts[1] != "argon2id" ||
		parts[2] != "v=19" ||
		parts[3] != "m=19456,t=2,p=1" {
		return dummySalt, dummyExpected, false
	}

	parsedSalt, saltErr :=
		base64.RawStdEncoding.DecodeString(
			parts[4],
		)
	parsedExpected, expectedErr :=
		base64.RawStdEncoding.DecodeString(
			parts[5],
		)

	if saltErr != nil ||
		expectedErr != nil ||
		len(parsedSalt) !=
			passwordHashSaltLength ||
		len(parsedExpected) !=
			passwordHashKeyLength {
		clear(parsedSalt)
		clear(parsedExpected)

		return dummySalt, dummyExpected, false
	}

	clear(dummySalt)
	clear(dummyExpected)

	return parsedSalt, parsedExpected, true
}
```

## Isi final `password_hasher_test.go`

Listing berikut harus sama persis dengan source setelah `gofmt`.

```go
package identity

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestNewArgon2IDPasswordHasher(
	t *testing.T,
) {
	hasher, err :=
		NewArgon2IDPasswordHasher(1)
	if err != nil {
		t.Fatalf(
			"NewArgon2IDPasswordHasher() error = %v, want nil",
			err,
		)
	}

	if hasher == nil {
		t.Fatal(
			"NewArgon2IDPasswordHasher() returned nil",
		)
	}

	for _, concurrency := range []int{0, -1} {
		t.Run(
			"invalid concurrency",
			func(t *testing.T) {
				_, err :=
					NewArgon2IDPasswordHasher(
						concurrency,
					)

				if !errors.Is(
					err,
					ErrPasswordHasherConcurrencyInvalid,
				) {
					t.Fatalf(
						"NewArgon2IDPasswordHasher() error = %v, want %v",
						err,
						ErrPasswordHasherConcurrencyInvalid,
					)
				}
			},
		)
	}
}

func TestArgon2IDPasswordHasherHashAndVerify(
	t *testing.T,
) {
	hasher := mustNewArgon2IDPasswordHasher(
		t,
		1,
	)
	ctx := context.Background()
	password := " practice-password "

	firstHash, err := hasher.Hash(
		ctx,
		password,
	)
	if err != nil {
		t.Fatalf(
			"Hash() error = %v, want nil",
			err,
		)
	}

	if firstHash.Encoded() == password {
		t.Fatal(
			"Hash() returned plaintext password",
		)
	}

	if !strings.HasPrefix(
		firstHash.Encoded(),
		passwordHashPrefix,
	) {
		t.Fatalf(
			"Hash() = %q, want prefix %q",
			firstHash.Encoded(),
			passwordHashPrefix,
		)
	}

	secondHash, err := hasher.Hash(
		ctx,
		password,
	)
	if err != nil {
		t.Fatalf(
			"second Hash() error = %v, want nil",
			err,
		)
	}

	if firstHash.Encoded() ==
		secondHash.Encoded() {
		t.Fatal(
			"same password produced identical hashes",
		)
	}

	tests := []struct {
		name      string
		plaintext string
		want      bool
	}{
		{
			name:      "correct password",
			plaintext: password,
			want:      true,
		},
		{
			name: "trimmed password",
			plaintext: strings.TrimSpace(
				password,
			),
			want: false,
		},
		{
			name:      "wrong password",
			plaintext: "wrong-practice-password",
			want:      false,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				verified, err := hasher.Verify(
					ctx,
					test.plaintext,
					firstHash,
				)
				if err != nil {
					t.Fatalf(
						"Verify() error = %v, want nil",
						err,
					)
				}

				if verified != test.want {
					t.Errorf(
						"Verify() = %v, want %v",
						verified,
						test.want,
					)
				}
			},
		)
	}
}

func TestParseEncodedPasswordHash(
	t *testing.T,
) {
	hasher := mustNewArgon2IDPasswordHasher(
		t,
		1,
	)

	validHash, err := hasher.Hash(
		context.Background(),
		"practice-password",
	)
	if err != nil {
		t.Fatalf(
			"Hash() error = %v",
			err,
		)
	}

	invalidBase64 :=
		passwordHashPrefix +
			strings.Repeat(
				"!",
				base64.RawStdEncoding.EncodedLen(
					passwordHashSaltLength,
				),
			) +
			"$" +
			strings.Repeat(
				"!",
				base64.RawStdEncoding.EncodedLen(
					passwordHashKeyLength,
				),
			)

	tests := []struct {
		name      string
		value     string
		wantValid bool
	}{
		{
			name:      "valid encoded hash",
			value:     validHash.Encoded(),
			wantValid: true,
		},
		{
			name: "wrong algorithm",
			value: strings.Replace(
				validHash.Encoded(),
				"argon2id",
				"argon2zz",
				1,
			),
		},
		{
			name: "wrong version",
			value: strings.Replace(
				validHash.Encoded(),
				"v=19",
				"v=18",
				1,
			),
		},
		{
			name: "wrong fixed parameters",
			value: strings.Replace(
				validHash.Encoded(),
				"m=19456,t=2,p=1",
				"m=19457,t=2,p=1",
				1,
			),
		},
		{
			name: "hostile parameters",
			value: strings.Replace(
				validHash.Encoded(),
				"m=19456,t=2,p=1",
				"m=999999999,t=99,p=255",
				1,
			),
		},
		{
			name:  "invalid base64",
			value: invalidBase64,
		},
		{
			name: "salt has wrong size",
			value: passwordHashPrefix +
				base64.RawStdEncoding.EncodeToString(
					make([]byte, 15),
				) +
				"$" +
				base64.RawStdEncoding.EncodeToString(
					make([]byte, 32),
				),
		},
		{
			name: "key has wrong size",
			value: passwordHashPrefix +
				base64.RawStdEncoding.EncodeToString(
					make([]byte, 16),
				) +
				"$" +
				base64.RawStdEncoding.EncodeToString(
					make([]byte, 31),
				),
		},
		{
			name: "padded base64",
			value: validHash.Encoded() +
				"=",
		},
		{
			name: "extra field",
			value: validHash.Encoded() +
				"$extra",
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				salt, expected, valid :=
					parseEncodedPasswordHash(
						test.value,
					)
				defer clear(salt)
				defer clear(expected)

				if valid != test.wantValid {
					t.Errorf(
						"parseEncodedPasswordHash() valid = %v, want %v",
						valid,
						test.wantValid,
					)
				}

				if len(salt) !=
					passwordHashSaltLength {
					t.Errorf(
						"salt length = %d, want %d",
						len(salt),
						passwordHashSaltLength,
					)
				}

				if len(expected) !=
					passwordHashKeyLength {
					t.Errorf(
						"key length = %d, want %d",
						len(expected),
						passwordHashKeyLength,
					)
				}
			},
		)
	}
}

func TestArgon2IDPasswordHasherMalformedHashUsesDummyWork(
	t *testing.T,
) {
	hasher := mustNewArgon2IDPasswordHasher(
		t,
		1,
	)

	verified, err := hasher.Verify(
		context.Background(),
		"practice-password",
		EncodedPasswordHash{},
	)
	if err != nil {
		t.Fatalf(
			"Verify() error = %v, want nil",
			err,
		)
	}

	if verified {
		t.Fatal(
			"Verify() accepted zero-value hash",
		)
	}

	malformed, err := newEncodedPasswordHash(
		"$argon2id$v=19$m=999999999,t=99,p=255$x$x",
	)
	if err != nil {
		t.Fatalf(
			"newEncodedPasswordHash() error = %v",
			err,
		)
	}

	hasher.slots <- struct{}{}

	_, busyErr := hasher.Verify(
		context.Background(),
		"practice-password",
		malformed,
	)

	<-hasher.slots

	if !errors.Is(
		busyErr,
		ErrPasswordHasherBusy,
	) {
		t.Fatalf(
			"Verify() error = %v, want %v",
			busyErr,
			ErrPasswordHasherBusy,
		)
	}
}

func TestArgon2IDPasswordHasherCapacityAndCancellation(
	t *testing.T,
) {
	hasher := mustNewArgon2IDPasswordHasher(
		t,
		1,
	)

	hasher.slots <- struct{}{}

	_, busyErr := hasher.Hash(
		context.Background(),
		"practice-password",
	)

	<-hasher.slots

	if !errors.Is(
		busyErr,
		ErrPasswordHasherBusy,
	) {
		t.Fatalf(
			"Hash() error = %v, want %v",
			busyErr,
			ErrPasswordHasherBusy,
		)
	}

	ctx, cancel :=
		context.WithCancel(
			context.Background(),
		)
	cancel()

	_, err := hasher.Hash(
		ctx,
		"practice-password",
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"Hash() error = %v, want %v",
			err,
			context.Canceled,
		)
	}

	_, err = hasher.Verify(
		ctx,
		"practice-password",
		EncodedPasswordHash{},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"Verify() error = %v, want %v",
			err,
			context.Canceled,
		)
	}
}

func TestArgon2IDPasswordHasherNilReceiver(
	t *testing.T,
) {
	var hasher *Argon2IDPasswordHasher

	_, err := hasher.Hash(
		context.Background(),
		"practice-password",
	)
	if !errors.Is(
		err,
		ErrPasswordHasherNotConfigured,
	) {
		t.Fatalf(
			"Hash() error = %v, want %v",
			err,
			ErrPasswordHasherNotConfigured,
		)
	}

	_, err = hasher.Verify(
		context.Background(),
		"practice-password",
		EncodedPasswordHash{},
	)
	if !errors.Is(
		err,
		ErrPasswordHasherNotConfigured,
	) {
		t.Fatalf(
			"Verify() error = %v, want %v",
			err,
			ErrPasswordHasherNotConfigured,
		)
	}
}

func mustNewArgon2IDPasswordHasher(
	t *testing.T,
	concurrency int,
) *Argon2IDPasswordHasher {
	t.Helper()

	hasher, err :=
		NewArgon2IDPasswordHasher(
			concurrency,
		)
	if err != nil {
		t.Fatalf(
			"NewArgon2IDPasswordHasher() error = %v",
			err,
		)
	}

	return hasher
}
```

## Penjelasan setiap test

### `TestNewArgon2IDPasswordHasher`

Test memastikan concurrency `1` menghasilkan pointer non-nil tanpa error. Nilai `0` dan `-1` harus menghasilkan `ErrPasswordHasherConcurrencyInvalid`. Kedua subtest memakai nama sumber yang sama, sehingga output verbose Go membedakannya sebagai `invalid_concurrency` dan `invalid_concurrency#01`.

### `TestArgon2IDPasswordHasherHashAndVerify`

Test memakai password dengan spasi tepi untuk memastikan implementation tidak melakukan trim tersembunyi. Hash pertama harus berbeda dari plaintext dan mempunyai prefix exact. Hash kedua dari plaintext yang sama harus berbeda karena salt baru. Verification harus menerima password exact, menolak versi yang sudah di-trim, dan menolak password lain.

### `TestParseEncodedPasswordHash`

Test membuat satu hash valid, lalu menguji seluruh bentuk yang harus ditolak:

- algorithm `argon2zz`, bukan `argon2id`; replacement dengan panjang sama memastikan guard nama algorithm yang menolak, bukan length precheck;
- version `v=18`, bukan `v=19`; panjangnya sama sehingga menguji guard version;
- fixed parameters `m=19457,t=2,p=1`, bukan `m=19456,t=2,p=1`; panjangnya sama sehingga menguji guard parameter exact;
- hostile parameters yang lebih panjang, `m=999999999,t=99,p=255`, untuk menguji total-length precheck;
- karakter Base64 yang invalid;
- salt 15 byte;
- key 31 byte;
- Base64 dengan padding `=`;
- field `$extra`.

Untuk semua kasus, parser harus mengembalikan salt 16 byte dan expected key 32 byte. Pada input invalid, keduanya adalah buffer dummy sehingga caller tetap mempunyai bentuk yang aman untuk bounded derivation dan fixed-length comparison.

### `TestArgon2IDPasswordHasherMalformedHashUsesDummyWork`

Zero-value `EncodedPasswordHash` harus menghasilkan `verified == false` tanpa parse error yang bocor ke caller. Lalu test membentuk hostile encoded value, memenuhi satu-satunya slot secara manual, dan memanggil `Verify`. Hasil `ErrPasswordHasherBusy` membuktikan jalur malformed tetap mencoba masuk ke `derive`; test tidak mengklaim durasi jalur malformed dan valid identik.

### `TestArgon2IDPasswordHasherCapacityAndCancellation`

Ketika slot sudah penuh, `Hash` harus fail-fast dengan `ErrPasswordHasherBusy`. Context yang sudah dibatalkan harus membuat `Hash` dan `Verify` mengembalikan `context.Canceled`. Test ini mengunci pemeriksaan cancellation sebelum KDF; keterbatasan cancellation di tengah `argon2.IDKey` tetap berlaku.

### `TestArgon2IDPasswordHasherNilReceiver`

Pemanggilan `Hash` dan `Verify` melalui nil pointer harus mengembalikan `ErrPasswordHasherNotConfigured`, bukan panic. Ini juga mencakup jalur `ensureConfigured` yang dipakai oleh method public dan `derive`.

### `mustNewArgon2IDPasswordHasher`

Helper ini bukan test terpisah. `t.Helper()` membuat lokasi kegagalan menunjuk ke caller, lalu helper menghentikan test bila constructor gagal sehingga test lain dapat memakai hasher valid dengan ringkas.

## Dependency setelah `go mod tidy`

Step 39 mengimpor:

```go
"golang.org/x/crypto/argon2"
```

Karena dipakai langsung oleh source module, `golang.org/x/crypto v0.49.0` harus berada pada blok direct dependency tanpa komentar `// indirect`. State relevan `go.mod` setelah tidy adalah:

```go
require (
    github.com/jackc/pgx/v5 v5.8.0
    github.com/pressly/goose/v3 v3.27.0
    golang.org/x/crypto v0.49.0
)

require (
    github.com/jackc/pgpassfile v1.0.0 // indirect
    github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
    github.com/jackc/puddle/v2 v2.2.2 // indirect
    github.com/mfridman/interpolate v0.0.2 // indirect
    github.com/sethvargo/go-retry v0.3.0 // indirect
    go.uber.org/multierr v1.11.0 // indirect
    golang.org/x/sync v0.20.0 // indirect
    golang.org/x/sys v0.42.0 // indirect
    golang.org/x/text v0.35.0 // indirect
)
```

Tidy juga memperbarui pencatatan dependency transitif:

```text
go.mod: golang.org/x/sync  v0.19.0 -> v0.20.0
go.mod: golang.org/x/text  v0.34.0 -> v0.35.0
go.mod: golang.org/x/sys   belum tercantum -> v0.42.0 // indirect
go.sum: golang.org/x/sys   checksum v0.41.0 -> checksum v0.42.0
```

Perubahan transitif tersebut adalah hasil module graph saat `golang.org/x/crypto v0.49.0` ditambahkan dan `go mod tidy` dijalankan, bukan tiga dependency yang dipilih manual sebagai requirement domain Step 39. `go.sum` juga menambahkan checksum `x/crypto`, mengganti checksum versi lama untuk module transitif yang diperbarui, dan mempertahankan checksum lain yang masih dibutuhkan graph.

## Masalah awal format dan module metadata

Pada pemeriksaan awal, focused test dapat berjalan, tetapi verifikasi seluruh module meminta pembaruan metadata karena `x/crypto` masih terbaca sebagai indirect walaupun `argon2` sudah diimpor langsung. Dua source baru juga perlu `gofmt` dan final newline.

Workflow yang benar adalah:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server

gofmt -w \
  internal/domain/identity/password_hasher.go \
  internal/domain/identity/password_hasher_test.go

go mod tidy
```

Setelah format dan tidy, baru jalankan focused test, seluruh test, race test, dan vet. Urutan ini mencegah hasil akhir terdokumentasi sementara source atau module file masih belum canonical.

Pada mesin verifikasi, environment sempat menunjuk `GOROOT` 1.25.1 sementara binary Go aktif 1.25.4. Perintah aktual menghapus override tersebut dan memakai cache writable terpisah:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server

env -u GOROOT \
  GOCACHE=/private/tmp/bbge-step39-go-cache \
  /opt/homebrew/bin/go test ./internal/domain/identity \
  -run '^(TestNewArgon2IDPasswordHasher|TestArgon2IDPasswordHasherHashAndVerify|TestParseEncodedPasswordHash|TestArgon2IDPasswordHasherMalformedHashUsesDummyWork|TestArgon2IDPasswordHasherCapacityAndCancellation|TestArgon2IDPasswordHasherNilReceiver)$' \
  -count=1 \
  -v

env -u GOROOT \
  GOCACHE=/private/tmp/bbge-step39-go-cache \
  /opt/homebrew/bin/go test ./... -count=1

env -u GOROOT \
  GOCACHE=/private/tmp/bbge-step39-go-cache \
  /opt/homebrew/bin/go test -race \
  ./internal/domain/identity -count=1

env -u GOROOT \
  GOCACHE=/private/tmp/bbge-step39-go-cache \
  /opt/homebrew/bin/go vet ./...
```

## Hasil verifikasi aktual

Focused package selesai sukses:

```text
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/domain/identity  0.197s
```

Seluruh package selesai sukses:

```text
ok  github.com/imamsyafii-dev/bbge-practice/srv-server                         0.010s
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/config                  0.006s
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/db                      0.015s
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/app/account    0.013s
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/domain/account 0.013s
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/domain/identity 0.207s
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/router         0.021s
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/store          0.010s
?   github.com/imamsyafii-dev/bbge-practice/srv-server/pkg/postgres            [no test files]
```

Identity race test selesai sukses:

```text
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/domain/identity  2.905s
```

`go vet ./...` selesai tanpa diagnostic. Durasi bersifat observasi dari run tersebut, bukan performance benchmark atau acceptance threshold Argon2.

## Batas dan hal yang sengaja belum dikerjakan

Step 39 tidak mencakup:

- persistence hash ke PostgreSQL;
- registration domain service yang memanggil hasher;
- login consumer dan interface verifier milik login;
- endpoint HTTP dan error mapping;
- distributed/global concurrency limit untuk beberapa process atau replica;
- rate limiting per identity, IP, device, atau tenant;
- password pepper atau integrasi secret manager/HSM;
- upgrade/rehash policy ketika parameter berubah;
- penerimaan beberapa versi parameter lama;
- observability production seperti metrics dan alerting;
- secure memory locking atau jaminan penghapusan semua salinan plaintext;
- proof-of-possession, signature verification, atau encrypted identity backup;
- benchmark dan tuning parameter berdasarkan hardware/deployment practice;
- audit keamanan independen.

Semua test yang lulus hanya membuktikan contract yang diuji pada code dan environment ini. Ia tidak membuktikan bahwa keseluruhan sistem registrasi aman secara production.

## Pitfall umum

1. Menggunakan `math/rand` untuk salt. Salt harus berasal dari CSPRNG seperti `crypto/rand`.
2. Menyimpan plaintext atau membungkus plaintext sebagai `EncodedPasswordHash`. Typed wrapper adalah guardrail, bukan bukti bahwa hashing benar-benar terjadi.
3. Melakukan trim atau lowercase password. Password harus diproses persis sesuai input yang lolos policy.
4. Memakai Base64 padded sementara parser meminta `RawStdEncoding`, atau memakai URL-safe Base64 tanpa mengubah contract.
5. Memercayai parameter `m`, `t`, dan `p` dari stored input tanpa batas. Hal ini dapat membuka resource-exhaustion path. Test dengan parameter hostile yang lebih panjang mengunci length precheck, sedangkan test parameter salah dengan panjang sama diperlukan untuk benar-benar mengunci exact field guard.
6. Menganggap `ConstantTimeCompare` membuat seluruh verification constant-time. Ia hanya mencakup perbandingan byte slice.
7. Return terlalu dini pada malformed hash lalu mengklaim perlindungan enumeration/timing sudah cukup.
8. Mengklaim dummy work membuat timing sempurna identik. Step ini tidak membuktikannya.
9. Menganggap `ctx.Cancel` menghentikan `argon2.IDKey` yang sudah berjalan. Pemeriksaan baru terjadi lagi setelah KDF selesai.
10. Membuat unbounded goroutine/queue di depan KDF. Channel Step 39 sengaja fail-fast ketika penuh.
11. Menyamakan `p=1` dengan kapasitas channel. Keduanya mengontrol jenis concurrency yang berbeda.
12. Mengalikan 19 MiB lalu menyebut hasilnya sebagai batas seluruh process. Itu hanya perkiraan memori KDF aktif.
13. Memperbesar interface `PasswordHasher` dengan `Verify` sebelum consumer registrasi membutuhkannya.
14. Lupa menjalankan `go mod tidy`, sehingga dependency langsung masih tercatat indirect atau module graph belum konsisten.
15. Mengedit listing lesson tanpa mengecek kembali bahwa byte di dalam fenced block masih sama dengan source final.

## Recap

Step 39 mengubah boundary abstrak Step 38 menjadi hasher konkret dengan keputusan yang dapat diuji:

```text
Argon2id v=19
t=2
m=19456 KiB = 19 MiB
p=1
salt=16 byte dari crypto/rand
key=32 byte
raw standard Base64 tanpa padding
strict fixed-format parser
constant-time key comparison
bounded dummy work untuk malformed input
fail-fast per-process concurrency semaphore
context check sebelum dan setelah KDF
```

Kata kuncinya adalah bounded dan explicit. Parameter, bentuk encoding, resource concurrency, serta failure behavior dikunci. Pada saat yang sama, lesson tetap menyatakan keterbatasan timing, cancellation, memory clearing, deployment, dan integration yang belum diselesaikan.

## Preview Step 40

Step 40 beralih dari credential hashing ke identity ownership. Practice akan membuat generator `user_id` sebagai dependency yang dapat di-fake, lalu menguji bahwa server—bukan input registrasi—yang menentukan identity pengguna. Hasher Step 39 tetap menjadi dependency terpisah dan belum dirangkai menjadi registration domain service sampai Step 42.
