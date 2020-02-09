package internal

import (
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	_ "github.com/lib/pq"
	"github.com/mattn/go-sqlite3"
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

		gormdb, err = gorm.Open("sqlite3", DbPath)
		if err != nil {
			log.Fatal(err)
		}

		regex := func(re, s string) (bool, error) {
			b, e := regexp.MatchString(re, s)
			return b, e
		}

		sql.Register("sqlite3_with_regex",
			&sqlite3.SQLiteDriver{
				ConnectHook: func(conn *sqlite3.SQLiteConn) error {
					return conn.RegisterFunc("regexp", regex, true)
				},
			})

		DbPath = fmt.Sprintf("file:%v?cache=shared&mode=rwc", DbPath)
		DB, err = sql.Open("sqlite3_with_regex", DbPath)
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
	gormdb.Model(&Command{}).AddIndex("idx_exit_command", "exit_status, command")

	// just need gorm for migration.
	gormdb.Close()
}

func (user User) userExists() bool {
	var exists bool
	err := DB.QueryRow("SELECT exists (select id FROM users WHERE username = $1 AND password = $2)",
		user.Username, user.Password).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		log.Fatalf("error checking if row exists %v", err)
	}
	return exists
}

func (user User) userCreate() int64 {
	res, err := DB.Exec(`INSERT INTO users("registration_code", "username","password","email")
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
	_, err := DB.Exec(`UPDATE users SET "token" = $1 WHERE "username" = $2 `, user.Token, user.Username)
	if err != nil {
		log.Fatal(err)
	}
}

func (user User) tokenExists() bool {
	var exists bool
	err := DB.QueryRow("SELECT exists (select id FROM users WHERE token = $1)",
		user.Token).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		log.Fatalf("error checking if row exists %v", err)
	}
	return exists
}

func (cmd Command) commandInsert() int64 {
	res, err := DB.Exec(`INSERT INTO commands("process_id","process_start_time","exit_status","uuid", "command", "created", "path", "user_id")
 							 VALUES ($1,$2,$3,$4,$5,$6,$7,(select "id" FROM users WHERE "token" = $8))`,
		cmd.ProcessId, cmd.ProcessStartTime, cmd.ExitStatus, cmd.Uuid, cmd.Command, cmd.Created, cmd.Path, cmd.Token)
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
	if cmd.Unique || cmd.Query != "" {
		//postgres
		if connectionLimit != 1 {
			if cmd.Path != "" && cmd.Query != "" && cmd.Unique {
				rows, err = DB.Query(`SELECT * FROM  ( 
										    SELECT DISTINCT ON ("command") command, "uuid", "created"
										    FROM commands
										   	WHERE "user_id" IN (SELECT "id" FROM users WHERE "token" = $1)
										   	AND ("exit_status" = 0 OR "exit_status" = 130) 
										   	AND "command" not like 'bh%'  
										   	AND "path" = $3 
										   	AND "command" ~ $4
										    ) c
										ORDER BY "created" DESC limit $2;`, cmd.Token, cmd.Limit, cmd.Path, cmd.Query)

			} else if cmd.Path != "" && cmd.Query != "" {
				rows, err = DB.Query(`SELECT "command", "uuid", "created"
										    FROM commands
										   	WHERE "user_id" IN (SELECT "id" FROM users WHERE "token" = $1)
										   	AND ("exit_status" = 0 OR "exit_status" = 130) 
										   	AND "command" not like 'bh%'  
										   	AND "path" = $3 
										   	AND "command" ~ $4
											ORDER BY "created" DESC limit $2;`, cmd.Token, cmd.Limit, cmd.Path, cmd.Query)

			} else if cmd.Path != "" && cmd.Unique {
				rows, err = DB.Query(`SELECT * FROM  ( 
										    SELECT DISTINCT ON ("command") command, "uuid", "created"
										    FROM commands
										   	WHERE "user_id" IN (SELECT "id" FROM users WHERE "token" = $1)
										   	AND ("exit_status" = 0 OR "exit_status" = 130) 
										   	AND "command" not like 'bh%'  
										   	AND "path" = $3
										    ) c
										ORDER BY "created" DESC limit $2;`, cmd.Token, cmd.Limit, cmd.Path)

			} else if cmd.Query != "" && cmd.Unique {
				rows, err = DB.Query(`SELECT * FROM  ( 
										    SELECT DISTINCT ON ("command") command, "uuid", "created"
										    FROM commands
										   	WHERE "user_id" IN (SELECT "id" FROM users WHERE "token" = $1)
										   	AND ("exit_status" = 0 OR "exit_status" = 130) 
										   	AND "command" not like 'bh%'  
										   	AND "command" ~ $3
										    ) c
										ORDER BY "created" DESC limit $2;`, cmd.Token, cmd.Limit, cmd.Query)

			} else if cmd.Query != "" {
				rows, err = DB.Query(`SELECT "command", "uuid", "created"
										    FROM commands
										   	WHERE "user_id" IN (SELECT "id" FROM users WHERE "token" = $1)
										   	AND ("exit_status" = 0 OR "exit_status" = 130) 
										   	AND "command" not like 'bh%'  
										   	AND "command" ~ $3
										ORDER BY "created" DESC limit $2;`, cmd.Token, cmd.Limit, cmd.Query)

			} else {
				// unique
				rows, err = DB.Query(`SELECT * FROM  ( 
										    SELECT DISTINCT ON ("command") command, "uuid", "created"
										    FROM commands
										   	WHERE "user_id" IN (SELECT "id" FROM users WHERE "token" = $1)
										   	AND ("exit_status" = 0 OR "exit_status" = 130) 
										    AND "command" not like 'bh%'   
										    ) c
									  ORDER BY "created" DESC limit $2;`, cmd.Token, cmd.Limit)
			}
		} else {
			// sqlite
			if cmd.Path != "" && cmd.Query != "" && cmd.Unique {
				query := fmt.Sprintf(`SELECT "command",  "uuid", "created" FROM commands
                                     WHERE "user_id" IN (select "id" FROM users WHERE "token" = '%v')
								     AND ("exit_status" = 0 OR "exit_status" = 130) 
								     AND "command" not like '%v'  
								     AND "path" = '%v' 
								     AND "command" regexp '%v'
								     GROUP BY "command" ORDER  BY  "created" DESC limit '%v'`,
								     cmd.Token, "bh%", cmd.Path, cmd.Query, cmd.Limit)
				rows, err = DB.Query(query)
			} else if cmd.Path != "" && cmd.Query != "" {
				query := fmt.Sprintf(`SELECT "command",  "uuid", "created" FROM commands
									 WHERE "user_id" IN (select "id" FROM users WHERE "token" = '%v') 
									 AND ("exit_status" = 0 OR "exit_status" = 130) 
					                 AND "command" not like '%v'  
					                 AND "path" = %v' 
					                 AND "command" regexp %v'
									 ORDER  BY  "created" DESC limit '%v'`,
									 cmd.Token, "bh%", cmd.Path, cmd.Query, cmd.Limit)

				rows, err = DB.Query(query)

			} else if cmd.Path != "" && cmd.Unique {
				rows, err = DB.Query(`SELECT "command",  "uuid", "created" FROM commands
									  WHERE "user_id" IN (select "id" FROM users WHERE "token" = $1) 
									  AND ("exit_status" = 0 OR "exit_status" = 130) 
									  AND "command" not like 'bh%'  
									  AND "path" = $2
									  GROUP BY "command" ORDER  BY  "created" DESC limit $3`,
									  cmd.Token, cmd.Path, cmd.Limit)

			} else if cmd.Query != "" && cmd.Unique {
				query := fmt.Sprintf(`SELECT "command", "uuid", "created" FROM commands
								     WHERE "user_id" IN (select "id" FROM users WHERE "token" = '%v') 
								     AND ("exit_status" = 0 OR "exit_status" = 130) 
                				     AND "command" not like '%v'  
                				     AND "command" regexp '%v'  
								     GROUP BY "command" ORDER  BY "created" DESC limit '%v'`,
								     cmd.Token, "bh%", cmd.Query, cmd.Limit)
				rows, err = DB.Query(query)

			} else if cmd.Query != "" {
				query := fmt.Sprintf(`SELECT "command",  "uuid", "created" FROM commands
									 WHERE "user_id" IN (select "id" FROM users WHERE "token" = '%v') 
									 AND ("exit_status" = 0 OR "exit_status" = 130) 
									 AND "command" not like '%v'  
									 AND "command" regexp'%v'
									 ORDER  BY  "created" DESC limit '%v'`,
									 cmd.Token, "bh%", cmd.Query, cmd.Limit)
				rows, err = DB.Query(query)

			} else {
				// unique
				rows, err = DB.Query(`SELECT "command", "uuid", "created"
										    FROM commands
										   	WHERE "user_id" IN (SELECT "id" FROM users WHERE "token" = $1)
										   	AND ("exit_status" = 0 OR "exit_status" = 130) 
										   	AND "command" not like 'bh%'  
										GROUP BY "command"  ORDER BY "created" DESC limit $2;`, cmd.Token, cmd.Limit)
			}
		}
	} else {
		if cmd.Path != "" {
			rows, err = DB.Query(`SELECT "command",  "uuid", "created" FROM commands
								 WHERE "user_id" IN (select "id" FROM users WHERE "token" = $1) AND "path" = $3
								 AND ("exit_status" = 0 OR "exit_status" = 130) AND "command" not like 'bh%'  
								 ORDER  BY  "created" DESC limit $2`, cmd.Token, cmd.Limit, cmd.Path)
		} else {
			rows, err = DB.Query(`SELECT "command",  "uuid", "created" FROM commands
								 WHERE "user_id" IN (select "id" FROM users WHERE "token" = $1)
								 AND ("exit_status" = 0 OR "exit_status" = 130) AND "command" not like 'bh%'  
								 ORDER  BY  "created" DESC limit $2`, cmd.Token, cmd.Limit)
		}

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
	res, err := DB.Exec(`INSERT INTO systems ("name", "mac", "user_id", "hostname", "client_version", "created", "updated")
 									  VALUES ($1, $2, (select "id" FROM users WHERE "token" = $3), $4, $5, $6, $7)`,
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
 									  "id", "created", "updated" FROM systems WHERE mac = $1`,
		sys.Mac).Scan(&row)
	if err != nil {
		return SystemQuery{}, err
	}
	return row, nil

}
