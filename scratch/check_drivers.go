package main

import (
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
)

func main() {
	fmt.Println("Registered drivers:", sql.Drivers())
}
