# Step 24 — Unit Test Account Application Use Case

Pada Step 23, application account sudah memiliki `UseCase` yang bergantung pada interface kecil `AccountService`. Step ini menguji batas application tersebut menggunakan fake service, tanpa membuat domain service asli, store sqlc, koneksi PostgreSQL, ataupun Docker.

Tujuan Step 24:

1. memastikan `UseCase` menolak receiver atau dependency yang belum dikonfigurasi;
2. memastikan input application dipetakan ke input domain dengan tepat;
3. memastikan `context.Context` diteruskan tanpa diganti;
4. memastikan result dan error dari service diteruskan apa adanya;
5. memastikan hanya method dependency yang semestinya yang dipanggil;
6. memahami perbedaan method use case, method fake, dan callback yang diatur oleh test.

Step ini tidak menjalankan domain service, generated store, query SQL, PostgreSQL, migration, atau Docker.

## File yang dibuat

Buat file:

```text
srv-server/internal/app/account/usecase_test.go
```

Test memakai package yang sama dengan source:

```go
package account
```

Dengan package yang sama, test dapat mengakses identifier package application secara langsung, seperti `UseCase`, `CreateInput`, `AccountService`, dan `ErrServiceNotConfigured`.

## Isi final `usecase_test.go`

```go
package account

import (
	"context"
	"errors"
	"reflect"
	"testing"

	accountdomain "github.com/imamsyafii-dev/bbge-practice/srv-server/internal/domain/account"
	"github.com/imamsyafii-dev/bbge-practice/srv-server/internal/store"
)

type useCaseTestContextKey struct{}

type fakeAccountService struct {
	createFunc func(
		context.Context,
		accountdomain.CreateInput,
	) (store.Account, error)

	getByUserIDFunc func(
		context.Context,
		string,
	) (store.Account, error)

	createCalls      int
	getByUserIDCalls int
}

var _ AccountService = (*fakeAccountService)(nil)

func (f *fakeAccountService) Create(
	ctx context.Context,
	input accountdomain.CreateInput,
) (store.Account, error) {
	f.createCalls++

	if f.createFunc == nil {
		panic("unexpected Create call")
	}

	return f.createFunc(ctx, input)
}

func (f *fakeAccountService) GetByUserID(
	ctx context.Context,
	userID string,
) (store.Account, error) {
	f.getByUserIDCalls++

	if f.getByUserIDFunc == nil {
		panic("unexpected GetByUserID call")
	}

	return f.getByUserIDFunc(
		ctx,
		userID,
	)
}

func TestUseCaseRequiresConfiguredService(
	t *testing.T,
) {
	tests := []struct {
		name    string
		useCase *UseCase
	}{
		{
			name:    "nil receiver",
			useCase: nil,
		},
		{
			name:    "nil dependency",
			useCase: NewUseCase(nil),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			account, err := test.useCase.Create(
				context.Background(),
				CreateInput{
					UserID: "practice-user-024",
					Name:   "Practice User",
				},
			)
			if err != ErrServiceNotConfigured {
				t.Fatalf(
					"Create error = %v, want %v",
					err,
					ErrServiceNotConfigured,
				)
			}

			if !reflect.DeepEqual(
				account,
				store.Account{},
			) {
				t.Fatalf(
					"Create account = %+v, want zero value",
					account,
				)
			}

			account, err = test.useCase.GetByUserID(
				context.Background(),
				"practice-user-024",
			)
			if err != ErrServiceNotConfigured {
				t.Fatalf(
					"GetByUserID error = %v, want %v",
					err,
					ErrServiceNotConfigured,
				)
			}

			if !reflect.DeepEqual(
				account,
				store.Account{},
			) {
				t.Fatalf(
					"GetByUserID account = %+v, want zero value",
					account,
				)
			}
		})
	}
}

func TestUseCaseCreateForwardsInputAndResult(
	t *testing.T,
) {
	ctx := context.WithValue(
		context.Background(),
		useCaseTestContextKey{},
		"create",
	)

	input := CreateInput{
		UserID: "  practice-user-024  ",
		Name:   "  Practice User  ",
	}

	expectedDomainInput := accountdomain.CreateInput{
		UserID: input.UserID,
		Name:   input.Name,
	}

	expectedResult := store.Account{
		ID:      24,
		UserID:  "practice-user-024",
		Name:    "Practice User",
		Status:  1,
		Version: 0,
	}

	fake := &fakeAccountService{
		createFunc: func(
			gotCtx context.Context,
			gotInput accountdomain.CreateInput,
		) (store.Account, error) {
			if gotCtx != ctx {
				t.Fatal(
					"Create context was not forwarded unchanged",
				)
			}

			if gotInput != expectedDomainInput {
				t.Fatalf(
					"Create input = %+v, want %+v",
					gotInput,
					expectedDomainInput,
				)
			}

			return expectedResult, nil
		},
	}

	useCase := NewUseCase(fake)

	result, err := useCase.Create(
		ctx,
		input,
	)
	if err != nil {
		t.Fatalf(
			"Create returned error: %v",
			err,
		)
	}

	if fake.createCalls != 1 {
		t.Fatalf(
			"Create calls = %d, want 1",
			fake.createCalls,
		)
	}

	if fake.getByUserIDCalls != 0 {
		t.Fatalf(
			"GetByUserID calls = %d, want 0",
			fake.getByUserIDCalls,
		)
	}

	if !reflect.DeepEqual(
		result,
		expectedResult,
	) {
		t.Fatalf(
			"Create result = %+v, want %+v",
			result,
			expectedResult,
		)
	}
}

func TestUseCaseCreateForwardsExactError(
	t *testing.T,
) {
	serviceErr := errors.New("create failed")

	expectedResult := store.Account{
		ID:     24,
		UserID: "practice-user-024",
		Name:   "Practice User",
	}

	fake := &fakeAccountService{
		createFunc: func(
			context.Context,
			accountdomain.CreateInput,
		) (store.Account, error) {
			return expectedResult, serviceErr
		},
	}

	useCase := NewUseCase(fake)

	result, err := useCase.Create(
		context.Background(),
		CreateInput{
			UserID: "practice-user-024",
			Name:   "Practice User",
		},
	)

	if err != serviceErr {
		t.Fatalf(
			"Create error = %v, want exact error %v",
			err,
			serviceErr,
		)
	}

	if fake.createCalls != 1 {
		t.Fatalf(
			"Create calls = %d, want 1",
			fake.createCalls,
		)
	}

	if !reflect.DeepEqual(
		result,
		expectedResult,
	) {
		t.Fatalf(
			"Create result = %+v, want %+v",
			result,
			expectedResult,
		)
	}
}

func TestUseCaseGetByUserIDForwardsInputAndResult(
	t *testing.T,
) {
	ctx := context.WithValue(
		context.Background(),
		useCaseTestContextKey{},
		"get",
	)

	userID := "  practice-user-024  "

	expectedResult := store.Account{
		ID:      24,
		UserID:  "practice-user-024",
		Name:    "Practice User",
		Status:  1,
		Version: 0,
	}

	fake := &fakeAccountService{
		getByUserIDFunc: func(
			gotCtx context.Context,
			gotUserID string,
		) (store.Account, error) {
			if gotCtx != ctx {
				t.Fatal(
					"GetByUserID context was not forwarded unchanged",
				)
			}

			if gotUserID != userID {
				t.Fatalf(
					"GetByUserID userID = %q, want %q",
					gotUserID,
					userID,
				)
			}

			return expectedResult, nil
		},
	}

	useCase := NewUseCase(fake)

	result, err := useCase.GetByUserID(
		ctx,
		userID,
	)
	if err != nil {
		t.Fatalf(
			"GetByUserID returned error: %v",
			err,
		)
	}

	if fake.getByUserIDCalls != 1 {
		t.Fatalf(
			"GetByUserID calls = %d, want 1",
			fake.getByUserIDCalls,
		)
	}

	if fake.createCalls != 0 {
		t.Fatalf(
			"Create calls = %d, want 0",
			fake.createCalls,
		)
	}

	if !reflect.DeepEqual(
		result,
		expectedResult,
	) {
		t.Fatalf(
			"GetByUserID result = %+v, want %+v",
			result,
			expectedResult,
		)
	}
}

func TestUseCaseGetByUserIDForwardsExactError(
	t *testing.T,
) {
	serviceErr := errors.New("get failed")

	expectedResult := store.Account{
		ID:     24,
		UserID: "practice-user-024",
		Name:   "Practice User",
	}

	fake := &fakeAccountService{
		getByUserIDFunc: func(
			context.Context,
			string,
		) (store.Account, error) {
			return expectedResult, serviceErr
		},
	}

	useCase := NewUseCase(fake)

	result, err := useCase.GetByUserID(
		context.Background(),
		"practice-user-024",
	)

	if err != serviceErr {
		t.Fatalf(
			"GetByUserID error = %v, want exact error %v",
			err,
			serviceErr,
		)
	}

	if fake.getByUserIDCalls != 1 {
		t.Fatalf(
			"GetByUserID calls = %d, want 1",
			fake.getByUserIDCalls,
		)
	}

	if !reflect.DeepEqual(
		result,
		expectedResult,
	) {
		t.Fatalf(
			"GetByUserID result = %+v, want %+v",
			result,
			expectedResult,
		)
	}
}
```

Simpan file dengan satu newline setelah kurung kurawal terakhir.

## Import yang dipakai

```go
import (
	"context"
	"errors"
	"reflect"
	"testing"

	accountdomain "github.com/imamsyafii-dev/bbge-practice/srv-server/internal/domain/account"
	"github.com/imamsyafii-dev/bbge-practice/srv-server/internal/store"
)
```

- `context` dipakai untuk membuat context yang akan diperiksa apakah diteruskan tanpa perubahan.
- `errors` dipakai untuk membuat instance error khusus yang identitasnya diperiksa.
- `reflect` dipakai untuk membandingkan seluruh nilai `store.Account`, termasuk zero value.
- `testing` menyediakan `*testing.T`, subtest, dan assertion manual.
- `accountdomain` diperlukan karena signature `AccountService.Create` menerima `accountdomain.CreateInput`, bukan `CreateInput` milik application.
- `store` menyediakan tipe result `store.Account`.

## Struktur fake service

Fake memiliki callback dan counter:

```go
type fakeAccountService struct {
	createFunc func(
		context.Context,
		accountdomain.CreateInput,
	) (store.Account, error)

	getByUserIDFunc func(
		context.Context,
		string,
	) (store.Account, error)

	createCalls      int
	getByUserIDCalls int
}
```

Callback membuat setiap test dapat menentukan perilaku dependency tanpa database. Counter mencatat berapa kali masing-masing method dipanggil.

Kedua signature harus sama persis dengan interface `AccountService`:

```go
Create(
	context.Context,
	accountdomain.CreateInput,
) (store.Account, error)

GetByUserID(
	context.Context,
	string,
) (store.Account, error)
```

Khususnya, `Create` harus menerima `accountdomain.CreateInput`. Jika fake memakai `CreateInput` milik package application, fake tidak memenuhi interface walaupun field struct-nya terlihat sama.

## Compile-time assertion

```go
var _ AccountService = (*fakeAccountService)(nil)
```

Baris ini hanya meminta compiler memastikan bahwa `*fakeAccountService` memenuhi `AccountService`. Baris ini tidak membuat fake, tidak memanggil callback, dan tidak menjalankan domain atau database.

Jika salah satu method hilang atau signature-nya berbeda, compilation akan gagal langsung di assertion ini.

## Tiga lapisan pemanggilan di dalam test

Alur `Create`:

```text
useCase.Create(ctx, input)        method application yang sedang diuji
            |
            v
fake.Create(ctx, domainInput)     implementation palsu dari dependency
            |
            v
fake.createFunc(...)              callback yang diatur oleh test
            |
            v
expectedResult, error             hasil buatan test
```

Alur `GetByUserID`:

```text
useCase.GetByUserID(ctx, userID)  method application yang sedang diuji
            |
            v
fake.GetByUserID(ctx, userID)     implementation palsu dari dependency
            |
            v
fake.getByUserIDFunc(...)         callback yang diatur oleh test
            |
            v
expectedResult, error             hasil buatan test
```

Jadi, `useCase.Create` dan `useCase.GetByUserID` berasal dari `usecase.go`. Method `fakeAccountService.Create` dan `fakeAccountService.GetByUserID` adalah dependency palsu. Sementara `createFunc` dan `getByUserIDFunc` bukan method use case; keduanya adalah fungsi yang disuntikkan oleh masing-masing test untuk mengatur response fake.

## Counter dan panic untuk panggilan tidak terduga

Method fake menaikkan counter sebelum memanggil callback:

```go
f.createCalls++
```

Dengan counter, test membuktikan bukan hanya hasil akhirnya, tetapi juga interaksinya:

- operasi `Create` harus memanggil service `Create` tepat satu kali;
- operasi `Create` tidak boleh memanggil `GetByUserID`;
- operasi `GetByUserID` harus memanggil service `GetByUserID` tepat satu kali;
- operasi `GetByUserID` tidak boleh memanggil `Create`.

Jika callback yang tidak disiapkan justru dipanggil, fake menghentikan test:

```go
if f.createFunc == nil {
	panic("unexpected Create call")
}
```

Panic tersebut adalah penjaga kesalahan penyusunan alur. Misalnya, test `GetByUserID` tidak mengisi `createFunc`; jika use case keliru memanggil `Create`, test langsung gagal dan memperlihatkan panggilan yang tidak semestinya.

## Test receiver dan dependency nil

`TestUseCaseRequiresConfiguredService` menguji dua keadaan:

```text
nil receiver     -> var useCase *UseCase = nil
nil dependency   -> NewUseCase(nil)
```

Keduanya harus mengembalikan:

```go
ErrServiceNotConfigured
```

dan zero value:

```go
store.Account{}
```

Test memanggil `Create` dan `GetByUserID` pada kedua keadaan. Pemanggilan method dengan receiver pointer nil diperbolehkan oleh Go selama implementasi method memeriksa receiver sebelum mencoba mengakses field-nya. Guard di source adalah:

```go
if u == nil || u.service == nil {
	return store.Account{}, ErrServiceNotConfigured
}
```

`||` dievaluasi dari kiri ke kanan. Ketika `u == nil`, bagian `u.service` tidak dievaluasi sehingga tidak terjadi dereference pada pointer nil.

## Peringatan typed nil di dalam interface

Guard `u.service == nil` mendeteksi interface nil biasa:

```go
NewUseCase(nil)
```

Namun, interface Go menyimpan pasangan tipe dinamis dan nilai dinamis. Interface yang berisi typed nil pointer tidak sama dengan `nil`:

```go
var fake *fakeAccountService = nil
useCase := NewUseCase(fake)
```

Pada contoh itu, `useCase.service != nil` karena interface masih membawa tipe dinamis `*fakeAccountService`, meskipun nilai pointer di dalamnya nil. Guard saat ini tidak dimaksudkan untuk menangani keadaan tersebut. Jika method dipanggil, fake dapat panic saat mengakses `f.createCalls` atau `f.getByUserIDCalls`.

Praktik sederhananya: jangan menyuntikkan typed nil sebagai dependency. Berikan `nil` interface biasa untuk keadaan belum dikonfigurasi, atau berikan pointer implementation yang benar-benar sudah dibuat.

## Mapping input application ke input domain

Application dan domain memiliki named input type yang berbeda:

```go
input := CreateInput{
	UserID: "  practice-user-024  ",
	Name:   "  Practice User  ",
}

expectedDomainInput := accountdomain.CreateInput{
	UserID: input.UserID,
	Name:   input.Name,
}
```

Test memastikan use case melakukan mapping field yang benar. Ini penting karena compiler hanya memastikan tipe output mapping benar; compiler tidak dapat mengetahui jika programmer tanpa sengaja menukar atau melupakan field yang memiliki tipe sama.

## Spasi sengaja tidak dibuang oleh application

Input test sengaja memiliki spasi:

```go
UserID: "  practice-user-024  "
Name:   "  Practice User  "
```

Callback mengharapkan spasi yang sama. Ini membuktikan application hanya memetakan input dan tidak mengulangi `strings.TrimSpace`.

Normalisasi sudah menjadi tanggung jawab domain service dari Step 21. Jika application juga melakukan normalisasi, aturan yang sama akan tersebar di dua lapisan dan berisiko berbeda saat salah satunya diubah.

Result dari callback memakai nilai yang sudah normal:

```go
expectedResult := store.Account{
	UserID: "practice-user-024",
	Name:   "Practice User",
}
```

Ini mensimulasikan domain service yang menerima input mentah, menormalisasinya, lalu mengembalikan account hasil proses.

## Context diteruskan tanpa perubahan

Test membuat context tertentu:

```go
ctx := context.WithValue(
	context.Background(),
	useCaseTestContextKey{},
	"create",
)
```

Callback kemudian membandingkan object context yang diterima:

```go
if gotCtx != ctx {
	t.Fatal("Create context was not forwarded unchanged")
}
```

Pemeriksaan yang sama dilakukan pada `GetByUserID`. Ini memastikan cancellation, deadline, dan value dari caller dapat terus mengalir ke domain dan akhirnya ke operasi persistence.

Tipe key kosong khusus test mencegah penggunaan string biasa sebagai context key:

```go
type useCaseTestContextKey struct{}
```

## Dari `expectedResult` sampai kembali dari use case

Pada test sukses, `expectedResult` dibuat oleh test lalu dikembalikan oleh callback:

```go
return expectedResult, nil
```

Urutan nilainya adalah:

```text
expectedResult
      |
      v
createFunc/getByUserIDFunc
      |
      v
fakeAccountService method
      |
      v
AccountService interface
      |
      v
UseCase result
```

Karena `UseCase` langsung mengembalikan hasil pemanggilan service, nilai akhir harus sama dengan `expectedResult`.

## Result dan exact error diteruskan bersama-sama

Test error sengaja mengembalikan result nonzero bersama error:

```go
return expectedResult, serviceErr
```

Walaupun pola umum Go sering mengembalikan zero value ketika ada error, secara teknis sebuah fungsi dapat mengembalikan result dan error sekaligus. Test ini memastikan use case tidak diam-diam mengganti result menjadi zero value dan tidak mengganti atau membungkus error.

Identitas error diperiksa dengan:

```go
if err != serviceErr {
```

Pemeriksaan tersebut lebih ketat daripada hanya membandingkan teks error. Ia membuktikan bahwa instance `serviceErr` yang sama diteruskan oleh use case. Ini sesuai implementasi Step 23 yang langsung me-return hasil service:

```go
return u.service.Create(...)
```

dan:

```go
return u.service.GetByUserID(...)
```

Jika use case kelak sengaja membungkus error dengan `%w`, kontrak dan test harus ditinjau kembali; saat ini kontraknya adalah exact pass-through.

## Mengapa memakai `reflect.DeepEqual`

Test membandingkan `store.Account` menggunakan:

```go
reflect.DeepEqual(result, expectedResult)
```

Pendekatan ini membandingkan seluruh struct, termasuk field yang tidak disebut satu per satu. Ia juga dipakai untuk memastikan hasil error konfigurasi benar-benar sama dengan `store.Account{}`.

Untuk struct sederhana yang seluruh field-nya comparable, operator `==` bisa saja cukup. `reflect.DeepEqual` dipakai konsisten di latihan ini agar perbandingan tetap mudah ketika struct memiliki field nested yang tidak nyaman dibandingkan satu per satu.

## Menjalankan format dan test

Dari root server:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server

gofmt -w internal/app/account/usecase_test.go
go test ./internal/app/account -v
go test ./...
```

Tidak perlu menjalankan Docker atau PostgreSQL.

Output focused test yang representatif:

```text
=== RUN   TestUseCaseRequiresConfiguredService
=== RUN   TestUseCaseRequiresConfiguredService/nil_receiver
=== RUN   TestUseCaseRequiresConfiguredService/nil_dependency
--- PASS: TestUseCaseRequiresConfiguredService (0.00s)
    --- PASS: TestUseCaseRequiresConfiguredService/nil_receiver (0.00s)
    --- PASS: TestUseCaseRequiresConfiguredService/nil_dependency (0.00s)
=== RUN   TestUseCaseCreateForwardsInputAndResult
--- PASS: TestUseCaseCreateForwardsInputAndResult (0.00s)
=== RUN   TestUseCaseCreateForwardsExactError
--- PASS: TestUseCaseCreateForwardsExactError (0.00s)
=== RUN   TestUseCaseGetByUserIDForwardsInputAndResult
--- PASS: TestUseCaseGetByUserIDForwardsInputAndResult (0.00s)
=== RUN   TestUseCaseGetByUserIDForwardsExactError
--- PASS: TestUseCaseGetByUserIDForwardsExactError (0.00s)
PASS
ok      github.com/imamsyafii-dev/bbge-practice/srv-server/internal/app/account
```

Durasi dan penanda `(cached)` dapat berbeda. Hal pentingnya adalah kelima test utama dan dua subtest menghasilkan `PASS`, lalu seluruh package pada `go test ./...` juga lulus.

## Yang berubah pada Step 24

File source test baru:

```text
srv-server/internal/app/account/usecase_test.go
```

Catatan belajar baru:

```text
learning-steps/01-foundation/06-application-usecase/step-24-account-usecase-test.md
```

## Yang tidak berubah

Step ini tidak mengubah:

- `internal/app/account/usecase.go`;
- `internal/domain/account/service.go` dan test domain;
- generated files dalam `internal/store`;
- query SQL atau migration;
- `main.go` dan config;
- dependency dalam `go.mod` atau `go.sum`;
- Docker Compose dan database.

Test hanya memanggil application use case dan fake `AccountService`. Tidak ada domain service, store, atau database yang dijalankan.

## Ringkasan kontrak yang sudah diuji

Setelah Step 24, unit test membuktikan bahwa application account:

1. mengembalikan `ErrServiceNotConfigured` dan zero account untuk receiver atau dependency nil;
2. memetakan `CreateInput` application menjadi `accountdomain.CreateInput`;
3. meneruskan whitespace tanpa normalisasi ulang;
4. meneruskan context yang sama;
5. memanggil operasi service yang benar tepat satu kali;
6. tidak memanggil operasi service yang tidak diperlukan;
7. meneruskan result service tanpa perubahan;
8. meneruskan instance error yang sama tanpa wrapping;
9. tetap meneruskan result nonzero ketika service mengembalikan result dan error bersama-sama.

## Preview Step 25

Step 25 akan mulai membuat lapisan HTTP handler/router untuk account. Handler akan bergantung pada interface use case yang sempit, bukan langsung pada domain service atau store.

Alur berikutnya:

```text
HTTP request
      |
      v
account handler/router
      |
      v
application use case interface
      |
      v
domain service
      |
      v
store
```

Handler akan menerjemahkan JSON atau path parameter menjadi input use case, lalu memetakan hasil dan error ke HTTP response. Contoh arah pemetaan status yang akan dipelajari:

```text
request tidak valid            -> 400 Bad Request
account tidak ditemukan        -> 404 Not Found
dependency/internal error      -> 500 Internal Server Error
create berhasil                -> status sukses yang sesuai
get berhasil                   -> 200 OK
```

Detail kontrak request, response, interface use case, dan pemetaan error-to-status akan ditentukan dan diuji pada Step 25 tanpa meletakkan aturan domain di dalam handler.
