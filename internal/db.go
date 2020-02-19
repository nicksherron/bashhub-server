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
	db              *sql.DB
	connectionLimit int
)

func dbInit(dbPath string) {
	var gormdb *gorm.DB
	var err error
	if strings.HasPrefix(dbPath, "postgres://") {
		// postgres

		db, err = sql.Open("postgres", dbPath)
		if err != nil {
			log.Fatal(err)
		}

		gormdb, err = gorm.Open("postgres", dbPath)
		if err != nil {
			log.Fatal(err)
		}
		connectionLimit = 50
	} else {
		// sqlite
		gormdb, err = gorm.Open("sqlite3", dbPath)
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

		dbPath = fmt.Sprintf("file:%v?cache=shared&mode=rwc&_loc=auto", dbPath)
		db, err = sql.Open("sqlite3_with_regex", dbPath)
		if err != nil {
			log.Fatal(err)
		}

		_, err = db.Exec("PRAGMA journal_mode=WAL;")
		if err != nil {
			log.Fatal(err)
		}
		connectionLimit = 1

	}
	db.SetMaxOpenConns(connectionLimit)
	gormdb.AutoMigrate(&User{})
	gormdb.AutoMigrate(&Command{})
	gormdb.AutoMigrate(&System{})
	gormdb.AutoMigrate(&Config{})

	//TODO: ensure these are the most efficient indexes
	gormdb.Model(&User{}).AddUniqueIndex("idx_user", "username")
	gormdb.Model(&System{}).AddIndex("idx_mac", "mac")
	gormdb.Model(&Command{}).AddIndex("idx_user_command_created", "user_id, created, command")
	gormdb.Model(&Command{}).AddIndex("idx_user_uuid", "user_id, uuid")
	gormdb.Model(&Config{}).AddUniqueIndex("idx_config_id", "id")
	gormdb.Model(&Command{}).AddUniqueIndex("idx_uuid", "uuid")

	// Just need gorm for migration and index creation.
	gormdb.Close()
}

func (c Config) getSecret() string {
	var err error
	if connectionLimit != 1 {
		_, err = db.Exec(`INSERT INTO configs ("id","created", "secret") 
						VALUES (1, now(), (SELECT md5(random()::text)))
						ON conflict do nothing;`)

	} else {
		_, err = db.Exec(`INSERT INTO configs ("id","created" ,"secret") 
						VALUES (1, current_timestamp, lower(hex(randomblob(16)))) 
						ON conflict do nothing;`)
	}
	if err != nil {
		log.Fatal(err)
	}
	err = db.QueryRow(`SELECT "secret" from configs where "id" = 1 `).Scan(&c.Secret)
	return c.Secret
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

func (user User) userGetID() uint {
	var id uint
	err := db.QueryRow(`SELECT "id" 
							FROM users 
							WHERE "username"  = $1`,
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

	res, err := db.Exec(`
	INSERT INTO commands("process_id","process_start_time","exit_status","uuid","command", "created", "path", "user_id", "system_name")
 	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT do nothing`,
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

func (cmd Command) commandGet() ([]Query, error) {
	var (
		results []Query
		query   string
	)
	if cmd.Unique || cmd.Query != "" {
		//postgres
		if connectionLimit != 1 {
			if cmd.SystemName != "" && cmd.Path != "" && cmd.Query != "" && cmd.Unique {
				query = fmt.Sprintf(`
				SELECT * FROM  ( 
			        SELECT DISTINCT ON ("command") command, "uuid", "created"
			        FROM commands
			       	WHERE  "user_id" = '%v'  
			       	AND "path" = '%v' 
			       	AND "system_name" = '%v'								
			       	AND "command" ~ '%v'
			        ) c
			    ORDER BY "created" DESC limit '%v';`, cmd.User.ID, cmd.Path, cmd.SystemName, cmd.Query, cmd.Limit)

			} else if cmd.Path != "" && cmd.Query != "" && cmd.Unique {
				query = fmt.Sprintf(`
				SELECT * FROM  ( 
					SELECT DISTINCT ON ("command") command, "uuid", "created"
					FROM commands
					WHERE  "user_id" = '%v'  
					AND "path" = '%v' 
					AND "command" ~ '%v'
					) c
				ORDER BY "created" DESC limit '%v';`, cmd.User.ID, cmd.Path, cmd.Query, cmd.Limit)

			} else if cmd.SystemName != "" && cmd.Query != "" {
				query = fmt.Sprintf(`
				SELECT "command", "uuid", "created"
					FROM commands
					WHERE  "user_id" = '%v'  
					AND "system_name" = '%v' 
					AND "command" ~ '%v'
				ORDER BY "created" DESC limit '%v';`, cmd.User.ID, cmd.SystemName, cmd.Query, cmd.Limit)

			} else if cmd.Path != "" && cmd.Query != "" {
				query = fmt.Sprintf(`
				SELECT "command", "uuid", "created"
					FROM commands
					WHERE  "user_id" = '%v'  
					AND "path" = '%v' 
					AND "command" ~ '%v'
				ORDER BY "created" DESC limit '%v';`, cmd.User.ID, cmd.Path, cmd.Query, cmd.Limit)

			} else if cmd.SystemName != "" && cmd.Unique {
				query = fmt.Sprintf(`
				SELECT * FROM  ( 
					SELECT DISTINCT ON ("command") command, "uuid", "created"
					FROM commands
					WHERE  "user_id" = '%v'  
					AND "system_name" = '%v' 
					) c
				ORDER BY "created" DESC limit '%v';`, cmd.User.ID, cmd.SystemName, cmd.Limit)

			} else if cmd.Path != "" && cmd.Unique {
				query = fmt.Sprintf(`
				SELECT * FROM  ( 
					SELECT DISTINCT ON ("command") command, "uuid", "created"
					FROM commands
					WHERE  "user_id" = '%v'  
					AND "path" = '%v'
					) c
				ORDER BY "created" DESC limit '%v';`, cmd.User.ID, cmd.Path, cmd.Limit)

			} else if cmd.Query != "" && cmd.Unique {
				query = fmt.Sprintf(`
				SELECT * FROM  ( 
					SELECT DISTINCT ON ("command") command, "uuid", "created"
					FROM commands
					WHERE  "user_id" = '%v'  
					AND "command" ~ '%v'
					) c
				ORDER BY "created" DESC limit '%v';`, cmd.User.ID, cmd.Query, cmd.Limit)

			} else if cmd.Query != "" {
				query = fmt.Sprintf(`
				SELECT "command", "uuid", "created"
					FROM commands
					WHERE  "user_id" = '%v'  
					AND "command" ~ '%v'
				ORDER BY "created" DESC limit '%v';`, cmd.User.ID, cmd.Query, cmd.Limit)

			} else {
				// unique
				query = fmt.Sprintf(`
				SELECT * FROM  ( 
					SELECT DISTINCT ON ("command") command, "uuid", "created"
					FROM commands
					WHERE  "user_id" = '%v'   
					) c
				ORDER BY "created" DESC limit '%v';`, cmd.User.ID, cmd.Limit)
			}
		} else {
			// sqlite
			if cmd.SystemName != "" && cmd.Path != "" && cmd.Query != "" && cmd.Unique {
				// Have to use fmt.Sprintf to build queries where sqlite regexp function is used because of single quotes. Haven't found any other work around.
				query = fmt.Sprintf(`
				SELECT "command",  "uuid", "created" FROM commands
                    WHERE "user_id" =  '%v'  
					AND "path" = '%v'
					AND "system_name" = '%v'
					AND "command" regexp '%v'
				GROUP BY "command" ORDER  BY  "created" DESC limit '%v'`, cmd.User.ID, cmd.Path, cmd.SystemName, cmd.Query, cmd.Limit)

			} else if cmd.SystemName != "" && cmd.Query != "" && cmd.Unique {
				query = fmt.Sprintf(`
				SELECT "command",  "uuid", "created" FROM commands
                    WHERE "user_id" =  '%v'  
					AND "system_name" = '%v' 
					AND "command" regexp '%v'
				GROUP BY "command" ORDER  BY  "created" DESC limit '%v'`, cmd.User.ID, cmd.SystemName, cmd.Query, cmd.Limit)

			} else if cmd.Path != "" && cmd.Query != "" && cmd.Unique {
				query = fmt.Sprintf(`
				SELECT "command",  "uuid", "created" FROM commands
                    WHERE "user_id" =  '%v'  
					AND "path" = '%v' 
					AND "command" regexp '%v'
				GROUP BY "command" ORDER  BY  "created" DESC limit '%v'`, cmd.User.ID, cmd.Path, cmd.Query, cmd.Limit)

			} else if cmd.SystemName != "" && cmd.Query != "" {
				query = fmt.Sprintf(`
				SELECT "command",  "uuid", "created" FROM commands
					WHERE "user_id" =  '%v'   
					AND "system_name" = '%v' 
					AND "command" regexp '%v'
				ORDER  BY  "created" DESC limit '%v'`, cmd.User.ID, cmd.SystemName, cmd.Query, cmd.Limit)

			} else if cmd.Path != "" && cmd.Query != "" {
				query = fmt.Sprintf(`
				SELECT "command",  "uuid", "created" FROM commands
					WHERE "user_id" =  '%v'   
					AND "path" = '%v' 
					AND "command" regexp '%v'
				ORDER  BY  "created" DESC limit '%v'`, cmd.User.ID, cmd.Path, cmd.Query, cmd.Limit)

			} else if cmd.SystemName != "" && cmd.Unique {
				query = fmt.Sprintf(`
				SELECT "command",  "uuid", "created" FROM commands
					WHERE  "user_id" = '%v'   
					AND "system_name" = '%v'
				GROUP BY "command" ORDER  BY  "created" DESC limit '%v'`, cmd.User.ID, cmd.SystemName, cmd.Limit)

			} else if cmd.Path != "" && cmd.Unique {
				query = fmt.Sprintf(`
				SELECT "command",  "uuid", "created" FROM commands
					WHERE  "user_id" = '%v'   
					AND "path" = '%v'
				GROUP BY "command" ORDER  BY  "created" DESC limit '%v'`, cmd.User.ID, cmd.Path, cmd.Limit)

			} else if cmd.Query != "" && cmd.Unique {
				query = fmt.Sprintf(`
				SELECT "command", "uuid", "created" FROM commands
					WHERE "user_id" =  '%v'   
					AND "command" regexp '%v'  
				GROUP BY "command" ORDER  BY "created" DESC limit '%v'`, cmd.User.ID, cmd.Query, cmd.Limit)

			} else if cmd.Query != "" {
				query = fmt.Sprintf(`
				SELECT "command",  "uuid", "created" FROM commands
					WHERE "user_id" =  '%v'   
					AND "command" regexp'%v'
				ORDER  BY  "created" DESC limit '%v'`, cmd.User.ID, cmd.Query, cmd.Limit)

			} else {
				// unique
				query = fmt.Sprintf(`
				SELECT "command", "uuid", "created"
					FROM commands
					WHERE  "user_id" = '%v'  
				GROUP BY "command"  ORDER BY "created" DESC limit '%v';`, cmd.User.ID, cmd.Limit)
			}
		}
	} else {
		if cmd.Path != "" {
			query = fmt.Sprintf(`
			SELECT "command",  "uuid", "created" FROM commands
				WHERE  "user_id" = '%v' 
				AND "path" = '%v'  
			ORDER  BY  "created" DESC limit '%v'`, cmd.User.ID, cmd.Path, cmd.Limit)
		} else if cmd.SystemName != "" {
			query = fmt.Sprintf(`SELECT "command",  "uuid", "created" FROM commands
			WHERE  "user_id" = '%v'
			AND "system_name" = '%v'
			ORDER  BY  "created" DESC limit '%v'`, cmd.User.ID, cmd.SystemName, cmd.Limit)

		} else {
			query = fmt.Sprintf(`
			SELECT "command",  "uuid", "created" FROM commands
				WHERE  "user_id" = '%v'  
			ORDER  BY  "created" DESC limit '%v'`, cmd.User.ID, cmd.Limit)
		}

	}

	rows, err := db.Query(query)

	if err != nil {
		return []Query{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var result Query
		err = rows.Scan(&result.Command, &result.Uuid, &result.Created)
		if err != nil {
			return []Query{}, err
		}
		results = append(results, result)
	}
	return results, nil

}

func (cmd Command) commandGetUUID() (Query, error) {
	var result Query
	err := db.QueryRow(`
	SELECT "command","path", "created" , "uuid", "exit_status", "system_name", "process_id" 
		FROM commands
		WHERE "uuid" = $1 
	AND "user_id" = $2`, cmd.Uuid, cmd.User.ID).Scan(&result.Command, &result.Path, &result.Created, &result.Uuid,
		&result.ExitStatus, &result.SystemName, &result.SessionID)
	if err != nil {
		return Query{}, err
	}
	return result, nil
}

func (cmd Command) commandDelete() int64 {
	res, err := db.Exec(`
	DELETE FROM commands WHERE "user_id" = $1 AND "uuid" = $2 `, cmd.User.ID, cmd.Uuid)
	if err != nil {
		log.Fatal(err)
	}
	inserted, err := res.RowsAffected()
	if err != nil {
		log.Fatal(err)
	}
	return inserted

}

func (sys System) systemUpdate() int64 {

	t := time.Now().Unix()
	res, err := db.Exec(`
	UPDATE systems 
		SET "hostname" = $1 , "updated" = $2
		WHERE "user_id" = $3
		AND "mac" = $4`,
		sys.Hostname, t, sys.User.ID, sys.Mac)
	if err != nil {
		log.Fatal(err)
	}
	inserted, err := res.RowsAffected()
	if err != nil {
		log.Fatal(err)
	}
	return inserted
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

func (sys System) systemGet() (System, error) {
	var row System
	err := db.QueryRow(`SELECT "name", "mac", "user_id", "hostname", "client_version",
 									  "id", "created", "updated" FROM systems 
 							  WHERE  "user_id" = $1
 							  AND "mac" = $2`,
		sys.User.ID, sys.Mac).Scan(&row.Name, &row.Mac, &row.UserId, &row.Hostname,
		&row.ClientVersion, &row.ID, &row.Created, &row.Updated)
	if err != nil {
		return System{}, err
	}
	return row, nil

}

func (status Status) statusGet() (Status, error) {
	var err error
	if connectionLimit != 1 {
		err = db.QueryRow(`
		select
      		( select count(*) from commands where user_id = $1) as totalCommands,
      		( select count(distinct process_id) from commands where user_id = $1) as totalSessions,
      		( select count(*) from systems where user_id = $1) as totalSystems,
      		( select count(*) from commands where to_timestamp(cast(created/1000 as bigint))::date = now()::date and  user_id = $1) as totalCommandsToday,
      		( select count(*) from commands where process_id = $2) as sessionTotalCommands`,
			status.User.ID, status.ProcessID).Scan(
			&status.TotalCommands, &status.TotalSessions, &status.TotalSystems,
			&status.TotalCommandsToday, &status.SessionTotalCommands)
	} else {
		err = db.QueryRow(`
		select
      		( select count(*) from commands where user_id = $1) as totalCommands,
      		( select count(distinct process_id) from commands where user_id = $1) as totalSessions,
      		( select count(*) from systems where user_id = $1) as totalSystems,
      		( select count(*) from commands where date(created/1000, 'unixepoch') = date('now') and  user_id = $1) as totalCommandsToday,
      		( select count(*) from commands where process_id = $2) as sessionTotalCommands`,
			status.User.ID, status.ProcessID).Scan(
			&status.TotalCommands, &status.TotalSessions, &status.TotalSystems,
			&status.TotalCommandsToday, &status.SessionTotalCommands)
	}
	if err != nil {
		return Status{}, err
	}
	return status, err
}

func importCommands(imp Import) error {
	_, err := db.Exec(`
	INSERT INTO commands ("command", "path", "created", "uuid", "exit_status","system_name", "session_id", "user_id" )
	VALUES ($1,$2,$3,$4,$5,$6,$7 ,(select "id" from users where "username" = $8)) ON CONFLICT do nothing`,
		imp.Command, imp.Path, imp.Created, imp.Uuid, imp.ExitStatus, imp.SystemName, imp.SessionID, imp.Username)
	if err != nil {
		return err
	}
	return nil
}
