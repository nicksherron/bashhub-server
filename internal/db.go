/*
 *
 * Copyright © 2020 nicksherron <nsherron90@gmail.com>
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package internal

import (
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
	// db driver are called by gorm
	_ "github.com/jinzhu/gorm/dialects/postgres"
	// db driver are called by gorm
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	// db driver are called by database/sql
	_ "github.com/lib/pq"
	"github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

var (
	db *sql.DB
	// DbPath is the postgres connection uri or the sqlite db file location to use for backend.
	DbPath          string
	connectionLimit int
)

// DbInit initializes our db.
func dbInit() {
	var gormdb *gorm.DB
	var err error
	if strings.HasPrefix(DbPath, "postgres://") {
		// postgres

		db, err = sql.Open("postgres", DbPath)
		if err != nil {
			log.Fatal(err)
		}

		gormdb, err = gorm.Open("postgres", DbPath)
		if err != nil {
			log.Fatal(err)
		}
		connectionLimit = 50
	} else {
		// sqlite
		gormdb, err = gorm.Open("sqlite3", DbPath)
		if err != nil {
			log.Fatal(err)
		}
		// sqlite regex function
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
		db, err = sql.Open("sqlite3_with_regex", DbPath)
		if err != nil {
			log.Fatal(err)
		}

		db.Exec("PRAGMA journal_mode=WAL;")
		connectionLimit = 1

	}
	db.SetMaxOpenConns(connectionLimit)
	gormdb.AutoMigrate(&User{})
	gormdb.AutoMigrate(&Command{})
	gormdb.AutoMigrate(&System{})
	//TODO: ensure these are the most efficient indexes
	gormdb.Model(&User{}).AddIndex("idx_user", "username")
	gormdb.Model(&System{}).AddIndex("idx_mac", "mac")
	gormdb.Model(&Command{}).AddIndex("idx_exit_command_created", "exit_status, created, command")
	gormdb.Model(&Command{}).AddIndex("idx_user_exit_command_created", "user_id, exit_status, created, command")
	// Just need gorm for migration and index creation.
	gormdb.Close()
}

func hashAndSalt(password string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		log.Println(err)
	}
	return string(hash)
}

func comparePasswords(hashedPwd string, plainPwd string) bool {
	byteHash := []byte(hashedPwd)
	err := bcrypt.CompareHashAndPassword(byteHash, []byte(plainPwd))
	if err != nil {
		log.Println(err)
		return false
	}
	return true
}

func (user User) userExists() bool {
	var password string
	err := db.QueryRow("SELECT password FROM users WHERE username = $1",
		user.Username).Scan(&password)
	if err != nil && err != sql.ErrNoRows {
		log.Fatalf("error checking if row exists %v", err)
	}
	if password != "" {
		return comparePasswords(password, user.Password)
	}
	return false
}

func (user User) userGetId() uint {
	var id uint
	err := db.QueryRow("SELECT id FROM users WHERE username = $1",
		user.Username).Scan(&id)
	if err != nil && err != sql.ErrNoRows {
		log.Fatalf("error checking if row exists %v", err)
	}
	return id
}

func (user User) userGetSystemName() string {
	var systemName string
	err := db.QueryRow(`SELECT name 
							FROM systems 
							WHERE user_id in (select id from users where username = $1)
							AND mac = $2`,
		user.Username, user.Mac).Scan(&systemName)
	if err != nil && err != sql.ErrNoRows {
		log.Fatalf("error checking if row exists %v", err)
	}
	return systemName
}

func (user User) usernameExists() bool {
	var exists bool
	err := db.QueryRow(`SELECT exists (select id FROM users WHERE "username" = $1)`,
		user.Username).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		log.Fatalf("error checking if row exists %v", err)
	}
	return exists
}

func (user User) emailExists() bool {
	var exists bool
	err := db.QueryRow(`SELECT exists (select id FROM users WHERE "email" = $1)`,
		user.Email).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		log.Fatalf("error checking if row exists %v", err)
	}
	return exists
}

func (user User) userCreate() int64 {
	user.Password = hashAndSalt(user.Password)
	res, err := db.Exec(`INSERT INTO users("registration_code", "username","password","email")
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

func (cmd Command) commandInsert() int64 {
	res, err := db.Exec(`INSERT INTO commands("process_id","process_start_time","exit_status","uuid", "command", "created", "path", "user_id", "system_name")
 							 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		cmd.ProcessId, cmd.ProcessStartTime, cmd.ExitStatus, cmd.Uuid, cmd.Command, cmd.Created, cmd.Path, cmd.User.ID, cmd.SystemName)
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
			if cmd.SystemName != "" && cmd.Path != "" && cmd.Query != "" && cmd.Unique {
				rows, err = db.Query(`SELECT * FROM  ( 
										    SELECT DISTINCT ON ("command") command, "uuid", "created"
										    FROM commands
										   	WHERE  "user_id" = $1
										   	AND ("exit_status" = 0 OR "exit_status" = 130) 
										   	AND "command" not like 'bh%'  
										   	AND "path" = $3 
										   	AND "system_name" = $4								
										   	AND "command" ~ $5
										    ) c
										ORDER BY "created" DESC limit $2;`, cmd.User.ID, cmd.Limit, cmd.Path, cmd.SystemName, cmd.Query)

			} else if cmd.Path != "" && cmd.Query != "" && cmd.Unique {
				rows, err = db.Query(`SELECT * FROM  ( 
										    SELECT DISTINCT ON ("command") command, "uuid", "created"
										    FROM commands
										   	WHERE  "user_id" = $1
										   	AND ("exit_status" = 0 OR "exit_status" = 130) 
										   	AND "command" not like 'bh%'  
										   	AND "path" = $3 
										   	AND "command" ~ $4
										    ) c
										ORDER BY "created" DESC limit $2;`, cmd.User.ID, cmd.Limit, cmd.Path, cmd.Query)

			} else if cmd.SystemName != "" && cmd.Query != "" {
				rows, err = db.Query(`SELECT "command", "uuid", "created"
										    FROM commands
										   	WHERE  "user_id" = $1
										   	AND ("exit_status" = 0 OR "exit_status" = 130) 
										   	AND "command" not like 'bh%'  
										   	AND "system_name" = $3 
										   	AND "command" ~ $4
											ORDER BY "created" DESC limit $2;`, cmd.User.ID, cmd.Limit, cmd.SystemName, cmd.Query)

			} else if cmd.Path != "" && cmd.Query != "" {
				rows, err = db.Query(`SELECT "command", "uuid", "created"
										    FROM commands
										   	WHERE  "user_id" = $1
										   	AND ("exit_status" = 0 OR "exit_status" = 130) 
										   	AND "command" not like 'bh%'  
										   	AND "path" = $3 
										   	AND "command" ~ $4
											ORDER BY "created" DESC limit $2;`, cmd.User.ID, cmd.Limit, cmd.Path, cmd.Query)

			} else if cmd.SystemName != "" && cmd.Unique {
				rows, err = db.Query(`SELECT * FROM  ( 
										    SELECT DISTINCT ON ("command") command, "uuid", "created"
										    FROM commands
										   	WHERE  "user_id" = $1
										   	AND ("exit_status" = 0 OR "exit_status" = 130) 
										   	AND "command" not like 'bh%'  
										   	AND "system_name" = $3 
										    ) c
										ORDER BY "created" DESC limit $2;`, cmd.User.ID, cmd.Limit, cmd.SystemName)

			} else if cmd.Path != "" && cmd.Unique {
				rows, err = db.Query(`SELECT * FROM  ( 
										    SELECT DISTINCT ON ("command") command, "uuid", "created"
										    FROM commands
										   	WHERE  "user_id" = $1
										   	AND ("exit_status" = 0 OR "exit_status" = 130) 
										   	AND "command" not like 'bh%'  
										   	AND "path" = $3
										    ) c
										ORDER BY "created" DESC limit $2;`, cmd.User.ID, cmd.Limit, cmd.Path)

			} else if cmd.Query != "" && cmd.Unique {
				rows, err = db.Query(`SELECT * FROM  ( 
										    SELECT DISTINCT ON ("command") command, "uuid", "created"
										    FROM commands
										   	WHERE  "user_id" = $1
										   	AND ("exit_status" = 0 OR "exit_status" = 130) 
										   	AND "command" not like 'bh%'  
										   	AND "command" ~ $3
										    ) c
										ORDER BY "created" DESC limit $2;`, cmd.User.ID, cmd.Limit, cmd.Query)

			} else if cmd.Query != "" {
				rows, err = db.Query(`SELECT "command", "uuid", "created"
										    FROM commands
										   	WHERE  "user_id" = $1
										   	AND ("exit_status" = 0 OR "exit_status" = 130) 
										   	AND "command" not like 'bh%'  
										   	AND "command" ~ $3
										ORDER BY "created" DESC limit $2;`, cmd.User.ID, cmd.Limit, cmd.Query)

			} else {
				// unique
				rows, err = db.Query(`SELECT * FROM  ( 
										    SELECT DISTINCT ON ("command") command, "uuid", "created"
										    FROM commands
										   	WHERE  "user_id" = $1
										   	AND ("exit_status" = 0 OR "exit_status" = 130) 
										    AND "command" not like 'bh%'   
										    ) c
									  ORDER BY "created" DESC limit $2;`, cmd.User.ID, cmd.Limit)
			}
		} else {
			// sqlite
			if cmd.SystemName != "" && cmd.Path != "" && cmd.Query != "" && cmd.Unique {
				// Have to use fmt.Sprintf to build queries where sqlite regexp function is used because of single quotes. Haven't found any other work around.
				query := fmt.Sprintf(`SELECT "command",  "uuid", "created" FROM commands
                                     WHERE "user_id" =  '%v'
								     AND ("exit_status" = 0 OR "exit_status" = 130) 
								     AND "command" not like '%v'  
								     AND "path" = '%v'
								     AND "system_name" = '%v'
								     AND "command" regexp '%v'
								     GROUP BY "command" ORDER  BY  "created" DESC limit '%v'`,
					cmd.User.ID, "bh%", cmd.Path, cmd.SystemName, cmd.Query, cmd.Limit)

				rows, err = db.Query(query)

			} else if cmd.SystemName != "" && cmd.Query != "" && cmd.Unique {
				query := fmt.Sprintf(`SELECT "command",  "uuid", "created" FROM commands
                                     WHERE "user_id" =  '%v'
								     AND ("exit_status" = 0 OR "exit_status" = 130) 
								     AND "command" not like '%v'  
								     AND "system_name" = '%v' 
								     AND "command" regexp '%v'
								     GROUP BY "command" ORDER  BY  "created" DESC limit '%v'`,
					cmd.User.ID, "bh%", cmd.SystemName, cmd.Query, cmd.Limit)

				rows, err = db.Query(query)

			} else if cmd.Path != "" && cmd.Query != "" && cmd.Unique {
				query := fmt.Sprintf(`SELECT "command",  "uuid", "created" FROM commands
                                     WHERE "user_id" =  '%v'
								     AND ("exit_status" = 0 OR "exit_status" = 130) 
								     AND "command" not like '%v'  
								     AND "path" = '%v' 
								     AND "command" regexp '%v'
								     GROUP BY "command" ORDER  BY  "created" DESC limit '%v'`,
					cmd.User.ID, "bh%", cmd.Path, cmd.Query, cmd.Limit)

				rows, err = db.Query(query)

			} else if cmd.SystemName != "" && cmd.Query != "" {
				query := fmt.Sprintf(`SELECT "command",  "uuid", "created" FROM commands
									 WHERE "user_id" =  '%v' 
									 AND ("exit_status" = 0 OR "exit_status" = 130) 
					                 AND "command" not like '%v'  
					                 AND "system_name" = %v' 
					                 AND "command" regexp %v'
									 ORDER  BY  "created" DESC limit '%v'`,
					cmd.User.ID, "bh%", cmd.SystemName, cmd.Query, cmd.Limit)

				rows, err = db.Query(query)

			} else if cmd.Path != "" && cmd.Query != "" {
				query := fmt.Sprintf(`SELECT "command",  "uuid", "created" FROM commands
									 WHERE "user_id" =  '%v' 
									 AND ("exit_status" = 0 OR "exit_status" = 130) 
					                 AND "command" not like '%v'  
					                 AND "path" = %v' 
					                 AND "command" regexp %v'
									 ORDER  BY  "created" DESC limit '%v'`,
					cmd.User.ID, "bh%", cmd.Path, cmd.Query, cmd.Limit)

				rows, err = db.Query(query)

			} else if cmd.SystemName != "" && cmd.Unique {
				rows, err = db.Query(`SELECT "command",  "uuid", "created" FROM commands
									  WHERE  "user_id" = $1 
									  AND ("exit_status" = 0 OR "exit_status" = 130) 
									  AND "command" not like 'bh%'  
									  AND "system_name" = $2
									  GROUP BY "command" ORDER  BY  "created" DESC limit $3`,
					cmd.User.ID, cmd.SystemName, cmd.Limit)

			} else if cmd.Path != "" && cmd.Unique {
				rows, err = db.Query(`SELECT "command",  "uuid", "created" FROM commands
									  WHERE  "user_id" = $1 
									  AND ("exit_status" = 0 OR "exit_status" = 130) 
									  AND "command" not like 'bh%'  
									  AND "path" = $2
									  GROUP BY "command" ORDER  BY  "created" DESC limit $3`,
					cmd.User.ID, cmd.Path, cmd.Limit)

			} else if cmd.Query != "" && cmd.Unique {
				query := fmt.Sprintf(`SELECT "command", "uuid", "created" FROM commands
								     WHERE "user_id" =  '%v' 
								     AND ("exit_status" = 0 OR "exit_status" = 130) 
                				     AND "command" not like '%v'  
                				     AND "command" regexp '%v'  
								     GROUP BY "command" ORDER  BY "created" DESC limit '%v'`,
					cmd.User.ID, "bh%", cmd.Query, cmd.Limit)

				rows, err = db.Query(query)

			} else if cmd.Query != "" {
				query := fmt.Sprintf(`SELECT "command",  "uuid", "created" FROM commands
									 WHERE "user_id" =  '%v' 
									 AND ("exit_status" = 0 OR "exit_status" = 130) 
									 AND "command" not like '%v'  
									 AND "command" regexp'%v'
									 ORDER  BY  "created" DESC limit '%v'`,
					cmd.User.ID, "bh%", cmd.Query, cmd.Limit)

				rows, err = db.Query(query)

			} else {
				// unique
				rows, err = db.Query(`SELECT "command", "uuid", "created"
										    FROM commands
										   	WHERE  "user_id" = $1
										   	AND ("exit_status" = 0 OR "exit_status" = 130) 
										   	AND "command" not like 'bh%'  
										GROUP BY "command"  ORDER BY "created" DESC limit $2;`, cmd.User.ID, cmd.Limit)
			}
		}
	} else {
		if cmd.Path != "" {
			rows, err = db.Query(`SELECT "command",  "uuid", "created" FROM commands
								 WHERE  "user_id" = $1 
								 AND "path" = $3
								 AND ("exit_status" = 0 OR "exit_status" = 130) 
								 AND "command" not like 'bh%'  
								 ORDER  BY  "created" DESC limit $2`, cmd.User.ID, cmd.Limit, cmd.Path)
		} else if cmd.SystemName != "" {
			rows, err = db.Query(`SELECT "command",  "uuid", "created" FROM commands
								 WHERE  "user_id" = $1 
								 AND "system_name" = $3
								 AND ("exit_status" = 0 OR "exit_status" = 130) AND "command" not like 'bh%'  
								 ORDER  BY  "created" DESC limit $2`, cmd.User.ID, cmd.Limit, cmd.SystemName)
		} else {
			rows, err = db.Query(`SELECT "command",  "uuid", "created" FROM commands
								 WHERE  "user_id" = $1
								 AND ("exit_status" = 0 OR "exit_status" = 130) 
								 AND "command" not like 'bh%'  
								 ORDER  BY  "created" DESC limit $2`, cmd.User.ID, cmd.Limit)
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

func (cmd Command) commandGetUUID() Query {
	var result Query
	err := db.QueryRow(`SELECT "command","path", "created" , "uuid", "exit_status", "system_name" 
								 FROM commands
								 WHERE "uuid" = $1 
								 AND "user_id" = $2`, cmd.Uuid, cmd.User.ID).Scan(
		&result.Command, &result.Path, &result.Created, &result.Uuid,
		&result.ExitStatus, &result.SystemName)
	if err != nil {
		log.Println(err)
	}
	return result
}

func (sys System) systemInsert() int64 {

	t := time.Now().Unix()
	res, err := db.Exec(`INSERT INTO systems ("name", "mac", "user_id", "hostname", "client_version", "created", "updated")
 									  VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		sys.Name, sys.Mac, sys.User.ID, sys.Hostname, sys.ClientVersion, t, t)
	if err != nil {
		log.Fatal(err)
	}
	inserted, err := res.RowsAffected()
	if err != nil {
		log.Fatal(err)
	}
	return inserted
}

func (sys System) systemGet() System {
	var row System
	err := db.QueryRow(`SELECT "name", "mac", "user_id", "hostname", "client_version",
 									  "id", "created", "updated" FROM systems 
 							  WHERE  "user_id" $1
 							  AND "mac" = $2`,
		sys.User.ID, sys.Mac).Scan(&row)
	if err != nil {
		return System{}
	}
	return row

}
