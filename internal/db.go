package internal

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	_ "github.com/lib/pq"
)

var (
	DB              *sql.DB
	DbPath          string
	connectionLimit int
)

// DbInit initializes our db.
func DbInit() {
	// GormDB contains DB connection state
	var gormdb *gorm.DB

	var err error
	if strings.HasPrefix(DbPath, "postgres://") {
		//
		DB, err = sql.Open("postgres", DbPath)
		if err != nil {
			log.Fatal(err)
		}

		gormdb, err = gorm.Open("postgres", DbPath)
		if err != nil {
			log.Fatal(err)
		}
		connectionLimit = 50
	} else {
		DbPath = fmt.Sprintf("file:%v?cache=shared&mode=rwc", DbPath)
		DB, err = sql.Open("sqlite3", DbPath)
		if err != nil {
			log.Fatal(err)
		}
		gormdb, err = gorm.Open("sqlite3", DbPath)
		if err != nil {
			log.Fatal(err)
		}
		DB.Exec("PRAGMA journal_mode=WAL;")
		connectionLimit = 1

	}
	DB.SetMaxOpenConns(connectionLimit)
	gormdb.AutoMigrate(&User{})
	gormdb.AutoMigrate(&Command{})
	gormdb.AutoMigrate(&System{})
	gormdb.Model(&User{}).AddIndex("idx_user", "username")
	gormdb.Model(&User{}).AddIndex("idx_token", "token")
	gormdb.Model(&System{}).AddIndex("idx_mac", "mac")

	// just need gorm for migration.
	gormdb.Close()
}

func (user User) userExists() bool {
	var exists bool
	err := DB.QueryRow("SELECT exists (select id from users where username = $1 and password = $2)",
		user.Username, user.Password).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		log.Fatalf("error checking if row exists %v", err)
	}
	return exists
}

func (user User) userCreate() int64 {
	res, err := DB.Exec(`INSERT into users("registration_code", "username","password","email")
 							 VALUES ($1,$2,$3,$4) ON CONFLICT(username) do nothing`, user.RegistrationCode,
		user.Username, user.Password, user.Email)
	if err != nil {
		log.Fatal(err)
	}
	inserted, err := res.RowsAffected()
	if err != nil {
		log.Fatal(err)
	}
	return inserted
}

func (user User) updateToken() {
	_, err := DB.Exec(`UPDATE users set "token" = $1 where "username" = $2 `, user.Token, user.Username)
	if err != nil {
		log.Fatal(err)
	}
}

func (user User) tokenExists() bool {
	var exists bool
	err := DB.QueryRow("SELECT exists (select id from users where token = $1)",
		user.Token).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		log.Fatalf("error checking if row exists %v", err)
	}
	return exists
}

func (cmd Command) commandInsert() int64 {
	res, err := DB.Exec(`INSERT into commands("uuid", "command", "created", "user_id")
 							 VALUES ($1,$2,$3,(select "id" from users where "token" = $4))`,
		cmd.Uuid, cmd.Command, cmd.Created, cmd.Token)
	if err != nil {
		log.Fatal(err)
	}
	inserted, err := res.RowsAffected()
	if err != nil {
		log.Fatal(err)
	}
	return inserted
}

func (cmd Command) commandGet() []Query {
	var results []Query
	var rows *sql.Rows
	var err error
	if cmd.Unique {
		if connectionLimit != 1 {
			// postgres
			rows, err = DB.Query(`SELECT DISTINCT on ("command") command,  "uuid", "created" from commands
								 where "user_id" in (select "id" from users where "token" = $1) 
								  order by "command", "created" desc limit $2`, cmd.Token, cmd.Limit)
		}else {
			// sqlite
			rows, err = DB.Query(`SELECT "command",  "uuid", "created" from commands
								 where "user_id" in (select "id" from users where "token" = $1)
								 group by "command" order by "created" desc limit $2`, cmd.Token, cmd.Limit)
		}
	} else {
		rows, err = DB.Query(`SELECT "command",  "uuid", "created" from commands
								 where "user_id" in (select "id" from users where "token" = $1)
								 order  by "created" desc  limit $2`, cmd.Token, cmd.Limit)
	}

	if err != nil {
		log.Println(err)
	}
	defer rows.Close()
	for rows.Next() {
		var result Query
		err = rows.Scan(&result.Command, &result.Uuid, &result.Created)
		if err != nil {
			log.Println(err)
		}
		results = append(results, result)
	}

	return results

}

func (sys System) systemInsert() int64 {

	t := time.Now().Unix()
	res, err := DB.Exec(`INSERT into systems ("name", "mac", "user_id", "hostname", "client_version", "created", "updated")
 									  VALUES ($1, $2, (select "id" from users where "token" = $3), $4, $5, $6, $7)`,
		sys.Name, sys.Mac, sys.Token, sys.Hostname, sys.ClientVersion, t, t)
	if err != nil {
		log.Fatal(err)
	}
	inserted, err := res.RowsAffected()
	if err != nil {
		log.Fatal(err)
	}
	return inserted
}

func (sys System) systemGet() (SystemQuery, error) {
	var row SystemQuery
	err := DB.QueryRow(`SELECT "name", "mac", "user_id", "hostname", "client_version",
 									  "id", "created", "updated" from systems where mac = $1`,
		sys.Mac).Scan(&row)
	if err != nil {
		return SystemQuery{}, err
	}
	return row, nil

}

//SELECT DISTINCT on ("command"), "uuid", "created" from commands
//where "user_id" in (select "id" from users where "token" = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJJZCI6Im5pY')
//desc limit 1;