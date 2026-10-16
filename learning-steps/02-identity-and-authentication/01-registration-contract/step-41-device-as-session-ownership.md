# Step 41 — Device-as-session ownership

Step ini memodelkan device sebagai pemilik state session. Satu record `Device` menghubungkan account dengan device tertentu dan membawa refresh token, status session, waktu penerbitan, serta waktu kedaluwarsa. Practice mengikuti keputusan inti source aktif `bbge-biz`: tidak membuat entity atau tabel session umum yang terpisah.

Step 41 juga membuat concrete `RandomRefreshTokenGenerator`. Generator membaca 32 byte dari `crypto/rand.Reader`, lalu menghasilkan token 64 karakter lowercase hexadecimal. Interface generator belum dibuat karena consumer sebenarnya baru muncul pada registration domain service di Step 42.

## Tujuan dan scope

Setelah Step 41, practice mempunyai:

- typed `DeviceFlag` dengan nilai canonical app `0`, web `1`, PC `2`, dan system `99`;
- validasi bahwa registration input hanya menerima device flag canonical;
- typed `DeviceSessionStatus` yang aman untuk domain practice;
- opaque wrapper `RefreshToken` dengan constructor internal;
- entity `Device` yang menyimpan ownership `(UserID, DeviceID)` dan state session;
- constructor internal `newActiveDevice` yang menjaga invariant active device;
- aturan waktu session `[IssuedAt, ExpiredAt)`;
- normalisasi waktu ke UTC dan penghapusan monotonic component memakai `Round(0)`;
- concrete refresh-token generator dengan private `io.Reader` test seam;
- 32 random byte atau 256 random bit yang di-encode menjadi 64 lowercase hexadecimal;
- test deterministic untuk entity, ownership, status, interval waktu, randomness, partial read, error, configuration, dan cancellation;
- hanya standard library, tanpa dependency module baru.

Step ini belum membuat registration service, interface `RefreshTokenGenerator`, persistence, migration practice, token digest, rotation, revocation use case, HTTP endpoint, access token, atau integration dengan WuKongIM.

## Hubungan dengan Step 40–42

Urutan desainnya adalah:

```text
Step 40  server membuat opaque user_id
    ↓
Step 41  device dimiliki user_id
         + record device membawa state session
         + server membuat opaque refresh token
    ↓
Step 42  registration domain service
         + consumer-owned UserIDGenerator interface
         + consumer-owned RefreshTokenGenerator interface
         + orchestration account, credential, dan device
```

Step 40 hanya menyediakan concrete account identity generator. Step 41 menyediakan bentuk device dan concrete token generator yang dibutuhkan untuk membentuk active device. Step 42 baru menjadi consumer keduanya; karena itu interface generator dan fake dependency dibuat di Step 42, bukan lebih awal di package implementation.

## Bukti production yang dibandingkan

Source production aktif yang relevan ketika step ini ditulis:

```text
bbge-biz/srv-server/db/migrations/0005_create_device.sql
bbge-biz/srv-server/db/migrations/0101_allow_device_id_reuse_across_accounts.sql
bbge-biz/srv-server/db/queries/device.sql
bbge-biz/srv-server/internal/domain/identity/service.go
bbge-biz/srv-server/internal/domain/identity/device_management.go
bbge-biz/proto/proto/bbge/identity/identity.proto
bbge-biz/proto/proto/bbge/types/types.proto
```

### Device sekaligus menyimpan session

Migration `0005_create_device.sql` mempunyai satu tabel `device` yang memuat data device dan state session secara bersamaan, antara lain:

```text
user_id
device_id
device_name
device_model
device_flag
refresh_token
wk_token
session_status
issued_at
expired_at
```

Komentar tabel production juga menyatakan bahwa tabel tersebut membawa device entity dan device-session state. Karena itu practice tidak membuat model `Session` umum yang terpisah. Istilah *device-as-session* berarti state autentikasi melekat pada hubungan account-device; bukan berarti device fisik dan secret token adalah hal yang sama.

### Ownership memakai pasangan `(user_id, device_id)`

Migration awal membuat `device_id` unique secara global. Migration `0101_allow_device_id_reuse_across_accounts.sql` kemudian menggantinya dengan:

```sql
UNIQUE (user_id, device_id)
```

Query aktif membaca dan melakukan upsert berdasarkan pasangan tersebut. Artinya, string `device_id` yang sama boleh muncul untuk owner berbeda, tetapi tidak boleh ambigu di dalam account yang sama. Test practice mengunci bagian domainnya dengan membuat `shared-device-id` untuk dua `UserID` berbeda. Unique constraint sebenarnya baru dibuat pada Step 43–45.

### Session production dibuat server

`service.go` production membuat session dengan waktu UTC, lifetime berdasarkan `common.UserTokenTTL`, dan UUIDv4 server-side. Saat source ini dibandingkan, production mengisi `RefreshToken` dan `WkToken` dengan string UUIDv4 yang sama. Itu adalah keputusan implementasi production saat ini, bukan invariant bahwa dua jenis token harus selalu sama.

Practice memakai 32 random byte menjadi 64 hex dan hanya memodelkan refresh token. `wk_token` sengaja ditunda karena autentikasi WuKongIM belum menjadi scope fase ini. Menghilangkannya dari practice Step 41 tidak berarti production tidak memilikinya.

## Device flag canonical dan perbedaan validasi

`types.proto` production mendefinisikan:

| Nama | Nilai |
|---|---:|
| `DEVICE_FLAG_APP` | 0 |
| `DEVICE_FLAG_WEB` | 1 |
| `DEVICE_FLAG_PC` | 2 |
| `DEVICE_FLAG_SYSTEM` | 99 |

Practice menyalin nilai tersebut ke typed `DeviceFlag`. Karena memakai `iota`, urutannya adalah:

```text
DeviceFlagApp    = 0
DeviceFlagWeb    = 1
DeviceFlagPC     = 2
DeviceFlagSystem = 99
```

Nilai `3` bukan system; assignment explicit `= 99` menggantikan kelanjutan `iota` untuk konstanta itu.

Practice sengaja lebih ketat daripada path registration production yang dibandingkan: `validateRegisterInput` dan `newActiveDevice` memanggil `DeviceFlag.valid()`. Source production aktif meneruskan `int32` tanpa guard canonical yang setara pada jalur tersebut. Ini adalah hardening contract latihan yang disengaja, bukan klaim bahwa production sudah melakukan validasi yang sama.

## Tiga ruang nilai session status

Status database, protobuf, dan domain practice tidak mempunyai angka yang sama:

| Makna | Database production | Protobuf production | Domain practice |
|---|---:|---:|---:|
| unspecified/unknown | tidak tersedia | `0` | `0` |
| active | `0` | `1` | `1` |
| invalid | `1` | `2` | `2` |
| logged out | `2` menurut komentar migration/protobuf mapping | `3` | `3` |

Database production memakai `0` untuk active, sedangkan protobuf memakai `0` untuk unspecified. `device_management.go` melakukan mapping explicit antara nilai database dan protobuf.

Practice memilih nilai domain `0 unknown`, `1 active`, `2 invalid`, `3 logged out` agar zero value tidak berarti sesi aktif. Konsekuensinya sangat penting: persistence Step 43–45 tidak boleh melakukan cast angka langsung. Adapter persistence harus memetakan nilai domain dan database secara explicit.

## Refresh token sebagai opaque value

`RefreshToken` menyimpan field private:

```go
type RefreshToken struct {
	value string
}
```

Caller package lain tidak dapat membentuk nilai valid dengan struct literal field tersebut. `newRefreshToken` menolak string kosong dan whitespace di awal atau akhir. Method `Value()` menjadi jalan explicit untuk mengambil nilai ketika benar-benar diperlukan.

Method berikut:

```go
func (token RefreshToken) Value() string
```

mempunyai receiver value bernama `token`. Receiver bukan parameter bisnis tambahan dari caller; receiver adalah instance pemilik method. Bentuk fungsi biasa sebenarnya mungkin:

```go
func refreshTokenValue(token RefreshToken) string
```

Namun method membuat perilaku ditemukan melalui `token.Value()`, dikelompokkan bersama tipenya, dan dapat memenuhi interface bila kelak diperlukan.

`RefreshToken` tidak mengimplementasikan `String()` atau `fmt.Stringer`, supaya formatting umum tidak secara sengaja dibuat mudah menampilkan secret. Meski demikian, `Value()` tetap mengembalikan bearer secret. Caller tidak boleh memasukkannya ke log, error, metrics label, tracing attribute, atau response yang tidak semestinya.

Test memakai token fixture seperti `practice-refresh-token` dan deterministic hex. Output kegagalan test boleh menampilkan fixture tersebut, tetapi pola itu tidak boleh dipakai untuk token nyata. Test invalid-device juga sengaja tidak mem-format seluruh `Device`, karena recursive formatting dapat mengekspos nested token jika terjadi bug.

## Device ownership dan constructor

`Device` membawa:

```text
identity: UserID + DeviceID
metadata: DeviceName + DeviceModel + DeviceFlag
session:  RefreshToken + SessionStatus + IssuedAt + ExpiredAt
```

`newActiveDevice` menjaga invariant berikut:

- owner setelah `TrimSpace` tidak kosong;
- device ID setelah `TrimSpace` tidak kosong;
- device flag canonical;
- refresh token bukan zero value;
- issued-at bukan zero time;
- expired-at harus benar-benar sesudah issued-at;
- string owner/device/metadata disimpan dalam bentuk trimmed;
- waktu disimpan sebagai UTC tanpa monotonic clock reading;
- status selalu dipaksa menjadi active, bukan diterima dari input caller.

`activeDeviceInput` tidak mempunyai `SessionStatus`. Dengan begitu caller tidak dapat meminta constructor active menghasilkan status invalid atau logged out.

Ada batas encapsulation yang disengaja untuk tahap belajar: `Device` diekspor dan seluruh field-nya exported, sehingga package lain secara teknis masih dapat membuat `Device{}` atau struct literal invalid tanpa constructor. Step ini mengandalkan convention bahwa entity baru dibentuk melalui domain operation. Jika invariant harus dipaksa lintas package secara penuh, desain berikutnya dapat memakai field private dan accessor, tetapi perubahan itu belum dilakukan sekarang.

## Interval aktif `[issued_at, expired_at)`

`SessionActiveAt(now)` memakai interval setengah terbuka:

```text
IssuedAt <= now < ExpiredAt
```

Konsekuensinya:

- satu nanosecond sebelum issuance: tidak aktif;
- tepat pada issuance: aktif;
- di antara issuance dan expiry: aktif;
- tepat pada expiry: tidak aktif;
- setelah expiry: tidak aktif.

Status juga harus `DeviceSessionStatusActive`, dan kedua waktu harus non-zero. Model setengah terbuka menghindari overlap di boundary saat token lama berakhir dan token baru mulai.

Method menerima `now` dari caller, bukan memanggil `time.Now()` sendiri. Injeksi waktu ini membuat test deterministic dan kelak memungkinkan service memakai satu clock/dependency yang konsisten.

## UTC dan `Round(0)`

Constructor dan `SessionActiveAt` menjalankan:

```go
value.Round(0).UTC()
```

`UTC()` menormalkan location agar persistence serta comparison tidak bergantung pada zona lokal. `Round(0)` menghapus monotonic clock reading yang dapat ikut melekat pada `time.Time` hasil runtime. Instant waktunya tidak berubah; representasinya dibuat lebih stabil saat melintasi boundary seperti database, serialization, atau equality test.

Normalisasi bukan pengganti validasi. Constructor tetap memastikan issued-at tersedia dan expiry benar-benar sesudah issuance.

## Generator: 32 byte menjadi 64 lowercase hex

Konstanta generator adalah:

```go
const refreshTokenRandomByteLength = 32
```

Perhitungannya:

```text
32 byte × 8 bit  = 256 random bit
32 byte × 2 hex  = 64 hexadecimal character
```

`hex.EncodeToString` menghasilkan alfabet lowercase `0-9a-f`. Token tidak mempunyai prefix, separator, timestamp, user ID, device ID, atau field yang dapat diinterpretasikan. Ia diperlakukan opaque.

Production aktif memakai UUIDv4 sebagai refresh token. Practice sengaja memakai random payload lebih panjang untuk latihan generator standard-library. Keduanya sama-sama dibuat server-side, tetapi formatnya berbeda dan tidak boleh dianggap interoperable.

## `io.Reader`, function adapter, dan `io.ReadFull`

Concrete generator menyimpan:

```go
random io.Reader
```

Constructor production memasang `crypto/rand.Reader`. Field tetap private supaya caller biasa tidak mudah mengganti entropy source. Test dalam package yang sama dapat memasang deterministic reader atau reader yang gagal.

`io.Reader.Read` boleh mengembalikan data lebih sedikit daripada ukuran buffer tanpa berarti seluruh operasi gagal. Karena contract generator memerlukan tepat 32 byte, implementation memakai `io.ReadFull`.

Test `TestRandomRefreshTokenGeneratorGenerate` membungkus `bytes.Reader` dengan `iotest.OneByteReader`. Reader itu hanya memberi satu byte setiap `Read`. Hasil tetap lengkap membuktikan generator tidak berasumsi satu pemanggilan `Read` langsung mengisi seluruh buffer.

Helper test:

```go
type refreshTokenReaderFunc func(buffer []byte) (int, error)
```

adalah named function type. Method `Read` membuat nilai function tersebut memenuhi interface `io.Reader`. Karena itu test dapat menulis inline callback untuk failure atau cancellation tanpa membuat banyak fake struct terpisah.

## Context cancellation dan batasnya

Generator memeriksa `ctx.Err()`:

1. sebelum random read, agar request yang sudah dibatalkan tidak melakukan pekerjaan baru;
2. setelah random read, agar hasil dibuang bila cancellation terjadi selama callback/read.

Pemeriksaan ini tidak dapat menghentikan arbitrary `io.Reader` yang sudah blocked. Interface `io.Reader` tidak menerima context. Nama test `during random read` berarti callback membatalkan context sebelum return; generator baru melihat cancellation setelah synchronous `ReadFull` selesai.

`crypto/rand.Reader` normalnya cocok untuk operasi kecil ini, tetapi Step 41 tidak menjanjikan hard interruption terhadap reader custom yang tidak pernah return.

## Best-effort pembersihan byte

Generator memakai:

```go
defer clear(randomBytes[:])
```

Ini membersihkan buffer random lokal ketika method selesai. Sifatnya best-effort:

- encoded token sudah menjadi immutable Go string;
- runtime/compiler dapat membuat salinan sementara;
- garbage collector dan OS memory behavior tidak dijamin oleh `clear`;
- `Value()` dapat membuat secret diteruskan ke komponen lain.

Jadi `clear` memperkecil lifetime satu buffer yang kita kontrol, tetapi bukan jaminan secure erase menyeluruh.

## Penyimpanan token pada step berikutnya

Entity Step 41 masih membawa raw token karena service perlu mengembalikannya kepada client pada issuance. Persistence yang aman sebaiknya menyimpan representasi verifikasi yang tidak langsung menjadi bearer credential—misalnya digest/token hash—bila query dan rotation design memungkinkan.

Roadmap Step 45 dan Step 59 harus membedakan:

```text
raw refresh token     -> hanya dikirim kepada pemilik yang berhak
safe representation  -> disimpan dan dicocokkan di server
```

Source production aktif saat ini menyimpan raw `refresh_token` dan mempunyai index atasnya. Rekomendasi digest adalah hardening practice masa depan, bukan deskripsi perilaku production saat ini. Detail skema, lookup, constant-time comparison, rotation, dan migration baru diputuskan pada step persistence/refresh.

## File yang dibuat dan diubah

Dibuat:

```text
srv-server/internal/domain/identity/device.go
srv-server/internal/domain/identity/device_test.go
srv-server/internal/domain/identity/refresh_token_generator.go
srv-server/internal/domain/identity/refresh_token_generator_test.go
```

Diubah:

```text
srv-server/internal/domain/identity/registration.go
srv-server/internal/domain/identity/registration_test.go
```

Perubahan registration mengganti `DeviceFlag int32` menjadi typed `DeviceFlag`, lalu menolak nilai yang tidak canonical.

## Isi final `device.go`

```go
package identity

import (
	"errors"
	"strings"
	"time"
)

type DeviceFlag int32

const (
	DeviceFlagApp DeviceFlag = iota
	DeviceFlagWeb
	DeviceFlagPC

	DeviceFlagSystem DeviceFlag = 99
)

type DeviceSessionStatus int16

const (
	DeviceSessionStatusUnknown DeviceSessionStatus = iota
	DeviceSessionStatusActive
	DeviceSessionStatusInvalid
	DeviceSessionStatusLoggedOut
)

var (
	ErrDeviceOwnerRequired = errors.New(
		"device owner is required",
	)
	ErrDeviceFlagInvalid = errors.New(
		"device flag is invalid",
	)
	ErrRefreshTokenInvalid = errors.New(
		"refresh token is invalid",
	)
	ErrDeviceSessionIssuedAtRequired = errors.New(
		"device session issued at is required",
	)
	ErrDeviceSessionExpirationInvalid = errors.New(
		"device session expiration is invalid",
	)
)

type RefreshToken struct {
	value string
}

func newRefreshToken(
	value string,
) (RefreshToken, error) {
	if value == "" ||
		strings.TrimSpace(value) != value {
		return RefreshToken{},
			ErrRefreshTokenInvalid
	}

	return RefreshToken{
		value: value,
	}, nil
}

func (token RefreshToken) Value() string {
	return token.value
}

type Device struct {
	UserID        string
	DeviceID      string
	DeviceName    string
	DeviceModel   string
	DeviceFlag    DeviceFlag
	RefreshToken  RefreshToken
	SessionStatus DeviceSessionStatus
	IssuedAt      time.Time
	ExpiredAt     time.Time
}

type activeDeviceInput struct {
	UserID       string
	DeviceID     string
	DeviceName   string
	DeviceModel  string
	DeviceFlag   DeviceFlag
	RefreshToken RefreshToken
	IssuedAt     time.Time
	ExpiredAt    time.Time
}

func newActiveDevice(
	input activeDeviceInput,
) (Device, error) {
	userID := strings.TrimSpace(input.UserID)
	if userID == "" {
		return Device{},
			ErrDeviceOwnerRequired
	}

	deviceID := strings.TrimSpace(
		input.DeviceID,
	)
	if deviceID == "" {
		return Device{},
			ErrDeviceIDRequired
	}

	if !input.DeviceFlag.valid() {
		return Device{},
			ErrDeviceFlagInvalid
	}

	if input.RefreshToken.Value() == "" {
		return Device{},
			ErrRefreshTokenInvalid
	}

	if input.IssuedAt.IsZero() {
		return Device{},
			ErrDeviceSessionIssuedAtRequired
	}

	issuedAt := input.IssuedAt.
		Round(0).
		UTC()
	expiredAt := input.ExpiredAt.
		Round(0).
		UTC()

	if !expiredAt.After(issuedAt) {
		return Device{},
			ErrDeviceSessionExpirationInvalid
	}

	return Device{
		UserID:   userID,
		DeviceID: deviceID,
		DeviceName: strings.TrimSpace(
			input.DeviceName,
		),
		DeviceModel: strings.TrimSpace(
			input.DeviceModel,
		),
		DeviceFlag:    input.DeviceFlag,
		RefreshToken:  input.RefreshToken,
		SessionStatus: DeviceSessionStatusActive,
		IssuedAt:      issuedAt,
		ExpiredAt:     expiredAt,
	}, nil
}

func (
	device Device,
) SessionActiveAt(
	now time.Time,
) bool {
	if device.SessionStatus !=
		DeviceSessionStatusActive ||
		device.IssuedAt.IsZero() ||
		device.ExpiredAt.IsZero() {
		return false
	}

	now = now.
		Round(0).
		UTC()

	return !now.Before(device.IssuedAt) &&
		now.Before(device.ExpiredAt)
}

func (
	flag DeviceFlag,
) valid() bool {
	switch flag {
	case DeviceFlagApp,
		DeviceFlagWeb,
		DeviceFlagPC,
		DeviceFlagSystem:
		return true
	default:
		return false
	}
}
```

## Isi final `device_test.go`

```go
package identity

import (
	"errors"
	"testing"
	"time"
)

func TestDeviceFlagValid(
	t *testing.T,
) {
	tests := []struct {
		name string
		flag DeviceFlag
		want bool
	}{
		{
			name: "app",
			flag: DeviceFlagApp,
			want: true,
		},
		{
			name: "web",
			flag: DeviceFlagWeb,
			want: true,
		},
		{
			name: "PC",
			flag: DeviceFlagPC,
			want: true,
		},
		{
			name: "system",
			flag: DeviceFlagSystem,
			want: true,
		},
		{
			name: "negative",
			flag: DeviceFlag(-1),
		},
		{
			name: "unknown sequential value",
			flag: DeviceFlag(3),
		},
		{
			name: "unknown large value",
			flag: DeviceFlag(100),
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				got := test.flag.valid()

				if got != test.want {
					t.Errorf(
						"DeviceFlag.valid() = %v, want %v",
						got,
						test.want,
					)
				}
			},
		)
	}
}

func TestDeviceSessionStatusDomainValues(
	t *testing.T,
) {
	tests := []struct {
		name   string
		status DeviceSessionStatus
		want   int16
	}{
		{
			name:   "unknown",
			status: DeviceSessionStatusUnknown,
			want:   0,
		},
		{
			name:   "active",
			status: DeviceSessionStatusActive,
			want:   1,
		},
		{
			name:   "invalid",
			status: DeviceSessionStatusInvalid,
			want:   2,
		},
		{
			name:   "logged out",
			status: DeviceSessionStatusLoggedOut,
			want:   3,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				got := int16(test.status)

				if got != test.want {
					t.Errorf(
						"status value = %d, want %d",
						got,
						test.want,
					)
				}
			},
		)
	}
}

func TestNewRefreshToken(
	t *testing.T,
) {
	token, err := newRefreshToken(
		"practice-refresh-token",
	)
	if err != nil {
		t.Fatalf(
			"newRefreshToken() error = %v, want nil",
			err,
		)
	}

	if token.Value() !=
		"practice-refresh-token" {
		t.Errorf(
			"RefreshToken.Value() = %q, want %q",
			token.Value(),
			"practice-refresh-token",
		)
	}

	invalidValues := []string{
		"",
		" ",
		" practice-refresh-token",
		"practice-refresh-token ",
		"\tpractice-refresh-token",
	}

	for _, value := range invalidValues {
		t.Run(
			value,
			func(t *testing.T) {
				_, err := newRefreshToken(value)

				if !errors.Is(
					err,
					ErrRefreshTokenInvalid,
				) {
					t.Fatalf(
						"newRefreshToken(%q) error = %v, want %v",
						value,
						err,
						ErrRefreshTokenInvalid,
					)
				}
			},
		)
	}
}

func TestNewActiveDevice(
	t *testing.T,
) {
	location := time.FixedZone(
		"practice-zone",
		7*60*60,
	)
	issuedAt := time.Date(
		2026,
		time.October,
		16,
		15,
		30,
		0,
		123456789,
		location,
	)
	expiredAt := issuedAt.Add(
		30 * 24 * time.Hour,
	)
	token := mustRefreshToken(
		t,
		"practice-refresh-token",
	)

	device, err := newActiveDevice(
		activeDeviceInput{
			UserID:       " practice-user-001 ",
			DeviceID:     " practice-device-001 ",
			DeviceName:   " Practice Phone ",
			DeviceModel:  " Practice Model ",
			DeviceFlag:   DeviceFlagWeb,
			RefreshToken: token,
			IssuedAt:     issuedAt,
			ExpiredAt:    expiredAt,
		},
	)
	if err != nil {
		t.Fatalf(
			"newActiveDevice() error = %v, want nil",
			err,
		)
	}

	if device.UserID != "practice-user-001" {
		t.Errorf(
			"Device.UserID = %q, want %q",
			device.UserID,
			"practice-user-001",
		)
	}

	if device.DeviceID !=
		"practice-device-001" {
		t.Errorf(
			"Device.DeviceID = %q, want %q",
			device.DeviceID,
			"practice-device-001",
		)
	}

	if device.DeviceName != "Practice Phone" {
		t.Errorf(
			"Device.DeviceName = %q, want %q",
			device.DeviceName,
			"Practice Phone",
		)
	}

	if device.DeviceModel != "Practice Model" {
		t.Errorf(
			"Device.DeviceModel = %q, want %q",
			device.DeviceModel,
			"Practice Model",
		)
	}

	if device.DeviceFlag != DeviceFlagWeb {
		t.Errorf(
			"Device.DeviceFlag = %d, want %d",
			device.DeviceFlag,
			DeviceFlagWeb,
		)
	}

	if device.RefreshToken.Value() !=
		token.Value() {
		t.Errorf(
			"Device.RefreshToken = %q, want %q",
			device.RefreshToken.Value(),
			token.Value(),
		)
	}

	if device.SessionStatus !=
		DeviceSessionStatusActive {
		t.Errorf(
			"Device.SessionStatus = %d, want %d",
			device.SessionStatus,
			DeviceSessionStatusActive,
		)
	}

	if !device.IssuedAt.Equal(
		issuedAt,
	) {
		t.Errorf(
			"Device.IssuedAt = %v, want instant %v",
			device.IssuedAt,
			issuedAt,
		)
	}

	if !device.ExpiredAt.Equal(
		expiredAt,
	) {
		t.Errorf(
			"Device.ExpiredAt = %v, want instant %v",
			device.ExpiredAt,
			expiredAt,
		)
	}

	if device.IssuedAt.Location() != time.UTC ||
		device.ExpiredAt.Location() != time.UTC {
		t.Errorf(
			"device times are not normalized to UTC",
		)
	}
}

func TestNewActiveDeviceRejectsInvalidInput(
	t *testing.T,
) {
	issuedAt := time.Date(
		2026,
		time.October,
		16,
		8,
		0,
		0,
		0,
		time.UTC,
	)

	validInput := activeDeviceInput{
		UserID:      "practice-user-001",
		DeviceID:    "practice-device-001",
		DeviceName:  "Practice Phone",
		DeviceModel: "Practice Model",
		DeviceFlag:  DeviceFlagApp,
		RefreshToken: mustRefreshToken(
			t,
			"practice-refresh-token",
		),
		IssuedAt: issuedAt,
		ExpiredAt: issuedAt.Add(
			30 * 24 * time.Hour,
		),
	}

	tests := []struct {
		name    string
		change  func(*activeDeviceInput)
		wantErr error
	}{
		{
			name: "owner is missing",
			change: func(
				input *activeDeviceInput,
			) {
				input.UserID = " "
			},
			wantErr: ErrDeviceOwnerRequired,
		},
		{
			name: "device ID is missing",
			change: func(
				input *activeDeviceInput,
			) {
				input.DeviceID = "\t"
			},
			wantErr: ErrDeviceIDRequired,
		},
		{
			name: "device flag is invalid",
			change: func(
				input *activeDeviceInput,
			) {
				input.DeviceFlag =
					DeviceFlag(3)
			},
			wantErr: ErrDeviceFlagInvalid,
		},
		{
			name: "refresh token is missing",
			change: func(
				input *activeDeviceInput,
			) {
				input.RefreshToken =
					RefreshToken{}
			},
			wantErr: ErrRefreshTokenInvalid,
		},
		{
			name: "issued at is missing",
			change: func(
				input *activeDeviceInput,
			) {
				input.IssuedAt = time.Time{}
			},
			wantErr: ErrDeviceSessionIssuedAtRequired,
		},
		{
			name: "expired at is missing",
			change: func(
				input *activeDeviceInput,
			) {
				input.ExpiredAt = time.Time{}
			},
			wantErr: ErrDeviceSessionExpirationInvalid,
		},
		{
			name: "expiration equals issuance",
			change: func(
				input *activeDeviceInput,
			) {
				input.ExpiredAt =
					input.IssuedAt
			},
			wantErr: ErrDeviceSessionExpirationInvalid,
		},
		{
			name: "expiration precedes issuance",
			change: func(
				input *activeDeviceInput,
			) {
				input.ExpiredAt =
					input.IssuedAt.Add(
						-time.Second,
					)
			},
			wantErr: ErrDeviceSessionExpirationInvalid,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				input := validInput
				test.change(&input)

				device, err :=
					newActiveDevice(input)

				if !errors.Is(
					err,
					test.wantErr,
				) {
					t.Fatalf(
						"newActiveDevice() error = %v, want %v",
						err,
						test.wantErr,
					)
				}

				if device != (Device{}) {
					t.Error(
						"newActiveDevice() returned non-zero device for invalid input",
					)
				}
			},
		)
	}
}

func TestDeviceSessionActiveAt(
	t *testing.T,
) {
	issuedAt := time.Date(
		2026,
		time.October,
		16,
		8,
		0,
		0,
		0,
		time.UTC,
	)
	expiredAt := issuedAt.Add(time.Hour)

	baseDevice := Device{
		SessionStatus: DeviceSessionStatusActive,
		IssuedAt:      issuedAt,
		ExpiredAt:     expiredAt,
	}

	tests := []struct {
		name   string
		status DeviceSessionStatus
		now    time.Time
		want   bool
	}{
		{
			name:   "before issuance",
			status: DeviceSessionStatusActive,
			now:    issuedAt.Add(-time.Nanosecond),
		},
		{
			name:   "exact issuance",
			status: DeviceSessionStatusActive,
			now:    issuedAt,
			want:   true,
		},
		{
			name:   "between issuance and expiry",
			status: DeviceSessionStatusActive,
			now:    issuedAt.Add(30 * time.Minute),
			want:   true,
		},
		{
			name:   "exact expiry",
			status: DeviceSessionStatusActive,
			now:    expiredAt,
		},
		{
			name:   "after expiry",
			status: DeviceSessionStatusActive,
			now:    expiredAt.Add(time.Nanosecond),
		},
		{
			name:   "unknown status",
			status: DeviceSessionStatusUnknown,
			now:    issuedAt,
		},
		{
			name:   "invalid status",
			status: DeviceSessionStatusInvalid,
			now:    issuedAt,
		},
		{
			name:   "logged out status",
			status: DeviceSessionStatusLoggedOut,
			now:    issuedAt,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				device := baseDevice
				device.SessionStatus =
					test.status

				got := device.SessionActiveAt(
					test.now,
				)

				if got != test.want {
					t.Errorf(
						"SessionActiveAt() = %v, want %v",
						got,
						test.want,
					)
				}
			},
		)
	}
}

func TestNewActiveDeviceAllowsDeviceIDAcrossOwners(
	t *testing.T,
) {
	issuedAt := time.Date(
		2026,
		time.October,
		16,
		8,
		0,
		0,
		0,
		time.UTC,
	)
	token := mustRefreshToken(
		t,
		"practice-refresh-token",
	)

	for _, userID := range []string{
		"practice-user-001",
		"practice-user-002",
	} {
		device, err := newActiveDevice(
			activeDeviceInput{
				UserID:       userID,
				DeviceID:     "shared-device-id",
				DeviceFlag:   DeviceFlagApp,
				RefreshToken: token,
				IssuedAt:     issuedAt,
				ExpiredAt: issuedAt.Add(
					time.Hour,
				),
			},
		)
		if err != nil {
			t.Fatalf(
				"newActiveDevice(%q) error = %v, want nil",
				userID,
				err,
			)
		}

		if device.UserID != userID ||
			device.DeviceID !=
				"shared-device-id" {
			t.Errorf(
				"device ownership = %q/%q, want %q/%q",
				device.UserID,
				device.DeviceID,
				userID,
				"shared-device-id",
			)
		}
	}
}

func mustRefreshToken(
	t *testing.T,
	value string,
) RefreshToken {
	t.Helper()

	token, err := newRefreshToken(value)
	if err != nil {
		t.Fatalf(
			"newRefreshToken(%q) error = %v",
			value,
			err,
		)
	}

	return token
}
```

## Isi final `refresh_token_generator.go`

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

const refreshTokenRandomByteLength = 32

var ErrRefreshTokenGeneratorNotConfigured = errors.New(
	"refresh token generator is not configured",
)

type RandomRefreshTokenGenerator struct {
	random io.Reader
}

func NewRandomRefreshTokenGenerator() *RandomRefreshTokenGenerator {
	return &RandomRefreshTokenGenerator{
		random: rand.Reader,
	}
}

func (
	generator *RandomRefreshTokenGenerator,
) Generate(
	ctx context.Context,
) (RefreshToken, error) {
	if generator == nil ||
		generator.random == nil {
		return RefreshToken{},
			ErrRefreshTokenGeneratorNotConfigured
	}

	if err := ctx.Err(); err != nil {
		return RefreshToken{}, err
	}

	var randomBytes [refreshTokenRandomByteLength]byte
	defer clear(randomBytes[:])

	if _, err := io.ReadFull(
		generator.random,
		randomBytes[:],
	); err != nil {
		return RefreshToken{},
			fmt.Errorf(
				"generate refresh token: %w",
				err,
			)
	}

	if err := ctx.Err(); err != nil {
		return RefreshToken{}, err
	}

	encoded := hex.EncodeToString(
		randomBytes[:],
	)

	return newRefreshToken(encoded)
}
```

## Isi final `refresh_token_generator_test.go`

```go
package identity

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"testing/iotest"
)

var errRefreshTokenRandomRead = errors.New(
	"practice refresh token random reader failed",
)

type refreshTokenReaderFunc func(
	buffer []byte,
) (int, error)

func (
	function refreshTokenReaderFunc,
) Read(
	buffer []byte,
) (int, error) {
	return function(buffer)
}

func TestNewRandomRefreshTokenGenerator(
	t *testing.T,
) {
	generator :=
		NewRandomRefreshTokenGenerator()

	if generator == nil {
		t.Fatal(
			"NewRandomRefreshTokenGenerator() returned nil",
		)
	}

	if generator.random == nil {
		t.Fatal(
			"NewRandomRefreshTokenGenerator() did not configure random reader",
		)
	}
}

func TestRandomRefreshTokenGeneratorGenerate(
	t *testing.T,
) {
	randomInput := make(
		[]byte,
		refreshTokenRandomByteLength,
	)
	for index := range randomInput {
		randomInput[index] = byte(index)
	}

	generator := &RandomRefreshTokenGenerator{
		random: iotest.OneByteReader(
			bytes.NewReader(randomInput),
		),
	}

	token, err := generator.Generate(
		context.Background(),
	)
	if err != nil {
		t.Fatalf(
			"Generate() error = %v, want nil",
			err,
		)
	}

	const wantToken = "000102030405060708090a0b0c0d0e0f" +
		"101112131415161718191a1b1c1d1e1f"

	if token.Value() != wantToken {
		t.Errorf(
			"Generate() = %q, want %q",
			token.Value(),
			wantToken,
		)
	}
}

func TestRandomRefreshTokenGeneratorUsesFreshRandomness(
	t *testing.T,
) {
	randomInput := append(
		bytes.Repeat(
			[]byte{0x11},
			refreshTokenRandomByteLength,
		),
		bytes.Repeat(
			[]byte{0x22},
			refreshTokenRandomByteLength,
		)...,
	)

	generator := &RandomRefreshTokenGenerator{
		random: bytes.NewReader(randomInput),
	}

	firstToken, err := generator.Generate(
		context.Background(),
	)
	if err != nil {
		t.Fatalf(
			"first Generate() error = %v, want nil",
			err,
		)
	}

	secondToken, err := generator.Generate(
		context.Background(),
	)
	if err != nil {
		t.Fatalf(
			"second Generate() error = %v, want nil",
			err,
		)
	}

	wantFirstToken := strings.Repeat(
		"11",
		refreshTokenRandomByteLength,
	)
	wantSecondToken := strings.Repeat(
		"22",
		refreshTokenRandomByteLength,
	)

	if firstToken.Value() != wantFirstToken {
		t.Errorf(
			"first Generate() = %q, want %q",
			firstToken.Value(),
			wantFirstToken,
		)
	}

	if secondToken.Value() != wantSecondToken {
		t.Errorf(
			"second Generate() = %q, want %q",
			secondToken.Value(),
			wantSecondToken,
		)
	}

	if firstToken.Value() ==
		secondToken.Value() {
		t.Fatal(
			"successive Generate() calls returned the same token",
		)
	}
}

func TestRandomRefreshTokenGeneratorRejectsMissingConfiguration(
	t *testing.T,
) {
	tests := []struct {
		name      string
		generator *RandomRefreshTokenGenerator
	}{
		{
			name:      "nil receiver",
			generator: nil,
		},
		{
			name:      "nil random reader",
			generator: &RandomRefreshTokenGenerator{},
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				token, err :=
					test.generator.Generate(
						context.Background(),
					)

				if token.Value() != "" {
					t.Errorf(
						"Generate() token = %q, want empty",
						token.Value(),
					)
				}

				if !errors.Is(
					err,
					ErrRefreshTokenGeneratorNotConfigured,
				) {
					t.Fatalf(
						"Generate() error = %v, want %v",
						err,
						ErrRefreshTokenGeneratorNotConfigured,
					)
				}
			},
		)
	}
}

func TestRandomRefreshTokenGeneratorForwardsReaderError(
	t *testing.T,
) {
	generator := &RandomRefreshTokenGenerator{
		random: refreshTokenReaderFunc(
			func(
				_ []byte,
			) (int, error) {
				return 0,
					errRefreshTokenRandomRead
			},
		),
	}

	token, err := generator.Generate(
		context.Background(),
	)

	if token.Value() != "" {
		t.Errorf(
			"Generate() token = %q, want empty",
			token.Value(),
		)
	}

	if !errors.Is(
		err,
		errRefreshTokenRandomRead,
	) {
		t.Fatalf(
			"Generate() error = %v, want wrapped %v",
			err,
			errRefreshTokenRandomRead,
		)
	}
}

func TestRandomRefreshTokenGeneratorHonorsCanceledContext(
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
			generator :=
				&RandomRefreshTokenGenerator{
					random: refreshTokenReaderFunc(
						func(
							buffer []byte,
						) (int, error) {
							readerCalled = true

							return len(buffer), nil
						},
					),
				}

			token, err :=
				generator.Generate(ctx)

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

			if token.Value() != "" {
				t.Errorf(
					"Generate() token = %q, want empty",
					token.Value(),
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

			generator :=
				&RandomRefreshTokenGenerator{
					random: refreshTokenReaderFunc(
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

			token, err :=
				generator.Generate(ctx)

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

			if token.Value() != "" {
				t.Errorf(
					"Generate() token = %q, want empty",
					token.Value(),
				)
			}
		},
	)
}
```

## Isi final `registration.go` setelah Step 41

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
	DeviceFlag        DeviceFlag
	IdentifierX25519  []byte
	IdentifierEd25519 []byte
}

func validateRegisterInput(
	input RegisterInput,
) error {
	if strings.TrimSpace(input.DeviceID) == "" {
		return ErrDeviceIDRequired
	}
	if !input.DeviceFlag.valid() {
		return ErrDeviceFlagInvalid
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

## Isi final `registration_test.go` setelah Step 41

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
			name: "device flag is invalid",
			change: func(input *RegisterInput) {
				input.DeviceFlag = DeviceFlag(3)
			},
			wantErr: ErrDeviceFlagInvalid,
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

## Penjelasan test entity device

### `TestDeviceFlagValid`

Mengunci empat nilai canonical sebagai valid dan menolak nilai negatif, sequential `3`, serta nilai besar yang tidak dikenal.

### `TestDeviceSessionStatusDomainValues`

Mengunci angka domain practice `unknown=0`, `active=1`, `invalid=2`, dan `logged-out=3`. Test ini sengaja tidak menyatakan bahwa angka tersebut sama dengan database production.

### `TestNewRefreshToken`

Memastikan constructor menerima fixture non-empty dan `Value()` mengembalikan isi exact. Empty string serta leading/trailing whitespace ditolak. Constructor tidak melakukan trimming diam-diam karena perubahan secret akan menyembunyikan input invalid.

### `TestNewActiveDevice`

Membuktikan trimming metadata, typed flag, ownership, active status, token, instant waktu, dan normalisasi location ke UTC. Waktu fixture memakai zona `UTC+7` serta nanosecond agar test tidak hanya lulus untuk input UTC bulat.

### `TestNewActiveDeviceRejectsInvalidInput`

Menguji owner kosong, device ID kosong, flag invalid, zero refresh token, zero issuance, zero expiry, expiry sama dengan issuance, dan expiry sebelum issuance. Semua failure mengembalikan zero `Device`. Pesan failure tidak mencetak seluruh entity agar nested secret tidak bocor ke output test.

### `TestDeviceSessionActiveAt`

Mengunci interval `[issued, expired)` serta semua status non-active. Test menggunakan waktu yang diinjeksi, bukan wall clock.

### `TestNewActiveDeviceAllowsDeviceIDAcrossOwners`

Membentuk device ID sama untuk dua owner berbeda. Ini membuktikan domain tidak menganggap `device_id` globally unique; constraint pasangan sebenarnya tetap menjadi tanggung jawab persistence.

### `mustRefreshToken`

Helper test memakai `t.Helper()` supaya lokasi kegagalan menunjuk caller. Ia hanya membuat fixture dan bukan API production.

## Penjelasan test generator

### `TestNewRandomRefreshTokenGenerator`

Memastikan constructor non-nil dan memasang random reader.

### `TestRandomRefreshTokenGeneratorGenerate`

Memberikan byte `0x00` sampai `0x1f`, satu byte per `Read`, lalu mengharuskan exact 64 lowercase hex. Ini sekaligus menguji `io.ReadFull` dan deterministic encoding, bukan kualitas entropy runtime.

### `TestRandomRefreshTokenGeneratorUsesFreshRandomness`

Reader berisi dua blok berbeda. Dua call harus mengonsumsi dua blok dan menghasilkan token berbeda. Ini membuktikan generator membaca ulang per call; bukan statistical randomness proof atau collision proof.

### `TestRandomRefreshTokenGeneratorRejectsMissingConfiguration`

Nil receiver dan nil reader harus menghasilkan `ErrRefreshTokenGeneratorNotConfigured` serta empty token, bukan panic.

### `TestRandomRefreshTokenGeneratorForwardsReaderError`

Function adapter mengembalikan sentinel. Generator membungkus error memakai `%w`, sehingga `errors.Is` tetap dapat menemukannya.

### `TestRandomRefreshTokenGeneratorHonorsCanceledContext`

Subtest pertama memastikan reader tidak dipanggil bila context sudah canceled. Subtest kedua membatalkan context dari callback reader dan memastikan hasil random dibuang setelah read selesai.

### Test registration yang berubah

`TestValidateRegisterInput` menambahkan case `DeviceFlag(3)` dan mengharapkan `ErrDeviceFlagInvalid`. Fixture valid masih memakai literal `1`; assignment itu sekarang dikonversi ke typed field dan bermakna web.

## Workflow implementasi dan verifikasi

Format seluruh file Step 41:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server

gofmt -w \
  internal/domain/identity/device.go \
  internal/domain/identity/device_test.go \
  internal/domain/identity/refresh_token_generator.go \
  internal/domain/identity/refresh_token_generator_test.go \
  internal/domain/identity/registration.go \
  internal/domain/identity/registration_test.go
```

Focused test entity:

```bash
env -u GOROOT \
  GOCACHE=/private/tmp/bbge-practice-go-cache \
  /opt/homebrew/bin/go test ./internal/domain/identity \
  -run '^(TestDeviceFlagValid|TestDeviceSessionStatusDomainValues|TestNewRefreshToken|TestNewActiveDevice|TestNewActiveDeviceRejectsInvalidInput|TestDeviceSessionActiveAt|TestNewActiveDeviceAllowsDeviceIDAcrossOwners|TestValidateRegisterInput)$' \
  -count=1 \
  -v
```

Focused test generator:

```bash
env -u GOROOT \
  GOCACHE=/private/tmp/bbge-practice-go-cache \
  /opt/homebrew/bin/go test ./internal/domain/identity \
  -run '^(TestNewRandomRefreshTokenGenerator|TestRandomRefreshTokenGeneratorGenerate|TestRandomRefreshTokenGeneratorUsesFreshRandomness|TestRandomRefreshTokenGeneratorRejectsMissingConfiguration|TestRandomRefreshTokenGeneratorForwardsReaderError|TestRandomRefreshTokenGeneratorHonorsCanceledContext)$' \
  -count=1 \
  -v
```

Verifikasi seluruh module, race detector, vet, dan dependency diff:

```bash
env -u GOROOT \
  GOCACHE=/private/tmp/bbge-practice-go-cache \
  /opt/homebrew/bin/go test ./... -count=1

env -u GOROOT \
  GOCACHE=/private/tmp/bbge-practice-go-race-cache \
  /opt/homebrew/bin/go test -race \
  ./internal/domain/identity -count=1

env -u GOROOT \
  GOCACHE=/private/tmp/bbge-practice-go-cache \
  /opt/homebrew/bin/go vet ./...

env -u GOROOT \
  GOCACHE=/private/tmp/bbge-practice-go-cache \
  /opt/homebrew/bin/go mod tidy -diff
```

Output focused test mempunyai pola:

```text
--- PASS: TestDeviceFlagValid
--- PASS: TestDeviceSessionStatusDomainValues
--- PASS: TestNewRefreshToken
--- PASS: TestNewActiveDevice
--- PASS: TestNewActiveDeviceRejectsInvalidInput
--- PASS: TestDeviceSessionActiveAt
--- PASS: TestNewActiveDeviceAllowsDeviceIDAcrossOwners
--- PASS: TestValidateRegisterInput
--- PASS: TestNewRandomRefreshTokenGenerator
--- PASS: TestRandomRefreshTokenGeneratorGenerate
--- PASS: TestRandomRefreshTokenGeneratorUsesFreshRandomness
--- PASS: TestRandomRefreshTokenGeneratorRejectsMissingConfiguration
--- PASS: TestRandomRefreshTokenGeneratorForwardsReaderError
--- PASS: TestRandomRefreshTokenGeneratorHonorsCanceledContext
PASS
```

Durasi sengaja tidak dikunci dalam dokumentasi karena berbeda antar-run. `go vet` dan `go mod tidy -diff` yang sukses tidak menghasilkan diagnostic/diff.

## Batas dan hal yang sengaja belum dikerjakan

Step 41 tidak mencakup:

- registration domain service;
- interface `RefreshTokenGenerator` atau fake consumer;
- persistence/migration device practice;
- enforcement database untuk unique `(user_id, device_id)`;
- mapping status domain ke nilai database;
- penyimpanan digest refresh token;
- lookup, rotation, reuse detection, atau revocation refresh token;
- access token/JWT;
- WK token dan integrasi WuKongIM;
- device level, push token, last login, last seen, location, atau notification state;
- proof-of-possession device key;
- full encapsulation `Device` dengan field private;
- guaranteed secure erase atas seluruh salinan secret;
- hard cancellation untuk blocked `io.Reader`;
- metrics, audit event, rate limiting, dan distributed uniqueness.

## Pitfall umum

1. Membuat tabel session umum baru walaupun model yang dipelajari menyimpan session pada device.
2. Menganggap `device_id` globally unique; production aktif memakai pasangan `(user_id, device_id)`.
3. Menganggap angka status database, protobuf, dan domain sama lalu melakukan direct cast.
4. Membiarkan zero value status berarti active. Practice sengaja memakai zero sebagai unknown.
5. Menganggap `DeviceFlagSystem` bernilai `3`; nilainya explicit `99`.
6. Menyatakan production sudah memvalidasi flag canonical seperti practice tanpa bukti source.
7. Menerima session status dari caller constructor active.
8. Memakai interval expiry inklusif sehingga token masih dianggap aktif tepat pada `ExpiredAt`.
9. Memanggil `time.Now()` di entity dan membuat test nondeterministic.
10. Menyimpan local timezone/monotonic reading tanpa normalisasi boundary.
11. Memakai `math/rand` atau token pendek/predictable untuk bearer secret.
12. Memanggil `Read` sekali dan menganggap buffer pasti penuh; gunakan `io.ReadFull`.
13. Menganggap context check dapat menghentikan reader yang sedang blocked.
14. Menganggap `clear` menjamin semua salinan string secret terhapus dari memory.
15. Menambahkan `String()` yang mengembalikan raw refresh token lalu tanpa sengaja mencetaknya.
16. Mencetak seluruh `Device` dalam log/test failure dan membocorkan nested token.
17. Menganggap raw token production yang tersimpan berarti practice masa depan juga harus menyimpan raw secret.
18. Menganggap refresh token practice adalah UUIDv4 karena production memakai UUIDv4.
19. Menganggap `wk_token` tidak ada di production karena belum dimodelkan di practice.
20. Membuat interface di package implementation sebelum registration service menjadi consumer.
21. Menganggap exported `Device` mustahil dibuat invalid. Constructor adalah convention pada bentuk sekarang, bukan enforcement total lintas package.

## Recap

Step 41 mengunci model berikut:

```text
owner key      = (user_id, device_id)
device flag    = app 0 | web 1 | PC 2 | system 99
domain status  = unknown 0 | active 1 | invalid 2 | logged-out 3
active window  = [issued_at, expired_at)
time storage   = Round(0) + UTC
refresh token  = opaque bearer secret
generation     = 32 crypto-random bytes -> 64 lowercase hex
read contract  = io.ReadFull
session model  = state lives on Device, no generic Session entity
```

Production mempunyai database status `0/1/2`, protobuf status `0/1/2/3`, UUIDv4 token, dan separate `refresh_token`/`wk_token` columns yang saat ini diisi nilai sama. Practice mengambil invariant ownership dan lifecycle, sambil menjaga mapping serta integration detail tetap explicit untuk step berikutnya.

## Preview Step 42

Step 42 membuat registration domain service yang menjadi consumer dari primitive Step 38–41. Pada saat itu interface kecil didefinisikan sesuai kebutuhan service, lalu fake dependency membuktikan bahwa service:

- memvalidasi request;
- menghasilkan `user_id` di server;
- melakukan password hashing;
- membuat password credential;
- menghasilkan refresh token;
- membentuk active device dengan owner yang sama;
- memakai waktu yang diinjeksi;
- meneruskan dependency error tanpa persistence nyata.

Atomic database transaction, idempotent retry berbasis stored request ID, dan status mapping persistence tetap dimulai pada Step 43–49.
