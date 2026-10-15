# Step 22 — Unit Test Account Domain Service dengan Fake Store

Pada Step 21, domain service account sudah memiliki validasi, normalisasi input, pemanggilan store, dan penerjemahan error. Step ini menguji setiap perilaku tersebut tanpa PostgreSQL dengan membuat `fakeStore` yang memenuhi interface `Store`.

Tujuan Step 22:

1. menguji `Service.Create` dan `Service.GetByUserID` secara terpisah dari database;
2. memastikan input tidak valid berhenti sebelum store dipanggil;
3. memastikan input yang valid dinormalisasi dan diteruskan dengan benar;
4. memastikan context yang diterima service diteruskan ke store;
5. memastikan hasil dan error store diproses sesuai kontrak domain;
6. memahami hubungan antara interface, fake method, dan callback test.

Step ini adalah **unit test**, bukan integration test. Docker, PostgreSQL, migration, dan environment variable `PRACTICE_RUN_POSTGRES_TESTS` tidak diperlukan.

## File yang dibuat

Buat file berikut:

```text
srv-server/internal/domain/account/service_test.go
```

Isi final file tersebut adalah:

```go
package account

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/imamsyafii-dev/bbge-practice/srv-server/internal/store"
	"github.com/jackc/pgx/v5"
)

type testContextKey struct{}

type fakeStore struct {
	createAccountFunc func(
		context.Context,
		store.CreateAccountParams,
	) (store.Account, error)

	getAccountByUserIDFunc func(
		context.Context,
		string,
	) (store.Account, error)

	createAccountCalls      int
	getAccountByUserIDCalls int
}

var _ Store = (*fakeStore)(nil)

func (f *fakeStore) CreateAccount(
	ctx context.Context,
	arg store.CreateAccountParams,
) (store.Account, error) {
	f.createAccountCalls++

	if f.createAccountFunc == nil {
		panic("unexpected CreateAccount call")
	}

	return f.createAccountFunc(ctx, arg)
}

func (f *fakeStore) GetAccountByUserID(
	ctx context.Context,
	userID string,
) (store.Account, error) {
	f.getAccountByUserIDCalls++

	if f.getAccountByUserIDFunc == nil {
		panic("unexpected GetAccountByUserID call")
	}

	return f.getAccountByUserIDFunc(
		ctx,
		userID,
	)
}

func TestServiceRequiresConfiguredStore(
	t *testing.T,
) {
	tests := []struct {
		name    string
		service *Service
	}{
		{
			name:    "nil receiver",
			service: nil,
		},
		{
			name:    "nil dependency",
			service: NewService(nil),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.service.Create(
				context.Background(),
				CreateInput{
					UserID: "practice-user-022",
					Name:   "Practice User",
				},
			)
			if !errors.Is(
				err,
				ErrStoreNotConfigured,
			) {
				t.Fatalf(
					"Create error = %v, want %v",
					err,
					ErrStoreNotConfigured,
				)
			}

			_, err = test.service.GetByUserID(
				context.Background(),
				"practice-user-022",
			)
			if !errors.Is(
				err,
				ErrStoreNotConfigured,
			) {
				t.Fatalf(
					"GetByUserID error = %v, want %v",
					err,
					ErrStoreNotConfigured,
				)
			}
		})
	}
}

func TestServiceCreateRejectsInvalidInputWithoutStoreCall(
	t *testing.T,
) {
	tests := []struct {
		name    string
		input   CreateInput
		wantErr error
	}{
		{
			name: "user id is required",
			input: CreateInput{
				UserID: " \t ",
				Name:   "Practice User",
			},
			wantErr: ErrUserIDRequired,
		},
		{
			name: "name is required",
			input: CreateInput{
				UserID: "practice-user-022",
				Name:   " \n ",
			},
			wantErr: ErrNameRequired,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := &fakeStore{}
			service := NewService(fake)

			_, err := service.Create(
				context.Background(),
				test.input,
			)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf(
					"Create error = %v, want %v",
					err,
					test.wantErr,
				)
			}

			if fake.createAccountCalls != 0 {
				t.Fatalf(
					"CreateAccount calls = %d, want 0",
					fake.createAccountCalls,
				)
			}
		})
	}
}

func TestServiceCreateNormalizesAndDelegates(
	t *testing.T,
) {
	ctx := context.WithValue(
		context.Background(),
		testContextKey{},
		"create",
	)

	expectedParams := store.CreateAccountParams{
		UserID: "practice-user-022",
		Name:   "Practice User",
	}

	expectedAccount := store.Account{
		ID:      22,
		UserID:  expectedParams.UserID,
		Name:    expectedParams.Name,
		Status:  1,
		Version: 0,
	}

	fake := &fakeStore{
		createAccountFunc: func(
			gotCtx context.Context,
			arg store.CreateAccountParams,
		) (store.Account, error) {
			if got := gotCtx.Value(
				testContextKey{},
			); got != "create" {
				t.Fatalf(
					"context value = %v, want create",
					got,
				)
			}

			if !reflect.DeepEqual(
				arg,
				expectedParams,
			) {
				t.Fatalf(
					"CreateAccount params = %+v, want %+v",
					arg,
					expectedParams,
				)
			}

			return expectedAccount, nil
		},
	}

	service := NewService(fake)

	account, err := service.Create(
		ctx,
		CreateInput{
			UserID: "  practice-user-022  ",
			Name:   "  Practice User  ",
		},
	)
	if err != nil {
		t.Fatalf(
			"Create returned error: %v",
			err,
		)
	}

	if fake.createAccountCalls != 1 {
		t.Fatalf(
			"CreateAccount calls = %d, want 1",
			fake.createAccountCalls,
		)
	}

	if !reflect.DeepEqual(
		account,
		expectedAccount,
	) {
		t.Fatalf(
			"Create account = %+v, want %+v",
			account,
			expectedAccount,
		)
	}
}

func TestServiceCreateWrapsStoreError(
	t *testing.T,
) {
	storeErr := errors.New("insert failed")

	fake := &fakeStore{
		createAccountFunc: func(
			context.Context,
			store.CreateAccountParams,
		) (store.Account, error) {
			return store.Account{}, storeErr
		},
	}

	service := NewService(fake)

	account, err := service.Create(
		context.Background(),
		CreateInput{
			UserID: "practice-user-022",
			Name:   "Practice User",
		},
	)

	if !errors.Is(err, storeErr) {
		t.Fatalf(
			"Create error = %v, want wrapped %v",
			err,
			storeErr,
		)
	}

	if err == nil ||
		!strings.HasPrefix(
			err.Error(),
			"create account: ",
		) {
		t.Fatalf(
			"Create error = %v, want create account context",
			err,
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
}

func TestServiceGetByUserIDRejectsBlankUserIDWithoutStoreCall(
	t *testing.T,
) {
	fake := &fakeStore{}
	service := NewService(fake)

	_, err := service.GetByUserID(
		context.Background(),
		" \t ",
	)
	if !errors.Is(err, ErrUserIDRequired) {
		t.Fatalf(
			"GetByUserID error = %v, want %v",
			err,
			ErrUserIDRequired,
		)
	}

	if fake.getAccountByUserIDCalls != 0 {
		t.Fatalf(
			"GetAccountByUserID calls = %d, want 0",
			fake.getAccountByUserIDCalls,
		)
	}
}

func TestServiceGetByUserIDNormalizesAndDelegates(
	t *testing.T,
) {
	ctx := context.WithValue(
		context.Background(),
		testContextKey{},
		"get",
	)

	expectedAccount := store.Account{
		ID:      22,
		UserID:  "practice-user-022",
		Name:    "Practice User",
		Status:  1,
		Version: 0,
	}

	fake := &fakeStore{
		getAccountByUserIDFunc: func(
			gotCtx context.Context,
			userID string,
		) (store.Account, error) {
			if got := gotCtx.Value(
				testContextKey{},
			); got != "get" {
				t.Fatalf(
					"context value = %v, want get",
					got,
				)
			}

			if userID != expectedAccount.UserID {
				t.Fatalf(
					"userID = %q, want %q",
					userID,
					expectedAccount.UserID,
				)
			}

			return expectedAccount, nil
		},
	}

	service := NewService(fake)

	account, err := service.GetByUserID(
		ctx,
		"  practice-user-022  ",
	)
	if err != nil {
		t.Fatalf(
			"GetByUserID returned error: %v",
			err,
		)
	}

	if fake.getAccountByUserIDCalls != 1 {
		t.Fatalf(
			"GetAccountByUserID calls = %d, want 1",
			fake.getAccountByUserIDCalls,
		)
	}

	if !reflect.DeepEqual(
		account,
		expectedAccount,
	) {
		t.Fatalf(
			"GetByUserID account = %+v, want %+v",
			account,
			expectedAccount,
		)
	}
}

func TestServiceGetByUserIDMapsNoRowsToAccountNotFound(
	t *testing.T,
) {
	fake := &fakeStore{
		getAccountByUserIDFunc: func(
			context.Context,
			string,
		) (store.Account, error) {
			return store.Account{}, fmt.Errorf(
				"query account: %w",
				pgx.ErrNoRows,
			)
		},
	}

	service := NewService(fake)

	account, err := service.GetByUserID(
		context.Background(),
		"practice-user-missing",
	)

	if !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf(
			"GetByUserID error = %v, want %v",
			err,
			ErrAccountNotFound,
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
}

func TestServiceGetByUserIDWrapsStoreError(
	t *testing.T,
) {
	storeErr := errors.New("query failed")

	fake := &fakeStore{
		getAccountByUserIDFunc: func(
			context.Context,
			string,
		) (store.Account, error) {
			return store.Account{}, storeErr
		},
	}

	service := NewService(fake)

	account, err := service.GetByUserID(
		context.Background(),
		"practice-user-022",
	)

	if !errors.Is(err, storeErr) {
		t.Fatalf(
			"GetByUserID error = %v, want wrapped %v",
			err,
			storeErr,
		)
	}

	if err == nil ||
		!strings.HasPrefix(
			err.Error(),
			"get account by user id: ",
		) {
		t.Fatalf(
			"GetByUserID error = %v, want operation context",
			err,
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
}
```

Simpan file dengan satu newline setelah kurung kurawal terakhir.

## Mengapa memakai `package account`

Test memakai:

```go
package account
```

bukan `package account_test`. Dengan package yang sama, test berada di dalam package domain account dan dapat menguji kontrak package secara langsung. Pada source saat ini semua simbol yang dipakai memang exported, tetapi pilihan ini juga menjaga contoh tetap dekat dengan implementasi `Service` dan interface `Store` yang sedang dipelajari.

## Penjelasan setiap import

- `context`: membuat context test, menyimpan nilai penanda, dan memeriksa bahwa context diteruskan tanpa diganti.
- `errors`: membuat error palsu dan memeriksa error maupun rantai error dengan `errors.Is`.
- `fmt`: membungkus `pgx.ErrNoRows` untuk membuktikan bahwa service tetap mengenalinya di dalam rantai error.
- `reflect`: membandingkan struct parameter dan hasil secara lengkap memakai `reflect.DeepEqual`.
- `strings`: memeriksa prefix pesan error setelah service menambahkan konteks operasi.
- `testing`: menyediakan `*testing.T`, subtest, dan kegagalan test.
- `internal/store`: menyediakan `store.Account` dan `store.CreateAccountParams` yang dipakai kontrak store.
- `pgx/v5`: menyediakan sentinel `pgx.ErrNoRows` yang diterjemahkan domain menjadi `ErrAccountNotFound`.

## Struktur `fakeStore`

`fakeStore` mempunyai dua jenis field:

```go
createAccountFunc       func(...) (store.Account, error)
getAccountByUserIDFunc  func(...) (store.Account, error)
```

Kedua field ini adalah callback. Setiap test dapat memasang perilaku yang berbeda tanpa membuat tipe fake baru. Contohnya, satu test membuat callback mengembalikan account sukses, sementara test lain membuat callback mengembalikan error.

Field berikut adalah counter:

```go
createAccountCalls      int
getAccountByUserIDCalls int
```

Counter membuktikan berapa kali dependency dipanggil. Ini penting karena memeriksa error validasi saja belum cukup. Ketika input tidak valid, test juga harus memastikan service tidak meneruskan operasi ke persistence.

## Fake harus memenuhi seluruh interface

Interface `Store` dari `service.go` mempunyai dua method:

```text
CreateAccount
GetAccountByUserID
```

Karena itu, `fakeStore` harus mengimplementasikan **keduanya**, walaupun satu test hanya memakai salah satu method. Pemeriksaan compile-time berikut memastikan kontrak tersebut:

```go
var _ Store = (*fakeStore)(nil)
```

Baris ini tidak membuat object dan tidak memanggil method. Go hanya memeriksa apakah method set milik `*fakeStore` memenuhi `Store`.

Jika `CreateAccount` belum dibuat, compiler dapat memberi error seperti:

```text
cannot use (*fakeStore)(nil) (value of type *fakeStore) as Store value in variable declaration:
*fakeStore does not implement Store (missing method CreateAccount)
```

Artinya bukan `nil` yang salah. Artinya compiler sedang memakai nilai pointer bertipe `*fakeStore` untuk memeriksa interface, lalu menemukan method yang belum lengkap.

## Alur pemanggilan callback fake

Untuk operasi create, alurnya adalah:

```text
service.Create(...)
        |
        v
fakeStore.CreateAccount(...)
        |
        v
createAccountFunc(...)
        |
        v
hasil callback dikembalikan ke service
        |
        v
service mengembalikan hasil kepada test
```

Untuk operasi get, alurnya sama:

```text
service.GetByUserID(...)
        |
        v
fakeStore.GetAccountByUserID(...)
        |
        v
getAccountByUserIDFunc(...)
        |
        v
hasil callback dikembalikan ke service lalu ke test
```

Jadi fungsi yang diuji tetap `Service.Create` atau `Service.GetByUserID`. Method pada `fakeStore` berperan sebagai dependency palsu, sedangkan field callback menentukan respons dependency untuk skenario test tersebut.

Callback sengaja hanya dikonfigurasi ketika method tersebut boleh dipanggil. Jika callback masih `nil`, fake melakukan:

```go
panic("unexpected CreateAccount call")
```

atau:

```go
panic("unexpected GetAccountByUserID call")
```

Panic membuat pemanggilan dependency yang tidak diharapkan langsung terlihat sebagai test gagal. Contohnya, test validasi membuat `&fakeStore{}` tanpa callback. Jika service keliru memanggil store setelah validasi gagal, test tidak akan diam-diam lolos.

## Table-driven test dan subtest

`TestServiceRequiresConfiguredStore` serta `TestServiceCreateRejectsInvalidInputWithoutStoreCall` memakai pola table-driven:

```go
tests := []struct {
	// input dan expected result
}{
	// beberapa kasus
}

for _, test := range tests {
	t.Run(test.name, func(t *testing.T) {
		// satu skenario
	})
}
```

Keuntungannya, skenario yang strukturnya sama dapat ditambah sebagai data. `t.Run` membuat setiap kasus memiliki nama dan hasil sendiri, seperti `nil_receiver`, `nil_dependency`, `user_id_is_required`, dan `name_is_required`.

## Test nil receiver dan nil dependency

`TestServiceRequiresConfiguredStore` menguji dua keadaan:

1. receiver `*Service` sendiri bernilai `nil`;
2. object `Service` ada, tetapi dependency store yang diberikan ke `NewService` bernilai `nil`.

Baik `Create` maupun `GetByUserID` harus mengembalikan `ErrStoreNotConfigured`. Pemeriksaan memakai `errors.Is`, sehingga tetap aman jika suatu hari error tersebut dibungkus.

## Validasi harus menghasilkan nol pemanggilan store

`TestServiceCreateRejectsInvalidInputWithoutStoreCall` mengirim user ID kosong setelah trim dan nama kosong setelah trim. Selain memeriksa `ErrUserIDRequired` atau `ErrNameRequired`, test memastikan:

```go
fake.createAccountCalls == 0
```

`TestServiceGetByUserIDRejectsBlankUserIDWithoutStoreCall` melakukan pembuktian yang sama untuk:

```go
fake.getAccountByUserIDCalls == 0
```

Dengan demikian, test membuktikan urutan perilaku: service memvalidasi lebih dahulu, baru memanggil store jika input valid.

## Normalisasi, penerusan context, dan delegasi

Test sukses mengirim input yang mempunyai spasi di awal dan akhir:

```go
CreateInput{
	UserID: "  practice-user-022  ",
	Name:   "  Practice User  ",
}
```

Callback mengharapkan parameter setelah `strings.TrimSpace`:

```go
store.CreateAccountParams{
	UserID: "practice-user-022",
	Name:   "Practice User",
}
```

`reflect.DeepEqual` membandingkan seluruh field struct, bukan hanya satu field. Counter kemudian memastikan method dipanggil tepat satu kali.

Test juga memasukkan nilai ke context memakai `context.WithValue`. Callback membaca nilai tersebut dari `gotCtx`. Jika nilainya masih sama, terbukti service meneruskan context yang diterimanya ke store. Nilai context di sini hanya penanda test; pada aplikasi nyata context lebih umum membawa cancellation, deadline, dan metadata request.

## Dari `expectedAccount` sampai hasil service

Pada test create sukses, variable `expectedAccount` dibuat lebih dahulu. Callback fake lalu mengembalikannya:

```go
return expectedAccount, nil
```

Nilai itu berjalan melalui alur berikut:

```text
expectedAccount
      -> createAccountFunc
      -> fakeStore.CreateAccount
      -> Service.Create
      -> variable account di test
```

Terakhir, `reflect.DeepEqual(account, expectedAccount)` memastikan service mengembalikan hasil store tanpa mengubahnya. Pola yang sama digunakan oleh test `GetByUserID`.

Nama variable dalam file ini adalah `expectedAccount`, bukan `expectedResult`, tetapi maknanya sama: nilai tersebut adalah hasil yang disiapkan fake dan diharapkan keluar kembali dari fungsi yang diuji.

## Menguji wrapping error

Test create dan get error melakukan dua pemeriksaan yang berbeda:

```go
errors.Is(err, storeErr)
```

memastikan penyebab asli masih berada di error chain karena service memakai `%w`.

```go
strings.HasPrefix(err.Error(), "create account: ")
```

atau:

```go
strings.HasPrefix(err.Error(), "get account by user id: ")
```

memastikan service juga menambahkan konteks operasi yang berguna untuk log dan diagnosis. Memeriksa keduanya membuktikan bahwa error informatif tanpa kehilangan identitas error aslinya.

Ketika gagal, service harus mengembalikan zero value:

```go
store.Account{}
```

Test membandingkannya dengan `reflect.DeepEqual` agar tidak ada data parsial yang dianggap sukses.

## Menerjemahkan `pgx.ErrNoRows`

Fake tidak langsung mengembalikan `pgx.ErrNoRows`, melainkan membungkusnya:

```go
fmt.Errorf("query account: %w", pgx.ErrNoRows)
```

Ini membuktikan service memakai `errors.Is`, bukan perbandingan langsung `err == pgx.ErrNoRows`. Service masih menemukan `pgx.ErrNoRows` di dalam error chain lalu menerjemahkannya menjadi:

```go
ErrAccountNotFound
```

Hasil account pada kondisi not found juga harus berupa `store.Account{}`.

## Peringatan typed nil pada interface

Pemeriksaan dalam service adalah:

```go
if s == nil || s.store == nil
```

Ini menangani receiver nil dan interface store yang benar-benar nil. Namun, perhatikan kasus berikut:

```go
var fake *fakeStore
service := NewService(fake)
```

`fake` adalah pointer nil, tetapi ketika dimasukkan ke interface `Store`, interface tersebut membawa tipe `*fakeStore` dan value nil. Karena masih mempunyai informasi tipe, `s.store == nil` bernilai `false`.

Jika method dipanggil, `fakeStore.CreateAccount` mencoba mengakses `f.createAccountCalls` dan dapat panic karena receiver `f` nil. Karena itu, dependency sebaiknya tidak diberikan sebagai typed-nil pointer. Gunakan `NewService(nil)` untuk dependency yang memang tidak ada, atau berikan pointer ke instance nyata seperti `&fakeStore{...}`.

## Menjalankan test

Dari root server, rapikan file terlebih dahulu:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server
gofmt -w internal/domain/account/service_test.go
```

Jalankan package domain account dengan output verbose:

```bash
go test ./internal/domain/account -v
```

Kemudian jalankan seluruh test server:

```bash
go test ./...
```

Tidak perlu menjalankan Docker, tidak perlu menyalakan PostgreSQL, dan tidak perlu memberi `PRACTICE_RUN_POSTGRES_TESTS=1`. Integration test store dari Step 20 tetap dilewati pada test biasa sesuai guard yang sudah dibuat.

Representative output untuk package domain account:

```text
=== RUN   TestServiceRequiresConfiguredStore
=== RUN   TestServiceRequiresConfiguredStore/nil_receiver
=== RUN   TestServiceRequiresConfiguredStore/nil_dependency
--- PASS: TestServiceRequiresConfiguredStore (0.00s)
    --- PASS: TestServiceRequiresConfiguredStore/nil_receiver (0.00s)
    --- PASS: TestServiceRequiresConfiguredStore/nil_dependency (0.00s)
=== RUN   TestServiceCreateRejectsInvalidInputWithoutStoreCall
=== RUN   TestServiceCreateRejectsInvalidInputWithoutStoreCall/user_id_is_required
=== RUN   TestServiceCreateRejectsInvalidInputWithoutStoreCall/name_is_required
--- PASS: TestServiceCreateRejectsInvalidInputWithoutStoreCall (0.00s)
    --- PASS: TestServiceCreateRejectsInvalidInputWithoutStoreCall/user_id_is_required (0.00s)
    --- PASS: TestServiceCreateRejectsInvalidInputWithoutStoreCall/name_is_required (0.00s)
=== RUN   TestServiceCreateNormalizesAndDelegates
--- PASS: TestServiceCreateNormalizesAndDelegates (0.00s)
=== RUN   TestServiceCreateWrapsStoreError
--- PASS: TestServiceCreateWrapsStoreError (0.00s)
=== RUN   TestServiceGetByUserIDRejectsBlankUserIDWithoutStoreCall
--- PASS: TestServiceGetByUserIDRejectsBlankUserIDWithoutStoreCall (0.00s)
=== RUN   TestServiceGetByUserIDNormalizesAndDelegates
--- PASS: TestServiceGetByUserIDNormalizesAndDelegates (0.00s)
=== RUN   TestServiceGetByUserIDMapsNoRowsToAccountNotFound
--- PASS: TestServiceGetByUserIDMapsNoRowsToAccountNotFound (0.00s)
=== RUN   TestServiceGetByUserIDWrapsStoreError
--- PASS: TestServiceGetByUserIDWrapsStoreError (0.00s)
PASS
ok  github.com/imamsyafii-dev/bbge-practice/srv-server/internal/domain/account
```

Durasi dan penanda `(cached)` dapat berbeda. Hal yang penting adalah semua test berstatus `PASS` dan perintah `go test ./...` selesai tanpa kegagalan.

## Perubahan pada Step 22

File yang berubah:

```text
srv-server/internal/domain/account/service_test.go
learning-steps/01-foundation/05-domain-service/step-22-account-domain-service-test.md
```

File yang tidak perlu diubah:

```text
srv-server/internal/domain/account/service.go
srv-server/internal/store/*
srv-server/db/queries/*
srv-server/db/migrations/*
srv-server/main.go
srv-server/go.mod
srv-server/go.sum
```

Tidak ada generated file yang diedit dan tidak ada data yang dimasukkan ke PostgreSQL.

## Ringkasan pemahaman

Setelah Step 22:

- unit test menguji fungsi domain service pada layer tempat fungsi itu berada;
- interface memungkinkan dependency nyata diganti fake tanpa PostgreSQL;
- fake tetap wajib memenuhi seluruh method interface;
- callback mengatur respons fake secara khusus untuk setiap test;
- counter membuktikan jumlah pemanggilan dependency;
- validation, normalisasi, penerusan context, hasil sukses, error wrapping, dan error translation sudah diuji;
- nilai yang dikembalikan callback fake mengalir kembali melalui service ke test;
- interface yang berisi typed-nil pointer tidak sama dengan interface nil.

## Preview Step 23

Step 23 akan mulai membuat layer application/use case. Use case akan bergantung pada interface service yang sempit, hanya berisi operasi domain yang benar-benar dibutuhkan.

Alur berikutnya menjadi:

```text
caller
  |
  v
application account use case
  |
  v
narrow account service interface
  |
  v
domain account Service
  |
  v
store interface dan PostgreSQL
```

Interface di layer application bukan berarti seluruh fungsi domain harus dibuat ulang. Interface tersebut hanya mendeskripsikan method domain yang diperlukan use case. Pada unit test application nanti, domain service nyata dapat diganti fake service dengan cara yang sama seperti domain service pada Step 22 mengganti PostgreSQL dengan fake store.
