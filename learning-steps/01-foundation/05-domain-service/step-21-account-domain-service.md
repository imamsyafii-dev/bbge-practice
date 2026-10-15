# Step 21 — Account Domain Service

Pada Step 20, package `internal/store` sudah dapat membuat dan mengambil account melalui PostgreSQL. Namun, generated store hanya mengetahui cara menjalankan SQL. Store belum menjadi tempat yang tepat untuk aturan seperti input wajib diisi, normalisasi spasi, atau penerjemahan error database menjadi error yang dimengerti aplikasi.

Step ini menambahkan domain service account sebagai lapisan di atas generated store. Domain service akan:

1. mendefinisikan dependency store sekecil yang dibutuhkan;
2. memvalidasi dan menormalisasi input;
3. memanggil generated query melalui interface;
4. membungkus error database tanpa menghilangkan penyebabnya;
5. menerjemahkan `pgx.ErrNoRows` menjadi `ErrAccountNotFound`.

Step ini sengaja masih sederhana. Belum ada use case, HTTP handler, transaction multi-query, ataupun unit test dengan fake store.

## Mengapa masuk kelompok `05-domain-service`

Step 17–20 berada di kelompok `04-sqlc-store` karena fokusnya adalah SQL, code generation, dan hubungan store dengan PostgreSQL. Mulai Step 21, fokus berpindah ke perilaku bisnis di atas persistence.

Karena itu, catatan ini ditempatkan di:

```text
learning-steps/01-foundation/05-domain-service/
```

Susunan sederhananya sekarang adalah:

```text
HTTP atau use case yang akan dibuat nanti
                |
                v
      domain/account.Service
                |
                v
        account.Store interface
                |
                v
     generated *store.Queries
                |
                v
             PostgreSQL
```

## Perbandingan dengan BBGE production

Latihan ini mengambil konsep berlapis dari BBGE, tetapi bukan salinan lengkap proses registrasi production.

Pada latihan, `account.Service.Create` hanya mengeksekusi satu `INSERT` ke tabel `account`. Sementara itu, alur `Identity.Register` di BBGE production melakukan beberapa write yang saling berkaitan, antara lain membuat account, data autentikasi, account setting, device/session, dan personal channel. Write tersebut dijalankan dalam transaction agar semuanya berhasil bersama-sama atau dibatalkan bersama-sama.

Jadi, jangan menyimpulkan bahwa seluruh registrasi production cukup diwakili oleh `Create` pada step ini. Tujuan step ini adalah memahami batas domain service terlebih dahulu.

Practice account juga sengaja **tidak memakai email**. Walaupun beberapa bagian atau schema lama di BBGE dapat memiliki email, kontrak account yang sedang dipelajari hanya memakai `user_id` dan `name` sebagai input pembuatan account.

## Membuat package domain account

Dari root server, buat direktori domain account:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server
mkdir -p internal/domain/account
```

Buat file:

```text
internal/domain/account/service.go
```

Isi final file:

```go
package account

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/imamsyafii-dev/bbge-practice/srv-server/internal/store"
	"github.com/jackc/pgx/v5"
)

var (
	ErrStoreNotConfigured = errors.New(
		"account store not configured",
	)
	ErrUserIDRequired = errors.New(
		"user id is required",
	)
	ErrNameRequired = errors.New(
		"name is required",
	)
	ErrAccountNotFound = errors.New(
		"account not found",
	)
)

type Store interface {
	CreateAccount(
		ctx context.Context,
		arg store.CreateAccountParams,
	) (store.Account, error)

	GetAccountByUserID(
		ctx context.Context,
		userID string,
	) (store.Account, error)
}

var _ Store = (*store.Queries)(nil)

type Service struct {
	store Store
}

type CreateInput struct {
	UserID string
	Name   string
}

func NewService(
	accountStore Store,
) *Service {
	return &Service{
		store: accountStore,
	}
}

func (s *Service) Create(
	ctx context.Context,
	input CreateInput,
) (store.Account, error) {
	if s == nil || s.store == nil {
		return store.Account{},
			ErrStoreNotConfigured
	}

	userID := strings.TrimSpace(input.UserID)
	if userID == "" {
		return store.Account{},
			ErrUserIDRequired
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return store.Account{},
			ErrNameRequired
	}

	account, err := s.store.CreateAccount(
		ctx,
		store.CreateAccountParams{
			UserID: userID,
			Name:   name,
		},
	)
	if err != nil {
		return store.Account{}, fmt.Errorf(
			"create account: %w",
			err,
		)
	}

	return account, nil
}

func (s *Service) GetByUserID(
	ctx context.Context,
	userID string,
) (store.Account, error) {
	if s == nil || s.store == nil {
		return store.Account{},
			ErrStoreNotConfigured
	}

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return store.Account{},
			ErrUserIDRequired
	}

	account, err := s.store.GetAccountByUserID(
		ctx,
		userID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.Account{},
				ErrAccountNotFound
		}

		return store.Account{}, fmt.Errorf(
			"get account by user id: %w",
			err,
		)
	}

	return account, nil
}
```

Simpan file dengan satu newline setelah kurung kurawal terakhir.

## Penjelasan setiap import

### `context`

```go
"context"
```

`context.Context` membawa deadline, cancellation, dan nilai request melewati batas fungsi. Service meneruskan context yang diterimanya ke store agar query database ikut berhenti ketika request dibatalkan atau timeout.

### `errors`

```go
"errors"
```

Package ini dipakai untuk dua hal:

- membuat sentinel error dengan `errors.New`;
- memeriksa rantai error dengan `errors.Is`.

### `fmt`

```go
"fmt"
```

`fmt.Errorf` memberi konteks tambahan pada error store, misalnya `create account: ...`, sambil mempertahankan error asal melalui `%w`.

### `strings`

```go
"strings"
```

`strings.TrimSpace` membuang spasi di awal dan akhir input sebelum validasi dan sebelum nilai diteruskan ke store.

### `internal/store`

```go
"github.com/imamsyafii-dev/bbge-practice/srv-server/internal/store"
```

Package generated sqlc ini menyediakan `store.Account`, `store.CreateAccountParams`, dan concrete implementation `*store.Queries`.

### `pgx/v5`

```go
"github.com/jackc/pgx/v5"
```

`pgx.ErrNoRows` adalah error yang dikembalikan query `:one` ketika tidak ada row. Domain service menerjemahkannya menjadi `ErrAccountNotFound`.

## Sentinel error

Empat error berikut adalah sentinel error:

```go
var (
	ErrStoreNotConfigured = errors.New(
		"account store not configured",
	)
	ErrUserIDRequired = errors.New(
		"user id is required",
	)
	ErrNameRequired = errors.New(
		"name is required",
	)
	ErrAccountNotFound = errors.New(
		"account not found",
	)
)
```

Sentinel error adalah nilai error bersama yang dapat dibandingkan dengan `errors.Is`. Lapisan use case atau HTTP yang dibuat nanti tidak perlu bergantung langsung pada pesan error database.

Maknanya:

- `ErrStoreNotConfigured`: service atau dependency store belum tersedia;
- `ErrUserIDRequired`: `user_id` kosong setelah normalisasi;
- `ErrNameRequired`: `name` kosong setelah normalisasi;
- `ErrAccountNotFound`: store tidak menemukan account.

Pada step ini belum ada pemetaan error menjadi status HTTP. Itu akan dilakukan ketika lapisan HTTP sudah dibuat.

## Interface `Store` yang sempit

```go
type Store interface {
	CreateAccount(
		ctx context.Context,
		arg store.CreateAccountParams,
	) (store.Account, error)

	GetAccountByUserID(
		ctx context.Context,
		userID string,
	) (store.Account, error)
}
```

Interface didefinisikan oleh package yang **memakai** dependency, yaitu domain account. Ini sering disebut consumer-owned interface.

sqlc juga menghasilkan `store.Querier`, tetapi interface generated tersebut akan bertambah setiap kali query lain ditambahkan. Jika domain account menggunakan `store.Querier`, fake untuk unit test harus mengimplementasikan semua query generated, termasuk query yang tidak dipakai account service.

`account.Store` hanya meminta dua kemampuan yang benar-benar dibutuhkan:

```text
CreateAccount
GetAccountByUserID
```

Manfaatnya:

- dependency service terlihat jelas;
- perubahan query yang tidak berkaitan tidak memaksa domain ini berubah;
- fake store pada Step 22 cukup membuat dua method tersebut;
- service tidak terikat pada seluruh generated query set.

Interface bukan berarti semua fungsi harus dibuat ulang. Concrete type cukup memiliki method dengan nama dan signature yang sama.

## Interface dipenuhi secara implisit

Go tidak memakai kata kunci seperti `implements`. `*store.Queries` otomatis memenuhi `account.Store` karena sudah memiliki kedua method yang sesuai.

Pemeriksaan ini dibuat eksplisit dengan:

```go
var _ Store = (*store.Queries)(nil)
```

Bagian-bagiannya:

- `_` adalah blank identifier; nilainya tidak disimpan untuk dipakai;
- `Store` adalah interface yang ingin diverifikasi;
- `(*store.Queries)(nil)` adalah ekspresi pointer nil yang memiliki tipe `*store.Queries`.

Compiler memeriksa apakah tipe `*store.Queries` dapat diberikan kepada `Store`. Jika salah satu method hilang atau signature berubah, proses compile gagal. Baris ini tidak membuka database dan tidak membuat instance `store.Queries`; ekspresi nil bertipe tersebut dipakai untuk pemeriksaan tipe.

Ada satu caveat penting tentang typed nil. Contoh berikut menghasilkan interface yang tidak sama dengan `nil`:

```go
var queries *store.Queries
var accountStore Store = queries

fmt.Println(accountStore == nil) // false
```

Interface menyimpan pasangan tipe dan nilai. Walaupun nilai pointer di dalamnya nil, bagian tipenya berisi `*store.Queries`, sehingga interface itu sendiri tidak nil. Pemeriksaan `s.store == nil` pada service hanya menangkap interface nil murni, bukan semua kemungkinan typed nil. Latihan ini belum menambahkan pemeriksaan refleksi untuk typed nil; wiring yang benar tidak boleh memasukkan pointer dependency nil ke dalam interface.

## `Service`, `CreateInput`, dan constructor

```go
type Service struct {
	store Store
}
```

`Service` menyimpan dependency melalui interface, bukan concrete `*store.Queries`. Karena itu, production code dapat memberikan generated store, sedangkan unit test dapat memberikan fake store.

```go
type CreateInput struct {
	UserID string
	Name   string
}
```

`CreateInput` adalah input milik operasi domain `Create`. HTTP request belum digunakan di sini agar domain tidak bergantung pada bentuk transport.

```go
func NewService(
	accountStore Store,
) *Service {
	return &Service{
		store: accountStore,
	}
}
```

`NewService` melakukan dependency injection: pihak yang melakukan wiring akan membuat store terlebih dahulu, kemudian memberikannya kepada account service.

Constructor ini belum menolak nil. Kedua method service tetap melindungi pemanggilan ketika receiver atau dependency tidak tersedia:

```go
if s == nil || s.store == nil {
	return store.Account{},
		ErrStoreNotConfigured
}
```

Pemeriksaan `s == nil` juga membuat pemanggilan method pada pointer `*Service` yang nil menghasilkan error yang terkontrol, bukan langsung mengakses field dan panic.

## Alur `Create`

Method `Create` menerima input domain:

```go
func (s *Service) Create(
	ctx context.Context,
	input CreateInput,
) (store.Account, error)
```

### 1. Normalisasi dan validasi `UserID`

```go
userID := strings.TrimSpace(input.UserID)
if userID == "" {
	return store.Account{},
		ErrUserIDRequired
}
```

Input seperti `"  practice-user-021  "` menjadi `"practice-user-021"`. Input yang hanya berisi spasi menjadi string kosong dan ditolak.

### 2. Normalisasi dan validasi `Name`

```go
name := strings.TrimSpace(input.Name)
if name == "" {
	return store.Account{},
		ErrNameRequired
}
```

Aturan yang sama diterapkan pada nama. Normalisasi dilakukan sebelum penyimpanan agar spasi luar tidak ikut masuk ke database.

### 3. Mengubah input domain menjadi parameter generated

```go
store.CreateAccountParams{
	UserID: userID,
	Name:   name,
}
```

`CreateAccountParams` dibuat oleh sqlc berdasarkan parameter query `CreateAccount`. Domain service mengisi parameter generated ini dengan nilai yang sudah dinormalisasi.

Field `status`, `version`, `created_at`, dan `updated_at` tidak dikirim karena nilainya berasal dari default PostgreSQL. Tidak ada field email pada input maupun parameter practice ini.

### 4. Membungkus error menggunakan `%w`

```go
if err != nil {
	return store.Account{}, fmt.Errorf(
		"create account: %w",
		err,
	)
}
```

`%w` menambahkan konteks tanpa menghilangkan identitas error asal. Pemanggil masih dapat memakai `errors.Is` atau `errors.As` untuk menemukan penyebab di dalam rantai error.

Jika memakai `%v` sebagai pengganti `%w`, pesan error tetap terlihat, tetapi hubungan dengan error asal tidak dapat ditelusuri menggunakan `errors.Is`.

## Alur `GetByUserID`

`GetByUserID` memeriksa dependency, menormalisasi `userID`, lalu memanggil:

```go
account, err := s.store.GetAccountByUserID(
	ctx,
	userID,
)
```

Query sqlc bertipe `:one`. Jika tidak menemukan row, pgx mengembalikan `pgx.ErrNoRows`. Error infrastruktur tersebut diterjemahkan menjadi bahasa domain:

```go
if errors.Is(err, pgx.ErrNoRows) {
	return store.Account{},
		ErrAccountNotFound
}
```

`errors.Is` dipakai, bukan `err == pgx.ErrNoRows`, agar pemeriksaan tetap benar apabila error pernah dibungkus.

Error selain `pgx.ErrNoRows` tidak dianggap sebagai account yang tidak ditemukan. Error tersebut dibungkus dengan konteks:

```go
return store.Account{}, fmt.Errorf(
	"get account by user id: %w",
	err,
)
```

Dengan demikian, gangguan koneksi atau error SQL tidak salah diterjemahkan menjadi `ErrAccountNotFound`.

## Mengapa masih mengembalikan `store.Account`

Idealnya, domain yang matang dapat memiliki entity sendiri agar tidak bergantung pada model generated sqlc. Pada foundation ini, service sengaja mengembalikan `store.Account` agar alur antarlapisan dapat dipelajari secara bertahap tanpa menambah mapper dan tipe domain baru sekaligus.

Trade-off ini disengaja, tetapi bukan berarti `store.Account` harus selamanya melewati batas domain. Ketika aturan domain bertambah atau model database dan model bisnis mulai berbeda, entity domain dan fungsi mapping dapat diperkenalkan.

## Mengapa belum memakai transaction eksplisit

`Create` saat ini hanya memanggil satu statement:

```text
INSERT INTO account ... RETURNING ...
```

Satu statement PostgreSQL sudah atomic: statement tersebut berhasil seluruhnya atau gagal seluruhnya. Membuka transaction eksplisit hanya untuk satu insert belum memberikan manfaat berarti.

Transaction dibutuhkan ketika satu operasi bisnis melakukan beberapa write yang harus dianggap satu kesatuan. Contohnya adalah registrasi production yang membuat account, auth, setting, device/session, dan personal channel. Jika salah satu write gagal, write lainnya harus ikut dibatalkan.

## Batasan service saat ini

Service pada Step 21 belum menangani:

- batas panjang `user_id` dan `name` sesuai kolom database;
- aturan karakter yang diperbolehkan;
- penerjemahan unique violation menjadi error domain seperti account sudah ada;
- entity domain terpisah dari `store.Account`;
- transaction untuk operasi multi-write;
- logging atau metrics;
- typed-nil dependency di dalam interface.

Nilai `version` juga belum ditentukan oleh service. Query hanya mengirim `user_id` dan `name`, sehingga `version` memakai default database, yaitu `0`. Demikian juga `status` memakai default database yang sudah dibuat migration.

Batasan tersebut tidak perlu diperbaiki sekaligus pada step ini. Tujuannya adalah menjaga perubahan kecil dan mudah diverifikasi.

## Format dan verifikasi

Dari root server, jalankan:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server
gofmt -w internal/domain/account/service.go
go test ./...
```

Hasil yang diharapkan:

```text
?    github.com/imamsyafii-dev/bbge-practice/srv-server/internal/domain/account    [no test files]
```

Package lain juga harus tetap lulus. Integration test PostgreSQL dari Step 20 boleh tampil sebagai `SKIP` ketika `PRACTICE_RUN_POSTGRES_TESTS` tidak diaktifkan.

Step ini belum memerlukan server berjalan dan belum menulis data ke database.

## File yang berubah

File baru yang dibuat:

```text
internal/domain/account/service.go
learning-steps/01-foundation/05-domain-service/step-21-account-domain-service.md
```

File yang tidak berubah:

```text
db/queries/account.sql
internal/store/account.sql.go
internal/store/db.go
internal/store/models.go
internal/store/querier.go
main.go
```

File di `internal/store` yang memiliki header `Code generated by sqlc. DO NOT EDIT.` tidak boleh diedit manual. Perubahan generated store harus selalu berasal dari SQL atau konfigurasi sqlc, kemudian dibuat ulang dengan:

```bash
./dev/sqlc
```

Step 21 tidak mengubah query sehingga tidak perlu menjalankan generator.

## Preview Step 22

Step 22 akan membuat unit test untuk domain service menggunakan fake store, tanpa PostgreSQL. Test akan memeriksa antara lain:

- dependency store yang tidak dikonfigurasi;
- validasi `user_id` dan `name`;
- normalisasi spasi sebelum store dipanggil;
- penerusan hasil `CreateAccount`;
- pembungkusan error dengan `%w`;
- penerjemahan `pgx.ErrNoRows` menjadi `ErrAccountNotFound`;
- penerusan error lain tanpa kehilangan penyebabnya.

Fake store pada Step 22 hanya perlu memenuhi dua method dalam interface `account.Store`; fake tersebut tidak perlu mengimplementasikan seluruh `store.Querier`.
