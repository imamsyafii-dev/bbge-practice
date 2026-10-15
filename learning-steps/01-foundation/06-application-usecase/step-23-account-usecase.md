# Step 23 — Account Application Use Case

Pada Step 21–22, domain service account sudah memiliki aturan validasi dan sudah diuji tanpa PostgreSQL. Step ini menambahkan lapisan application/use case di atas domain service.

Use case menjadi pintu masuk workflow aplikasi. Saat HTTP handler dibuat nanti, handler tidak perlu langsung mengetahui store atau mengatur aturan domain. Handler cukup menerjemahkan request HTTP menjadi input application lalu memanggil use case.

Ruang lingkup Step 23:

1. membuat package `internal/app/account`;
2. mendefinisikan interface sempit untuk dependency domain service;
3. membuat `UseCase` dan constructor-nya;
4. membuat operasi `Create` dan `GetByUserID`;
5. memetakan input application menjadi input domain;
6. meneruskan context, result, dan error dari domain service.

Step ini belum menambahkan unit test, HTTP router, wiring di `main.go`, transaction, ataupun akses database baru.

## Mengapa masuk kelompok `06-application-usecase`

Kelompok `05-domain-service` berfokus pada aturan domain account: validasi, normalisasi, dan penerjemahan error persistence. Mulai Step 23, fokus berpindah ke workflow aplikasi yang memakai domain tersebut.

Karena itu, catatan ini ditempatkan di:

```text
learning-steps/01-foundation/06-application-usecase/
```

Arsitektur yang sedang dibangun:

```text
HTTP handler — dibuat nanti
          |
          v
application account.UseCase
          |
          v
domain account.Service
          |
          v
domain account.Store interface
          |
          v
generated *store.Queries
          |
          v
PostgreSQL
```

Application layer berada di antara transport dan domain. Ia tidak mengetahui detail HTTP dan tidak menjalankan SQL secara langsung.

## Perbandingan dengan BBGE production

Di BBGE production, application/use-case tertentu dapat menyimpan concrete pointer ke domain service. Pendekatan seperti itu sah jika memang sesuai kebutuhan project.

Latihan ini memakai interface kecil yang didefinisikan oleh package pemakai:

```go
type AccountService interface {
	Create(...)
	GetByUserID(...)
}
```

Ini disebut consumer-owned interface: application menentukan hanya method yang benar-benar dibutuhkannya. Manfaat langsung untuk latihan adalah unit test Step 24 dapat memberikan fake service tanpa membuat domain service dan store asli.

Interface bukan berarti fungsi concrete tidak dapat dipanggil. Tanpa interface, `UseCase` juga dapat menyimpan `*accountdomain.Service` dan memanggil method-nya langsung. Interface dipakai di batas dependency agar implementation dapat diganti sesuai kebutuhan, terutama saat test.

## Membuat package application account

Dari root server, buat direktori:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server
mkdir -p internal/app/account
```

Buat file:

```text
internal/app/account/usecase.go
```

## Isi final `usecase.go`

```go
package account

import (
	"context"
	"errors"

	accountdomain "github.com/imamsyafii-dev/bbge-practice/srv-server/internal/domain/account"
	"github.com/imamsyafii-dev/bbge-practice/srv-server/internal/store"
)

var ErrServiceNotConfigured = errors.New(
	"account service not configured",
)

type AccountService interface {
	Create(
		ctx context.Context,
		input accountdomain.CreateInput,
	) (store.Account, error)

	GetByUserID(
		ctx context.Context,
		userID string,
	) (store.Account, error)
}

var _ AccountService = (*accountdomain.Service)(nil)

type UseCase struct {
	service AccountService
}

type CreateInput struct {
	UserID string
	Name   string
}

func NewUseCase(
	accountService AccountService,
) *UseCase {
	return &UseCase{
		service: accountService,
	}
}

func (u *UseCase) Create(
	ctx context.Context,
	input CreateInput,
) (store.Account, error) {
	if u == nil || u.service == nil {
		return store.Account{},
			ErrServiceNotConfigured
	}

	return u.service.Create(
		ctx,
		accountdomain.CreateInput{
			UserID: input.UserID,
			Name:   input.Name,
		},
	)
}

func (u *UseCase) GetByUserID(
	ctx context.Context,
	userID string,
) (store.Account, error) {
	if u == nil || u.service == nil {
		return store.Account{},
			ErrServiceNotConfigured
	}

	return u.service.GetByUserID(
		ctx,
		userID,
	)
}
```

Simpan file dengan satu newline setelah kurung kurawal terakhir.

## Penjelasan import

### `context`

```go
"context"
```

`context.Context` diterima use case lalu diteruskan tanpa diganti ke domain service. Dengan begitu, cancellation dan deadline dari HTTP request nantinya dapat terus mengalir sampai query database.

### `errors`

```go
"errors"
```

Package ini dipakai untuk membuat sentinel error `ErrServiceNotConfigured`.

### Alias `accountdomain`

```go
accountdomain "github.com/imamsyafii-dev/bbge-practice/srv-server/internal/domain/account"
```

Package application ini juga bernama `account`. Alias `accountdomain` membuat asal tipe terlihat jelas dan menghindari kebingungan saat menulis `accountdomain.Service` atau `accountdomain.CreateInput`.

### `internal/store`

```go
"github.com/imamsyafii-dev/bbge-practice/srv-server/internal/store"
```

Pada milestone fondasi ini, use case masih mengembalikan `store.Account`. Ini sengaja dipertahankan agar fokus latihan tetap pada pemisahan lapisan dan dependency.

Namun, ini juga merupakan batasan desain: model persistence masih melewati batas domain dan application. Pada aplikasi yang lebih matang, application dapat mengembalikan DTO atau domain entity sendiri agar tidak terikat pada model generated sqlc.

## `ErrServiceNotConfigured` dan `ErrStoreNotConfigured`

Application memiliki error berikut:

```go
var ErrServiceNotConfigured = errors.New(
	"account service not configured",
)
```

Error ini berbeda dari `accountdomain.ErrStoreNotConfigured`:

- `ErrServiceNotConfigured` berarti `UseCase` tidak diberi domain service;
- `accountdomain.ErrStoreNotConfigured` berarti domain `Service` ada, tetapi tidak diberi store.

Keduanya menandai dependency yang hilang pada batas lapisan yang berbeda.

## Interface `AccountService`

```go
type AccountService interface {
	Create(
		ctx context.Context,
		input accountdomain.CreateInput,
	) (store.Account, error)

	GetByUserID(
		ctx context.Context,
		userID string,
	) (store.Account, error)
}
```

Interface hanya berisi dua operasi yang dibutuhkan use case saat ini:

- `Create` untuk membuat account;
- `GetByUserID` untuk mengambil account berdasarkan `user_id`.

Application tidak perlu menyalin seluruh fungsi milik domain service. Jika domain service kelak memiliki method tambahan yang tidak dipakai application account, interface ini tidak perlu ikut berubah.

Signature interface harus sama persis dengan method concrete yang akan memenuhinya, termasuk tipe input dan output.

## Compile-time assertion

```go
var _ AccountService = (*accountdomain.Service)(nil)
```

Baris ini meminta compiler memastikan `*accountdomain.Service` memenuhi `AccountService`.

- `_` membuang nilai karena tidak diperlukan saat runtime;
- `(*accountdomain.Service)(nil)` menyatakan typed nil pointer tanpa membuat service;
- tidak ada constructor, database, atau method yang dijalankan oleh baris ini.

Jika signature domain berubah dan tidak lagi cocok, build gagal di lokasi yang jelas.

## Mengapa ada dua `CreateInput`

Application memiliki input sendiri:

```go
type CreateInput struct {
	UserID string
	Name   string
}
```

Domain juga memiliki `accountdomain.CreateInput` dengan field yang saat ini sama. Meskipun field-nya sama, kedua struct tersebut adalah named type yang berbeda di Go karena dideklarasikan dengan nama pada package berbeda.

Karena `*accountdomain.Service.Create` menerima `accountdomain.CreateInput`, interface juga harus memakai tipe domain tersebut:

```go
Create(
	ctx context.Context,
	input accountdomain.CreateInput,
) (store.Account, error)
```

Contoh yang salah:

```go
type AccountService interface {
	Create(
		ctx context.Context,
		input CreateInput,
	) (store.Account, error)
}
```

Walaupun kedua `CreateInput` mempunyai field yang sama, compile assertion akan gagal dengan error yang intinya seperti:

```text
*accountdomain.Service does not implement AccountService
(wrong type for method Create)
have Create(context.Context, accountdomain.CreateInput) (store.Account, error)
want Create(context.Context, CreateInput) (store.Account, error)
```

Use case melakukan mapping secara eksplisit:

```go
accountdomain.CreateInput{
	UserID: input.UserID,
	Name:   input.Name,
}
```

Mapping ini menunjukkan dengan jelas kapan data melewati batas application menuju domain. Kelak input application dapat memiliki field atau bentuk yang berbeda tanpa memaksa kontrak domain mengikuti bentuk transport.

## `UseCase` dan constructor

```go
type UseCase struct {
	service AccountService
}
```

`UseCase` menyimpan dependency melalui interface, bukan membuat domain service sendiri.

```go
func NewUseCase(
	accountService AccountService,
) *UseCase {
	return &UseCase{
		service: accountService,
	}
}
```

Constructor menerima dependency dari luar. Proses menghubungkan concrete dependency seperti ini disebut wiring, tetapi wiring ke `main.go` belum dilakukan pada step ini.

Constructor sengaja belum menolak nilai `nil`. Kedua method melakukan pemeriksaan sebelum memakai dependency supaya kesalahan konfigurasi dikembalikan sebagai error.

## Nil receiver dan dependency nil

Kedua method memiliki guard yang sama:

```go
if u == nil || u.service == nil {
	return store.Account{},
		ErrServiceNotConfigured
}
```

`u == nil` menangani pemanggilan method pada nil pointer:

```go
var useCase *UseCase
_, err := useCase.GetByUserID(ctx, "practice-user")
```

`u.service == nil` menangani use case yang dibuat tanpa dependency:

```go
useCase := NewUseCase(nil)
```

### Catatan typed nil di dalam interface

Interface Go dapat tidak sama dengan `nil` walaupun nilai concrete di dalamnya adalah nil pointer:

```go
var service *accountdomain.Service
useCase := NewUseCase(service)
```

Pada contoh tersebut, `useCase.service != nil` karena interface menyimpan informasi tipe `*accountdomain.Service`. Pada implementasi saat ini, pemanggilan diteruskan ke domain service dan domain service sendiri aman terhadap nil receiver sehingga mengembalikan `accountdomain.ErrStoreNotConfigured`.

Namun, implementation lain yang tidak menangani nil receiver dapat panic. Jadi, pemeriksaan `interface == nil` tidak dapat mendeteksi seluruh typed nil. Untuk latihan ini kita tidak menambahkan reflection; wiring yang benar harus memberikan instance concrete yang valid.

## Alur `Create`

```go
func (u *UseCase) Create(
	ctx context.Context,
	input CreateInput,
) (store.Account, error) {
	if u == nil || u.service == nil {
		return store.Account{},
			ErrServiceNotConfigured
	}

	return u.service.Create(
		ctx,
		accountdomain.CreateInput{
			UserID: input.UserID,
			Name:   input.Name,
		},
	)
}
```

Urutannya:

1. memastikan use case dan service tersedia;
2. mengubah application `CreateInput` menjadi domain `CreateInput`;
3. meneruskan context yang sama;
4. memanggil domain service;
5. meneruskan account dan error dari service apa adanya.

Use case tidak memanggil `strings.TrimSpace` dan tidak mengulang validasi `user_id` atau `name`. Aturan tersebut sudah menjadi tanggung jawab domain service. Duplikasi aturan pada dua lapisan dapat membuat perilaku berbeda ketika salah satunya berubah.

## Alur `GetByUserID`

```go
func (u *UseCase) GetByUserID(
	ctx context.Context,
	userID string,
) (store.Account, error) {
	if u == nil || u.service == nil {
		return store.Account{},
			ErrServiceNotConfigured
	}

	return u.service.GetByUserID(
		ctx,
		userID,
	)
}
```

Tidak ada mapping struct karena input-nya hanya string. Context, `userID`, result, dan error diteruskan langsung. Normalisasi spasi dan penerjemahan `pgx.ErrNoRows` tetap dilakukan domain service.

## Mengapa use case tetap berguna walaupun masih tipis

Pada kondisi sekarang, use case tampak hanya meneruskan pemanggilan. Lapisan ini tetap memberi batas yang jelas untuk workflow aplikasi.

Nantinya satu use case dapat:

- memanggil beberapa domain service dalam satu workflow;
- membuka dan menyelesaikan transaction;
- melakukan pemeriksaan authorization;
- mengatur urutan perubahan beberapa aggregate;
- mengirim event atau notification setelah operasi berhasil;
- menyusun output yang dibutuhkan transport.

HTTP handler kemudian tetap sederhana dan tidak mengambil alih orkestrasi proses bisnis.

Use case tidak harus selalu memanggil tepat satu domain service. Sebuah handler biasanya memanggil satu use case sesuai satu aksi aplikasi, sedangkan use case tersebut boleh mengoordinasikan beberapa dependency sesuai kebutuhan workflow.

## Hal yang sengaja belum dilakukan

Pada Step 23:

- tidak ada field email;
- tidak ada query atau perubahan database;
- tidak ada transaction;
- tidak ada koneksi PostgreSQL baru;
- tidak ada perubahan generated sqlc;
- belum ada wiring di `main.go`;
- belum ada HTTP handler atau router account;
- belum ada unit test use case.

## Troubleshooting: `package account` tertulis dua kali

Saat file dibuat manual, sempat terjadi typo berikut:

```go
package account
package account
```

Satu file Go hanya boleh mempunyai satu package clause. Bentuk final yang benar hanya:

```go
package account
```

Jika package clause tertulis dua kali, formatter atau compiler akan melaporkan syntax error. Hapus baris duplikat, lalu jalankan kembali `gofmt` dan test. Typo ini hanya catatan troubleshooting dan tidak ada pada source final di atas.

## Format dan verifikasi

Dari root server, jalankan:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server

gofmt -w internal/app/account/usecase.go
go test ./internal/app/account
go test ./...
```

Karena unit test package application baru dibuat pada Step 24, perintah package test saat ini diharapkan menampilkan bentuk seperti:

```text
?   github.com/imamsyafii-dev/bbge-practice/srv-server/internal/app/account   [no test files]
```

`go test ./...` juga harus selesai tanpa `FAIL`. PostgreSQL integration test dari Step 20 akan dilewati jika `PRACTICE_RUN_POSTGRES_TESTS` tidak diset ke `1`, sehingga Docker tidak diperlukan untuk verifikasi biasa Step 23.

## File yang berubah

File source baru:

```text
srv-server/internal/app/account/usecase.go
```

Catatan belajar baru:

```text
learning-steps/01-foundation/06-application-usecase/step-23-account-usecase.md
```

File berikut tidak berubah pada step ini:

- `internal/domain/account/service.go`;
- `internal/domain/account/service_test.go`;
- seluruh file `internal/store`;
- seluruh migration dan query SQL;
- `config`, `docker`, dan `main.go`.

## Ringkasan Step 23

Setelah step ini:

1. application account mempunyai batas use case sendiri;
2. dependency domain dinyatakan melalui interface `AccountService` yang sempit;
3. compiler memastikan `*accountdomain.Service` memenuhi interface;
4. application input dipetakan eksplisit menjadi domain input;
5. aturan validasi tetap berada di domain;
6. context, result, dan error diteruskan tanpa diubah;
7. desain masih sederhana dan belum terhubung ke HTTP atau runtime server.

## Preview Step 24

Step 24 akan membuat `internal/app/account/usecase_test.go` dengan fake `AccountService`. Test akan memeriksa:

- nil receiver dan dependency nil;
- mapping application `CreateInput` ke `accountdomain.CreateInput`;
- penerusan context;
- penerusan result;
- penerusan error tanpa perubahan;
- delegasi `GetByUserID`;
- jumlah pemanggilan fake service.

Test tersebut tidak membutuhkan Docker atau PostgreSQL.
