# Step 2 — Membuat entrypoint server

Buat file `srv-server/main.go`:

```go
package main

import "fmt"

func main() {
	fmt.Println("practice server starting")
}
```

Jalankan:

```bash
cd /Users/mac/Documents/bbge/practice/srv-server

gofmt -w main.go
go run .
go test ./...
```

Hasil `go run .` yang diharapkan:

```text
practice server starting
```

Hasil `go test ./...` kurang lebih:

```text
?    github.com/imamsyafii-dev/bbge-practice/srv-server    [no test files]
```
