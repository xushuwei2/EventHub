module github.com/eventhub/eventhub/tests

go 1.23.0

replace github.com/eventhub/eventhub => ../src

require github.com/eventhub/eventhub v0.0.0

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/go-sql-driver/mysql v1.9.2 // indirect
)
